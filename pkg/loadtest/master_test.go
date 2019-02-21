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

func testConfig(expectSlaves int, clientType string) *loadtest.Config {
	masterAddr := getFreeTCPAddress()
	return &loadtest.Config{
		Master: loadtest.MasterConfig{
			Bind:         masterAddr,
			ExpectSlaves: expectSlaves,
		},
		Slave: loadtest.SlaveConfig{
			Master: masterAddr,
		},
		Clients: loadtest.ClientConfig{
			Type:            clientType,
			Spawn:           1,
			SpawnRate:       1,
			MaxInteractions: 1,
			MaxTestTime:     loadtest.ParseableDuration(1 * time.Minute),
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
	cfg := testConfig(2, "noop")
	master := loadtest.NewMasterNode(cfg)
	if err := master.Start(); err != nil {
		t.Fatal("Failed to start master node", err)
	}

	slave1err := make(chan error)
	slave2err := make(chan error)

	// mock the interaction between the master and a slave
	go mockWebSocketsClient(cfg.Slave.Master, "slave1", slave1err)
	go mockWebSocketsClient(cfg.Slave.Master, "slave2", slave2err)

	done := make(chan error)
	go func() {
		done <- master.Wait()
	}()

	select {
	case err := <-slave1err:
		t.Fatal("Slave 1 error", err)

	case err := <-slave2err:
		t.Fatal("Slave 2 error", err)

	case err := <-done:
		if err != nil {
			t.Error(err)
		}

	case <-time.After(10 * time.Second):
		t.Error("Timed out waiting for test to complete")
	}
}

func mockWebSocketsClient(masterAddr, slaveID string, errc chan error) {
	c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s", masterAddr), nil)
	if err != nil {
		errc <- err
		return
	}
	defer c.Close()

	id := loadtest.SlaveIDMessage{ID: slaveID}

	// first tell the master that we're ready
	if err := loadtest.WebSocketsSend(c, actor.Message{Type: loadtest.SlaveReady, Data: id}); err != nil {
		errc <- err
		return
	}
	// check that we're accepted
	msg, err := loadtest.WebSocketsRecv(c)
	if err != nil {
		errc <- err
		return
	}
	if msg.Type != loadtest.SlaveAccepted {
		errc <- fmt.Errorf("Expected response \"%s\", but got \"%s\"", loadtest.SlaveAccepted, msg.Type)
	}

	// now wait for the go-ahead for load testing
	msg, err = loadtest.WebSocketsRecv(c)
	if err != nil {
		errc <- err
		return
	}
	if msg.Type != loadtest.StartLoadTest {
		errc <- fmt.Errorf("Expected response \"%s\", but got \"%s\"", loadtest.StartLoadTest, msg.Type)
	}

	//
	// this is where the load testing would happen
	//

	if err = loadtest.WebSocketsSend(c, actor.Message{Type: loadtest.SlaveFinished, Data: id}); err != nil {
		errc <- err
		return
	}

	// send the close message
	loadtest.WebSocketsClose(c)
}
