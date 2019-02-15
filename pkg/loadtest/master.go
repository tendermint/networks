package loadtest

import (
	"net/http"
	"time"

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

	eventChan        chan ReactorEvent // The primary channel on which the master node receives events.
	quitChan         chan struct{}     // A channel exclusively for quit notifications.
	quitCompleteChan chan struct{}     // A channel exclusively for getting notified as to when shutdown is complete.

	slaves            map[string]*connectedSlave // The slaves connected to the master node.
	slavesReadyTicker *time.Ticker               // To keep checking whether we're ready to tell the slaves to kick off the load test.
}

// connectedSlave helps us manage slaves from the master's perspective.
type connectedSlave struct {
	id       string            // A unique ID for this slave
	lastSeen time.Time         // When did we last receive a message from this slave?
	client   *webSocketsClient // This is how we can talk to the slave in a thread-safe way.
}

// MasterNode implements Reactor
var _ Reactor = (*MasterNode)(nil)

// NewMasterNode creates a new master node with the given configuration options.
func NewMasterNode(config *Config) *MasterNode {
	n := &MasterNode{
		BaseReactor:      NewBaseReactor(nil),
		Config:           config,
		Logger:           logrus.WithField("ctx", "master"),
		eventChan:        make(chan ReactorEvent, defaultMasterNodeEventBufSize),
		quitChan:         make(chan struct{}),
		quitCompleteChan: make(chan struct{}),
	}
	// configure the implementation for the base reactor
	n.BaseReactor.impl = n
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
	switch e.Type {
	case EvSlaveReady:
		n.handleSlaveReady(e)
	default:
		n.Logger.WithField("e", e).Infoln("Received unrecognized reactor event")
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
