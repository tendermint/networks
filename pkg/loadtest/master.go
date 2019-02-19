package loadtest

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/tendermint/networks/pkg/actor"
)

// RemoteSlaveWaitTimeout is how long to wait for remote slaves to shut down
// gracefully before forcibly exiting.
const RemoteSlaveWaitTimeout = 5 * time.Second

// MasterReadyCheckInterval specifies the interval at which we keep checking to
// see if we have all the slaves we need.
const MasterReadyCheckInterval = 1 * time.Second

// MasterSlaveCheckInterval specifies how frequently the master will attempt to
// read for new messages from the slaves.
const MasterSlaveCheckInterval = 10 * time.Second

// MasterNode implements the load testing functionality for the master.
type MasterNode struct {
	*actor.BaseActor

	cfg             *Config
	wss             *WebSocketsServer
	remoteSlaves    map[string]actor.Actor // To be able to interact with the remote slaves.
	readyTicker     *time.Ticker           // To keep checking for when we're ready to start load testing.
	readyTickerStop chan struct{}          // To stop the readiness checker.
	recvTicker      *time.Ticker           // To keep checking for updates from the slaves.
	recvTickerStop  chan struct{}          // To kill the recv checker.
	finishedCount   int                    // To keep track of how many slaves have successfully completed their load testing.
	shutdownErr     error                  // An error to pass back to the command line, if any.
	mtx             *sync.RWMutex
}

// MasterNode implements actor.Actor
var _ actor.Actor = (*MasterNode)(nil)

// NewMasterNode creates a new master node for load testing.
func NewMasterNode(cfg *Config) *MasterNode {
	n := &MasterNode{
		cfg:             cfg,
		wss:             nil,
		remoteSlaves:    make(map[string]actor.Actor),
		readyTicker:     nil,
		readyTickerStop: make(chan struct{}),
		recvTicker:      nil,
		recvTickerStop:  make(chan struct{}),
		finishedCount:   0,
		shutdownErr:     nil,
		mtx:             &sync.RWMutex{},
	}
	n.BaseActor = actor.NewBaseActor(n, "master")
	return n
}

func (n *MasterNode) OnStart() error {
	n.wss = NewWebSocketsServer(n.cfg.Master.Bind, n.wsClientFactory)
	if err := n.wss.Start(); err != nil {
		n.Logger.WithError(err).Errorln("Failed to start WebSockets server")
		n.wss = nil
		return err
	}
	n.readyTicker = time.NewTicker(MasterReadyCheckInterval)
	go n.readinessCheckLoop()
	return nil
}

func (n *MasterNode) OnShutdown() {
	if n.wss != nil {
		n.wss.Shutdown()
	}
	if n.readyTicker != nil {
		n.readyTicker.Stop()
	}
	if n.recvTicker != nil {
		close(n.recvTickerStop)
		n.recvTicker.Stop()
	}
}

// GetShutdownError will retrieve any error that occurred during the master
// node's lifecycle.
func (n *MasterNode) GetShutdownError() error {
	return n.shutdownErr
}

// Handle will interpret incoming messages in the master's event loop.
func (n *MasterNode) Handle(msg actor.Message) {
	switch msg.Type {
	case actor.Ping:
		n.Send(msg.Sender, actor.Message{Type: actor.Pong})

	case RemoteSlaveStarted:
		// we need to hear from the remote slave first
		n.Send(msg.Sender, actor.Message{Type: RecvMessage})

	case SlaveReady:
		n.slaveReady(msg)

	case AllSlavesReady:
		if msg.Sender.GetID() == n.GetID() {
			n.startLoadTesting()
		} else {
			n.Logger.WithField("id", msg.Sender.GetID()).Errorln("Unrecognized sender for AllSlavesReady message")
		}

	case SlaveFinished:
		n.slaveFinished(msg)

	case SlaveFailed:
		n.slaveFailed(msg)

	case ConnectionClosed:
		n.connectionClosed(msg)
	}
}

func (n *MasterNode) wsClientFactory(conn *websocket.Conn) (actor.Actor, error) {
	return newRemoteSlave(conn, n), nil
}

func (n *MasterNode) slaveReady(msg actor.Message) {
	id := msg.Data.(SlaveIDMessage).ID
	n.Logger.WithField("id", id).Infoln("Got SlaveReady notification from remote slave")

	// first check if we need any more slaves
	if n.remoteSlaveCount() >= n.cfg.Master.ExpectSlaves {
		n.Logger.Infoln("Slave tried to connect, but we already have enough")
		n.Send(msg.Sender, actor.Message{Type: TooManySlaves})
		return
	}
	// then check if we've seen this slave before
	if n.remoteSlaveExists(id) {
		n.Logger.WithField("id", id).Infoln("Already seen slave before")
		n.Send(msg.Sender, actor.Message{Type: AlreadySeenSlave})
		return
	}

	// we can now safely add this slave
	n.addRemoteSlave(id, msg.Sender)
	n.Send(msg.Sender, actor.Message{Type: SlaveAccepted})
}

func (n *MasterNode) slaveFailed(msg actor.Message) {
	id := msg.Data.(SlaveIDMessage).ID
	n.removeRemoteSlave(id)
	// if we have fewer slaves than we need
	if n.remoteSlaveCount() < n.cfg.Master.ExpectSlaves {
		n.failAllSlaves()
	}
}

func (n *MasterNode) failAllSlaves() {
	n.Logger.Infoln("Informing all slaves of failure")
	n.shutdownErr = NewError(ErrSlaveFailed, nil)

	// tell all the slaves that one of them failed
	n.broadcast(actor.Message{Type: SlaveFailed})

	// wait for all the slaves to shut down
	n.waitForSlaves()

	// shut down the master too
	n.Shutdown()
}

func (n *MasterNode) waitForSlaves() {
	// wait for all of the slaves to finish
	slaves := make([]actor.Actor, len(n.remoteSlaves))
	for _, rs := range n.remoteSlaves {
		slaves = append(slaves, rs)
	}
	done := make(chan struct{})
	go func(slaves_ []actor.Actor) {
		defer close(done)
		for _, rs := range slaves_ {
			rs.Wait()
		}
	}(slaves)

	select {
	case <-done:
	case <-time.After(RemoteSlaveWaitTimeout):
		n.Logger.Errorln("Waited long enough for all slaves to shut down")
	}
}

func (n *MasterNode) remoteSlaveCount() int {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	return len(n.remoteSlaves)
}

func (n *MasterNode) remoteSlaveExists(id string) bool {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	_, ok := n.remoteSlaves[id]
	return ok
}

func (n *MasterNode) addRemoteSlave(id string, slave actor.Actor) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	n.remoteSlaves[id] = slave
	n.Logger.WithFields(logrus.Fields{
		"id":    id,
		"slave": slave,
	}).Debugln("Added remote slave")
}

func (n *MasterNode) removeRemoteSlave(id string) {
	if n.remoteSlaveExists(id) {
		n.mtx.Lock()
		defer n.mtx.Unlock()
		delete(n.remoteSlaves, id)
	} else {
		n.Logger.WithField("id", id).Errorln("No such slave to remove")
	}
}

func (n *MasterNode) readinessCheckLoop() {
	for {
		select {
		case <-n.readyTicker.C:
			n.checkIfReady()

		case <-n.readyTickerStop:
			return
		}
	}
}

func (n *MasterNode) checkIfReady() {
	if n.remoteSlaveCount() >= n.cfg.Master.ExpectSlaves {
		n.Logger.Infoln("All slaves connected and ready")
		n.Send(n, actor.Message{Type: AllSlavesReady})
		// we no longer need to check if all slaves are ready
		close(n.readyTickerStop)
	}
}

// broadcast will send the given message to all of the remote slaves.
func (n *MasterNode) broadcast(msg actor.Message) {
	for _, rs := range n.remoteSlaves {
		n.Logger.WithFields(logrus.Fields{
			"msg": msg,
			"rs":  rs.GetID(),
		}).Debugln("Broadcasting message to remote slave")
		n.Send(rs, msg)
	}
}

func (n *MasterNode) startLoadTesting() {
	n.Logger.Infoln("Starting load testing")
	// tell all the slaves to start their load testing
	n.broadcast(actor.Message{Type: StartLoadTest})

	// then do another broadcast to receive any initial messages from the slaves
	n.broadcast(actor.Message{Type: RecvMessage})

	// regularly check in with the slaves to see if there are any messages for us
	n.recvTicker = time.NewTicker(MasterSlaveCheckInterval)
	go n.recvCheckLoop()
}

func (n *MasterNode) recvCheckLoop() {
	for {
		select {
		case <-n.recvTicker.C:
			// try to recv any and all data from all slaves
			n.broadcast(actor.Message{Type: RecvMessage})

		case <-n.recvTickerStop:
			return
		}
	}
}

func (n *MasterNode) slaveFinished(msg actor.Message) {
	id := msg.Data.(SlaveIDMessage).ID
	n.Logger.WithField("id", id).Infoln("Slave completed load testing")
	n.finishedCount++
	if n.finishedCount >= n.cfg.Master.ExpectSlaves {
		n.Logger.Infoln("All slaves successfully completed load testing")
		n.shutdownErr = nil
		n.Shutdown()
	}
}

func (n *MasterNode) connectionClosed(msg actor.Message) {
	idMsg, ok := msg.Data.(SlaveIDMessage)
	if ok {
		n.removeRemoteSlave(idMsg.ID)
	} else {
		n.Logger.Errorln("Got connection closed message from unknown source")
	}
}
