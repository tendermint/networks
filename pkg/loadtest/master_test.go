package loadtest_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
	"github.com/tendermint/networks/pkg/loadtest"
)

func testConfig() *loadtest.Config {
	masterAddr := getFreeTCPAddress()
	return &loadtest.Config{
		Master: loadtest.MasterConfig{
			Bind:         masterAddr,
			ExpectSlaves: 1,
		},
		Slave: loadtest.SlaveConfig{
			Master: masterAddr,
		},
	}
}

func getFreeTCPAddress() string {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:")
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return fmt.Sprintf("127.0.0.1:%d", l.Addr().(*net.TCPAddr).Port)
}

// TestMasterNodeLifecycle will test the "happy path" for a master node.
func TestMasterNodeLifecycle(t *testing.T) {
	cfg := testConfig()
	master := loadtest.NewMasterNode(cfg)
	if err := master.Start(); err != nil {
		t.Fatal("Failed to start master node", err)
	}

	// mock the interaction between the master and a slave
	mockWebSocketsClient(t, cfg.Slave.Master)

	done := make(chan struct{})
	go func() {
		master.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("Timed out waiting for test to complete")
	}

	if err := master.GetShutdownError(); err != nil {
		t.Error(err)
	}
}

func mockWebSocketsClient(t *testing.T, masterAddr string) {
	c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s", masterAddr), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	id := loadtest.SlaveIDMessage{ID: "test-slave"}

	// first tell the master that we're ready
	if err := loadtest.WebSocketsSend(c, actor.Message{Type: loadtest.SlaveReady, Data: id}); err != nil {
		t.Error(err)
		return
	}
	// check that we're accepted
	msg, err := loadtest.WebSocketsRecv(c)
	if err != nil {
		t.Error(err)
		return
	}
	if msg.Type != loadtest.SlaveAccepted {
		t.Errorf("Expected response \"%s\", but got \"%s\"", loadtest.SlaveAccepted, msg.Type)
	}

	// now wait for the go-ahead for load testing
	msg, err = loadtest.WebSocketsRecv(c)
	if err != nil {
		t.Error(err)
		return
	}
	if msg.Type != loadtest.StartLoadTest {
		t.Errorf("Expected response \"%s\", but got \"%s\"", loadtest.StartLoadTest, msg.Type)
	}

	//
	// this is where the load testing would happen
	//

	if err = loadtest.WebSocketsSend(c, actor.Message{Type: loadtest.SlaveFinished, Data: id}); err != nil {
		t.Error(err)
		return
	}

	// send the close message
	loadtest.WebSocketsClose(c)
}
