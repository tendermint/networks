package loadtest_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/networks/internal/logging"
	"github.com/tendermint/networks/pkg/actor"
	"github.com/tendermint/networks/pkg/loadtest"
)

type sharedKVStoreState struct {
	kvstore map[string]string
	mtx     *sync.RWMutex
}

func newSharedKVStoreState() *sharedKVStoreState {
	return &sharedKVStoreState{
		kvstore: make(map[string]string),
		mtx:     &sync.RWMutex{},
	}
}

func (s *sharedKVStoreState) put(key, value string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.kvstore[key] = value
}

func (s *sharedKVStoreState) get(key string) (string, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	value, ok := s.kvstore[key]
	if !ok {
		return "", fmt.Errorf("No such key")
	}
	return value, nil
}

// Runs a full integration test between a master node with two slaves,
// interacting with two mock `kvstore` proxy apps running on fake Tendermint RPC
// nodes.
func TestFullIntegrationHTTPKVStore(t *testing.T) {
	logger := logging.NewLogrusLogger("test")
	cfg := testConfig(2, "kvstore_http")

	state := newSharedKVStoreState()

	errc := make(chan error, 10)
	tm1stopc, tm2stopc := make(chan bool, 1), make(chan bool, 1)
	tm1donec, tm2donec := make(chan bool, 1), make(chan bool, 1)

	// fire up the mock Tendermint HTTP RPC servers
	tm1Host, tm1Port := getFreeTCPAddress()
	tm1Addr := fmt.Sprintf("%s:%d", tm1Host, tm1Port)
	go mockTendermintHTTPRPCServer(tm1Addr, state, errc, tm1stopc, tm1donec)

	tm2Host, tm2Port := getFreeTCPAddress()
	tm2Addr := fmt.Sprintf("%s:%d", tm2Host, tm2Port)
	go mockTendermintHTTPRPCServer(tm2Addr, state, errc, tm2stopc, tm2donec)

	cfg.TestNetwork.Targets = []loadtest.TestNetworkTargetConfig{
		loadtest.TestNetworkTargetConfig{
			ID:      "tm1",
			Host:    tm1Host,
			RPCPort: tm1Port,
		},
		loadtest.TestNetworkTargetConfig{
			ID:      "tm2",
			Host:    tm2Host,
			RPCPort: tm2Port,
		},
	}

	// start a probe actor to listen for the stats
	probe := actor.NewProbe()
	if err := probe.Start(); err != nil {
		t.Fatal(err)
	}

	// start the master and 2 slaves
	masterc := make(chan error, 1)
	go func() {
		master := loadtest.NewMasterNode(cfg)
		if err := master.Start(); err != nil {
			masterc <- err
			return
		}
		// subscribe to the final stats messages
		master.Subscribe(probe, loadtest.SlaveFinished)
		masterc <- master.Wait()
	}()

	logger.Debug("Waiting for master to start up...")
	timeoutCounter := 10
	// wait for the master to start up
	for {
		c, err := net.Dial("tcp", cfg.Slave.Master)
		if err == nil {
			c.Close()
			break
		}
		select {
		case err := <-masterc:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(100 * time.Millisecond):
			timeoutCounter--
			if timeoutCounter <= 0 {
				t.Fatal("Timed out waiting for master to start up")
			}
		}
	}
	logger.Debug("Master started. Running slaves...")

	slave1c := make(chan error, 1)
	go func() { slave1c <- loadtest.RunSlaveWithConfig(cfg) }()

	slave2c := make(chan error, 1)
	go func() { slave2c <- loadtest.RunSlaveWithConfig(cfg) }()

	// wait for the master and slaves to finish and shut down
loop:
	for i := 0; i < 3; i++ {
		select {
		case err := <-masterc:
			if err != nil {
				t.Error(err)
				break loop
			}
		case err := <-slave1c:
			if err != nil {
				t.Error(err)
				break loop
			}
		case err := <-slave2c:
			if err != nil {
				t.Error(err)
				break loop
			}

		case <-time.After(10 * time.Second):
			t.Error("Timed out waiting for test to complete")
		}
	}

	// wait for the probe to have captured 2 messages
	err := probe.WaitForCapturedMessages(2, 2*time.Second)
	if err != nil {
		t.Error(err)
	}

	// check the probe to see if it's successfully captured the messages
	statsMessages := probe.CapturedMessages()
	if len(statsMessages) != 2 {
		t.Errorf("Expected number of captured SlaveFinished messages to be 2, but got %d", len(statsMessages))
	} else {
		stats := loadtest.NewClientSummaryStats(time.Duration(cfg.Clients.InteractionTimeout))
		slave0Stats := statsMessages[0].Data.(loadtest.SlaveFinishedMessage).Stats
		slave1Stats := statsMessages[1].Data.(loadtest.SlaveFinishedMessage).Stats

		stats.Merge(slave0Stats)
		stats.Merge(slave1Stats)

		// we have 2 slaves, each with 1 client
		maxInteractions := int64(cfg.Clients.MaxInteractions * 2)
		if stats.Interactions.Count != maxInteractions {
			t.Errorf("Expected number of interactions to be %d, but got %d", maxInteractions, stats.Interactions.Count)
		}
		if len(stats.Requests) != 2 {
			t.Errorf("Expected number of request IDs to be 2, but got %d", len(stats.Requests))
		}
		bts, ok := stats.Requests["broadcast_tx_sync"]
		if !ok {
			t.Error("Expected request stats for broadcast_tx_sync, but got none")
		} else {
			if bts.Count != maxInteractions {
				t.Errorf("Expected number of broadcast_tx_sync requests to be %d, but got %d", maxInteractions, bts.Count)
			}
			if bts.Errors != 0 {
				t.Errorf("Expected number of broadcast_tx_sync errors to be 0, but got %d", bts.Errors)
			}
		}
		aq, ok := stats.Requests["abci_query"]
		if !ok {
			t.Error("Expected request stats for abci_query, but got none")
		} else {
			if aq.Count != maxInteractions {
				t.Errorf("Expected number of abci_query requests to be %d, but got %d", maxInteractions, aq.Count)
			}
			if aq.Errors != 0 {
				t.Errorf("Expected number of abci_query errors to be 0, but got %d", aq.Errors)
			}
		}

	}

	// shut the probe down
	probe.Recv(actor.Message{Type: actor.PoisonPill})

	// tell the mock HTTP servers to shut down
	tm1stopc <- true
	tm2stopc <- true

	// wait for the mock HTTP servers to shut down
	for i := 0; i < 2; i++ {
		select {
		case <-tm1donec:
		case <-tm2donec:
		case <-time.After(30 * time.Second):
			t.Error("Timed out waiting for mock HTTP servers to shut down")
		}
	}

	// wait for the probe to shut down properly
	if err := probe.Wait(); err != nil {
		t.Error(err)
	}
}

func mockTendermintHTTPRPCServer(bindAddr string, state *sharedKVStoreState, errc chan error, stopc chan bool, donec chan bool) {
	testLogger := logging.NewLogrusLogger("mock-http-rpc", "bindAddr", bindAddr)
	testMux := http.NewServeMux()
	testMux.HandleFunc("/broadcast_tx_sync", func(w http.ResponseWriter, r *http.Request) {
		tx := strings.Trim(r.URL.Query().Get("tx"), "\"")
		parts := strings.Split(tx, "=")
		if len(parts) != 2 {
			errc <- fmt.Errorf("Got malformed transaction: %s", tx)
			w.WriteHeader(400)
			return
		}

		testLogger.Debug("Storing key/value pair", "key", parts[0], "value", parts[1])

		// add the key/value pair to the state store
		state.put(parts[0], parts[1])

		w.WriteHeader(200)
	})
	testMux.HandleFunc("/abci_query", func(w http.ResponseWriter, r *http.Request) {
		data := strings.Trim(r.URL.Query().Get("data"), "\"")
		value, err := state.get(data)
		if err != nil {
			// we shouldn't be getting queries for things we don't recognise
			errc <- err
			w.WriteHeader(404)
			return
		}

		testLogger.Debug("Retrieved key/value pair", "key", data, "value", value)

		res := loadtest.ABCIQueryHTTPResponse{
			JSONRPC: "2.0",
			ID:      "",
			Result: loadtest.ABCIQueryResult{
				Response: loadtest.ABCIQueryResponse{
					Log:   "found",
					Key:   base64.StdEncoding.EncodeToString([]byte(data)),
					Value: base64.StdEncoding.EncodeToString([]byte(value)),
				},
			},
		}

		resData, err := json.Marshal(&res)
		if err != nil {
			errc <- err
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)

		if _, err = w.Write(resData); err != nil {
			errc <- err
			return
		}
	})

	httpServer := &http.Server{
		Addr:    bindAddr,
		Handler: testMux,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			errc <- err
		}
	}()

	select {
	case <-stopc:
		if err := httpServer.Shutdown(context.Background()); err != nil {
			errc <- err
		}

	case <-time.After(30 * time.Second):
		errc <- fmt.Errorf("Timed out waiting for test to complete")
	}

	// this goroutine's done
	donec <- true
}
