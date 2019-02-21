package loadtest_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
	"github.com/tendermint/networks/pkg/loadtest"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  loadtest.DefaultWebSocketsReadBufSize,
	WriteBufferSize: loadtest.DefaultWebSocketsWriteBufSize,
}

func noopClientFactory(th *loadtest.TestHarness) *loadtest.TestHarnessClient {
	return loadtest.NewTestHarnessClient(th, &loadtest.NoopInteractor{})
}

// Tests the basic "happy path" for the slave node.
func TestSlaveNodeLifecycle(t *testing.T) {
	cfg := testConfig(1, "noop")

	slave := loadtest.NewSlaveNode(cfg, noopClientFactory)

	errc := make(chan error)
	stopc := make(chan bool)
	go mockMasterNode(cfg.Master.Bind, slave.GetID(), errc, stopc)
	defer func() {
		// bring the HTTP server down
		stopc <- true
	}()

	if err := slave.Start(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error)
	go func() {
		done <- slave.Wait()
	}()

	select {
	case err := <-errc:
		t.Error(err)

	case err := <-done:
		if err != nil {
			t.Error(err)
		}

	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for slave lifecycle to complete")
	}
}

func mockMasterNode(bindAddr, slaveID string, errc chan error, stopc chan bool) {
	testMux := http.NewServeMux()
	testMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errc <- err
			return
		}
		defer conn.Close()

		// listen for an incoming SlaveReady message
		msg, err := loadtest.WebSocketsRecv(conn)
		if err != nil {
			errc <- fmt.Errorf("Failed to receive SlaveReady message from slave: %v", err)
			return
		}
		if msg.Type != loadtest.SlaveReady {
			errc <- fmt.Errorf("Expected %s message from slave, but got %s", loadtest.SlaveReady, msg.Type)
			return
		}
		idMsg, ok := msg.Data.(loadtest.SlaveIDMessage)
		if !ok {
			errc <- fmt.Errorf("Failed to parse slave ID message")
			return
		}
		if idMsg.ID != slaveID {
			errc <- fmt.Errorf("Expected slave ID %s, but got %s", slaveID, idMsg.ID)
			return
		}

		// we need to indicate to the slave that it's acceptable
		if err = loadtest.WebSocketsSend(conn, actor.Message{Type: loadtest.SlaveAccepted}); err != nil {
			errc <- err
			return
		}

		// now tell the slave to start load testing
		if err = loadtest.WebSocketsSend(conn, actor.Message{Type: loadtest.StartLoadTest}); err != nil {
			errc <- err
			return
		}

		// the slave should eventually tell us that the load testing is complete
		msg, err = loadtest.WebSocketsRecv(conn)
		if err != nil {
			errc <- fmt.Errorf("Failed to receive SlaveFinished message from slave: %v", err)
			return
		}
		if msg.Type != loadtest.SlaveFinished {
			errc <- fmt.Errorf("Expected %s message from slave, but got %s", loadtest.SlaveFinished, msg.Type)
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

	// wait until we get the stop signal to shut down the HTTP server
	select {
	case <-stopc:
		if err := httpServer.Shutdown(context.Background()); err != nil {
			errc <- fmt.Errorf("Failed to cleanly shut down HTTP server")
		} else {
			fmt.Println("Successfully terminated server")
		}

	case <-time.After(1 * time.Minute):
		errc <- fmt.Errorf("Timed out waiting for server stop signal")
	}
}
