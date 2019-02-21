package loadtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

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
	cfg := testConfig(2, "kvstore_http")

	state := newSharedKVStoreState()

	errc := make(chan error, 10)
	tm1stopc, tm2stopc := make(chan bool), make(chan bool)
	tm1donec, tm2donec := make(chan bool), make(chan bool)

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

	// start the master and 2 slaves
	masterc := make(chan error)
	go func() { masterc <- loadtest.RunMasterWithConfig(cfg) }()

	slave1c := make(chan error)
	go func() { slave1c <- loadtest.RunSlaveWithConfig(cfg) }()

	slave2c := make(chan error)
	go func() { slave2c <- loadtest.RunSlaveWithConfig(cfg) }()

	// wait for the master and slaves to finish and shut down
	for i := 0; i < 3; i++ {
		select {
		case err := <-masterc:
			if err != nil {
				t.Error(err)
			}
		case err := <-slave1c:
			if err != nil {
				t.Error(err)
			}
		case err := <-slave2c:
			if err != nil {
				t.Error(err)
			}

		case <-time.After(10 * time.Second):
			t.Error("Timed out waiting for test to complete")
		}
	}

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
}

func mockTendermintHTTPRPCServer(bindAddr string, state *sharedKVStoreState, errc chan error, stopc chan bool, donec chan bool) {
	testMux := http.NewServeMux()
	testMux.HandleFunc("/broadcast_tx_sync", func(w http.ResponseWriter, r *http.Request) {
		tx := strings.Trim(r.URL.Query().Get("tx"), "\"")
		parts := strings.Split(tx, "=")
		if len(parts) != 2 {
			errc <- fmt.Errorf("Got malformed transaction: %s", tx)
			w.WriteHeader(400)
			return
		}

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

		res := loadtest.ABCIQueryHTTPResponse{
			JSONRPC: "2.0",
			ID:      "",
			Result: loadtest.ABCIQueryResult{
				Response: loadtest.ABCIQueryResponse{
					Log:   "found",
					Key:   data,
					Value: value,
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
