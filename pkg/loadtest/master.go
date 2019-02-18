package loadtest

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	defaultMasterNodeEventBufSize = 1000
)

// MasterNode represents a load testing master node.
type MasterNode struct {
	*BaseReactor

	Config *Config
	Logger *logrus.Entry

	slaves            map[string]*connectedSlave // The slaves connected to the master node.
	slavesReadyTicker *time.Ticker               // To keep checking whether we're ready to tell the slaves to kick off the load test.
	slavesFinished    int                        // To keep track of how many slaves successfully completed their share of the load testing.
}

// connectedSlave helps us manage slaves from the master's perspective.
type connectedSlave struct {
	*BaseReactor

	id       string             // A unique ID for this slave
	lastSeen time.Time          // When did we last receive a message from this slave?
	master   *MasterNode        // A pointer back to the master node, so we can communicate with it.
	state    SlaveState         // So that the master can keep track of the state of this slave.
	ws       *WebSocketsReactor // For the asynchronous and thread-safe interaction with the WebSockets client.
}

// MasterNode implements Reactor
var _ Reactor = (*MasterNode)(nil)

// connectedSlave implements Reactor
var _ Reactor = (*connectedSlave)(nil)

// NewMasterNode creates a new master node with the given configuration options.
func NewMasterNode(config *Config) *MasterNode {
	n := &MasterNode{
		Config:         config,
		slaves:         make(map[string]*connectedSlave),
		slavesFinished: 0,
	}
	// configure the implementation for the base reactor
	n.BaseReactor = NewBaseReactor(n, "master")
	return n
}

// OnStart will trigger the background execution of the WebSockets server for
// the master node.
func (n *MasterNode) OnStart() error {
	// Configure the WebSockets handler function for this master node
	http.HandleFunc("/", makeNodeWebSocketsHandler(n))

	// Run the WebSockets server in a separate goroutine so we have more control
	// over the shutdown process
	go func(n_ *MasterNode) {
		n_.Logger.Errorln(http.ListenAndServe(n_.Config.Master.Bind, nil))
	}(n)

	return nil
}

// Handle will handle internal events inside the master node's event loop.
func (n *MasterNode) Handle(e ReactorEvent) {
	n.logger.Debugln("Inside MasterNode.Handle()")
	switch e.Type {
	case EvSlaveReady:
		n.handleSlaveReady(e)

	case EvSlaveStartedLoadTest:
		n.handleSlaveStartedLoadTest(e)

	case EvSlaveFailed:
		n.handleSlaveFailed(e)

	case EvSlaveFinishedLoadTest:
		n.handleSlaveFinishedLoadTest(e)

	default:
		n.logger.WithField("e", e).Debugln("Got unrecognized event type")
		e.Respond(ReactorEvent{Type: EvUnrecognizedEventType, Data: e.Type})
	}
}

// OnShutdown right now just logs the fact that we're shutting down.
func (n *MasterNode) OnShutdown() {
	n.Logger.Infoln("Shutting down reactor loop")
}

//
// Internal functions
//

func (n *MasterNode) handleSlaveReady(e ReactorEvent) {
	s := e.Data.(connectedSlave)
	n.Logger.WithField("s", s).Debugln("Got slave ready notification")
	// if we've already got a slave by this ID
	_, exists := n.slaves[s.id]
	if exists {
		n.Logger.WithField("s", s).Infoln("Already seen incoming slave - ignoring")
		e.Respond(ReactorEvent{
			Type: EvSlaveAlreadySeen,
			Data: s.id,
		})
		return
	}

	// if we've already got enough slaves connected
	if len(n.slaves) >= n.Config.Master.ExpectSlaves {
		n.Logger.WithField("s", s).Infoln("Too many connected slaves - ignoring new incoming connection")
		e.Respond(ReactorEvent{
			Type: EvTooManySlaves,
		})
		return
	}

	// keep track of this newly connected slave
	n.slaves[s.id] = &s
	n.Logger.WithField("id", s.id).Infoln("Slave connected")
	e.Respond(ReactorEvent{Type: EvSlaveAccepted})
}

// If one of the connected slaves fail, we need to kill them all.
func (n *MasterNode) handleSlaveFailed(e ReactorEvent) {
	slaveID := e.Data.(string)
	n.Logger.WithField("id", slaveID).Errorln("Slave failed")

	for _, slave := range n.slaves {
		slave.client.Recv(ReactorEvent{Type: EvPoisonPill})
	}

	n.shutdownError = NewError(ErrSlaveFailed, nil, slaveID)
	n.Shutdown()
}

func (n *MasterNode) handleSlaveStartedLoadTest(e ReactorEvent) {
	slaveID := e.Data.(string)
	n.Logger.WithField("id", slaveID).Infoln("Slave started load test")
	n.slaves[slaveID].client.state = SSTesting
}

func (n *MasterNode) handleSlaveFinishedLoadTest(e ReactorEvent) {
	slaveID := e.Data.(string)
	n.Logger.WithField("id", slaveID).Infoln("Slave finished load test")
	n.slaves[slaveID].client.state = SSFinished
	n.slavesFinished++

	if n.slavesFinished == len(n.slaves) {
		n.Logger.Infoln("All slaves successfully completed load testing")
		n.BaseReactor.shutdownError = nil
		n.Shutdown()
	}
}

//
// connectedSlave
//

func newConnectedSlave(conn *websocket.Conn, m *MasterNode) *connectedSlave {
	c := &connectedSlave{
		id:       "no-id-yet",
		lastSeen: time.Now(),
		master:   m,
		state:    SSConnected,
		ws:       NewWebSocketsReactor(conn),
	}
	c.BaseReactor = NewBaseReactor(c, "connectedSlave")
	return c
}

func (c *connectedSlave) Handle(e ReactorEvent) {
	switch e.Type {
	case EvGetWebSocketsClientState:
		e.Respond(ReactorEvent{Type: EvGetWebSocketsClientState, Data: c.state})

	case EvSetWebSocketsClientState:
		c.state = e.Data.(SlaveState)
		e.Respond(ReactorEvent{Type: EvOK})

	case EvSlaveReady:
		// update this client's ID
		c.id = e.Data.(EventSlaveReady).ID
		// inform the master about the newly connected slave
		r := c.master.RecvAndAwait(e)
		if r.Type == EvSlaveAccepted {
			c.state = SSAccepted
		} else {
			c.state = SSRejected
		}
		e.Respond(r)

	case EvStartLoadTest:
		// pass the test start message on to the client
		c.ws.SendMsg(&WebSocketsMsgEnvelope{Type: e.Type, Message: e.Data})

	case EvSlaveStartedLoadTest:
		c.state = SSTesting

	case EvSlaveFailed:
		c.state = SSFailed
		// indicate to the master that this slave failed, so it can stop the
		// load test across the other slaves
		c.master.Recv(ReactorEvent{Type: EvSlaveFailed, Data: c.id})

	default:
		e.Respond(ReactorEvent{Type: EvUnrecognizedEventType, Data: e.Type})
	}
}
