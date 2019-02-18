package loadtest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// SlaveState allows us to keep track of the state machine for connected slave
// nodes.
type SlaveState string

// The various possible states in which a slave node's state machine could be.
const (
	SSConnecting SlaveState = "connecting" // The slave is just starting up now.
	SSConnected  SlaveState = "connected"  // The slave has just connected to the master.
	SSRejected   SlaveState = "rejected"   // The slave has been rejected for some reason.
	SSAccepted   SlaveState = "accepted"   // The slave has been accepted and the master is waiting for all slaves to connect.
	SSFailed     SlaveState = "failed"     // The slave failed in some way.
	SSTesting    SlaveState = "testing"    // Load testing is underway.
	SSFinished   SlaveState = "finished"   // Load testing is finished.
)

// SlaveNode encapsulates the state for our slave reactor.
type SlaveNode struct {
	BaseReactor

	Config *Config
	Logger *logrus.Entry

	id     string
	master *connectedMaster
	state  SlaveState
}

type connectedMaster struct {
	conn *websocket.Conn
}

// SlaveNode implements Reactor
var _ Reactor = (*SlaveNode)(nil)

// NewSlaveNode allows us to instantiate a new reactor to handle slave node
// functionality.
func NewSlaveNode(config *Config) *SlaveNode {
	n := &SlaveNode{
		Config: config,
		Logger: logrus.WithField("ctx", "slave"),
		master: nil,
		id:     makeSlaveNodeID(),
		state:  SSConnecting,
	}
	n.BaseReactor = *NewBaseReactor(n)
	return n
}

func makeSlaveNodeID() string {
	var hostname string
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic("Failed to obtain hostname from OS")
	}

	b := make([]byte, 4)
	_, err = rand.Read(b)
	if err != nil {
		panic(fmt.Sprintf("Failed to read random bytes: %s", err))
	}

	return fmt.Sprintf("%s-%s", hostname, hex.EncodeToString(b))
}

// OnStart is called when starting the slave node, where it attempts to connect
// to the master node.
func (n *SlaveNode) OnStart() error {
	n.Logger.WithField("addr", n.Config.Slave.Master).Infoln("Connecting to master...")
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/", n.Config.Slave.Master), nil)
	if err != nil {
		n.Logger.WithError(err).Errorln("Failed to connect to master")
		return NewError(ErrFailedToConnectToMaster, err)
	}
	n.Logger.Infoln("Connected")
	n.master = &connectedMaster{
		conn: conn,
	}
	n.state = SSConnected

	// our primary routine for interaction with the master via the WebSockets
	// connection
	go n.handleWebSocketsLoop()

	// let the master node know we're ready to respond
	n.sendWebSocketsMsg(ReactorEvent{Type: EvSlaveReady, Data: EventSlaveReady{ID: n.id}})

	return nil
}

// OnShutdown is called as the slave node's event loop is terminating.
func (n *SlaveNode) OnShutdown() {
	n.Logger.Infoln("Shutting down connection to master...")
	if err := n.master.conn.Close(); err != nil {
		n.Logger.WithError(err).Errorln("Failed to close connection to master")
	}
	n.Logger.Infoln("Done")
}

// Handle will handle any incoming events into the slave node's internal event
// loop.
func (n *SlaveNode) Handle(e ReactorEvent) {
	switch e.Type {
	case EvRecvWebSocketsMsg:
		n.recvWebSocketsMsg(e)

	case EvSendWebSocketsMsg:
		ev, ok := e.Data.(ReactorEvent)
		if ok {
			n.sendWebSocketsMsg(ev)
		} else {
			n.Logger.WithField("e.Type", e.Type).Errorln("Failed to convert reactor event data to reactor event")
		}

	case EvSlaveAccepted:
		n.state = SSAccepted
		n.Logger.Infoln("Slave accepted by master, waiting for other slaves to connect to master...")

	case EvSlaveAlreadySeen, EvTooManySlaves:
		n.state = SSRejected
		n.Logger.WithField("state", n.state).Errorln("Slave rejected by master")
		n.BaseReactor.shutdownError = NewError(ErrSlaveRejected, nil, e.Type)
		n.Shutdown()

	case EvStartLoadTest:
		go n.ExecuteLoadTest()

	case EvSlaveFinishedLoadTest:
		// inform the master node
		n.sendWebSocketsMsg(e)
		// we're good to shut down now
		n.BaseReactor.shutdownError = nil
		n.Shutdown()

	default:
		e.Respond(ReactorEvent{Type: EvUnrecognizedEventType, Data: e.Type})
	}
}

// ExecuteLoadTest kicks off the actual load testing process.
func (n *SlaveNode) ExecuteLoadTest() {
	n.Logger.Infoln("Starting load test")
	time.Sleep(10)
	n.Logger.Infoln("Successfully finished load test")
	n.Recv(ReactorEvent{Type: EvSlaveFinishedLoadTest, Data: n.id})
}

//
// Internal SlaveNode functionality
//

func (n *SlaveNode) recvWebSocketsMsg(e ReactorEvent) {
	n.logger.WithField("e", e).Debugln("Waiting to receive WebSockets message...")
	timeout := DefaultWebSocketsReadDeadline
	if e.Timeout != nil {
		timeout = *e.Timeout
	}
	r := RecvWebSocketsMsg(n.master.conn, timeout)
	switch r.Type {
	case EvWebSocketsConnClosed:
		n.Logger.Infoln("WebSockets connection closed")
	case EvWebSocketsReadFailed:
		n.Logger.WithError(r.Data.(error)).Errorln("Failed to read message from WebSockets client")
	case EvInvalidWebSocketsMsgFormat:
		n.Logger.Errorln("Client sent non-text message")
	case EvParsingFailed:
		n.Logger.WithError(r.Data.(error)).Errorln("Failed to parse incoming WebSockets message envelope")
	}
	// respond with the result
	e.Respond(r)
	n.logger.WithField("r", r).Debugln("Received message")
}

func (n *SlaveNode) sendWebSocketsMsg(e ReactorEvent) {
	n.logger.WithField("e", e).Debugln("Sending WebSockets message...")
	err := SendWebSocketsMsg(n.master.conn, e)
	if err != nil {
		n.logger.WithError(err).Errorln("Failed to send WebSockets message")
		e.Respond(ReactorEvent{Type: EvWebSocketsWriteFailed, Data: err})
	} else {
		n.logger.Debugln("Successfully sent WebSockets message")
		e.Respond(ReactorEvent{Type: EvOK})
	}
}

func (n *SlaveNode) handleWebSocketsLoop() {
loop:
	for {
		// read a WebSockets message
		e := n.RecvAndAwait(ReactorEvent{Type: EvRecvWebSocketsMsg})
		if e.IsError() {
			if e.Type == EvWebSocketsConnClosed {
				n.logger.Infoln("Master closed WebSockets connection")
				break loop
			} else {
				n.logger.WithField("e.Type", e.Type).Errorln("Error while attempting to read from WebSockets connection")
			}
		} else {
			n.Recv(e)
		}
	}
}
