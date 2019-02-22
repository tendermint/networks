package loadtest

import (
	"os"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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
	readyTickerStop chan bool              // To stop the readiness checker.
	recvTicker      *time.Ticker           // To keep checking for updates from the slaves.
	recvTickerStop  chan bool              // To kill the recv checker.
	finishedCount   int                    // To keep track of how many slaves have successfully completed their load testing.
	stats           *ClientSummaryStats    // The amalgamated statistics across all slaves' clients.
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
		readyTickerStop: make(chan bool, 2),
		recvTicker:      nil,
		recvTickerStop:  make(chan bool, 2),
		finishedCount:   0,
		stats: &ClientSummaryStats{
			Interactions: NewSummaryStats(time.Duration(cfg.Clients.InteractionTimeout)),
			Requests:     make(map[string]*SummaryStats),
		},
		mtx: &sync.RWMutex{},
	}
	n.BaseActor = actor.NewBaseActor(n, "master")
	return n
}

// OnStart will start the WebSockets server and the readiness check loop, which
// keeps checking whether all slaves have connected and the load testing is
// ready to begin.
func (n *MasterNode) OnStart() error {
	n.wss = NewWebSocketsServer(n.cfg.Master.Bind, n.wsClientFactory)
	if err := n.wss.Start(); err != nil {
		n.Logger.Error("Failed to start WebSockets server", "err", err)
		n.wss = nil
		return err
	}
	n.readyTicker = time.NewTicker(MasterReadyCheckInterval)
	go n.readinessCheckLoop()
	return nil
}

// OnShutdown will stop the WebSockets server and any tickers that are still
// currently running.
func (n *MasterNode) OnShutdown() error {
	var err error

	if n.wss != nil {
		n.wss.Shutdown()
		err = n.wss.Wait()
	}
	if n.readyTicker != nil {
		n.readyTicker.Stop()
	}
	if n.recvTicker != nil {
		n.recvTickerStop <- true
		n.recvTicker.Stop()
	}

	// wait for all the slaves to shut down
	n.waitForSlaves()

	n.Logger.Debug("Master node shut down", "err", err)
	return err
}

// Handle will interpret incoming messages in the master's event loop.
func (n *MasterNode) Handle(msg actor.Message) {
	n.Logger.Debug("Got message", "msg", msg)
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
			n.Logger.Error("Unrecognized sender for AllSlavesReady message", "id", msg.Sender.GetID())
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
	n.Logger.Info("Got SlaveReady notification from remote slave", "id", id)

	// first check if we need any more slaves
	if n.remoteSlaveCount() >= n.cfg.Master.ExpectSlaves {
		n.Logger.Info("Slave tried to connect, but we already have enough")
		n.Send(msg.Sender, actor.Message{Type: TooManySlaves})
		return
	}
	// then check if we've seen this slave before
	if n.remoteSlaveExists(id) {
		n.Logger.Info("Already seen slave before", "id", id)
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
	n.Logger.Info("Informing all slaves of failure")

	// tell all the slaves that one of them failed
	n.broadcast(actor.Message{Type: SlaveFailed})

	// shut down the master too
	n.FailAndShutdown(NewError(ErrSlaveFailed, nil))
}

func (n *MasterNode) waitForSlaves() {
	n.Logger.Debug("Waiting for slaves to shut down")
	startTime := time.Now()
loop:
	for {
		if n.remoteSlaveCount() == 0 {
			n.Logger.Info("All slaves successfully shut down")
			break loop
		}
		time.Sleep(100 * time.Millisecond)
		if time.Since(startTime) > RemoteSlaveWaitTimeout {
			n.Logger.Error("Timed out waiting for remote slaves to shut down")
			n.FailAndShutdown(NewError(ErrRemoteSlavesShutdownFailed, nil, "timed out"))
			break loop
		}
	}
}

func (n *MasterNode) remoteSlaveCount() int {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	return len(n.remoteSlaves)
}

func (n *MasterNode) remoteSlaveExists(id string) bool {
	n.mtx.RLock()
	_, ok := n.remoteSlaves[id]
	n.mtx.RUnlock()
	return ok
}

func (n *MasterNode) addRemoteSlave(id string, slave actor.Actor) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	n.remoteSlaves[id] = slave
}

func (n *MasterNode) removeRemoteSlave(id string) {
	if n.remoteSlaveExists(id) {
		n.mtx.Lock()
		delete(n.remoteSlaves, id)
		n.mtx.Unlock()
		n.Logger.Debug("Removed remote slave", "id", id)
	} else {
		n.Logger.Error("No such slave to remove", "id", id)
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
		n.Logger.Info("All slaves connected and ready")
		n.Send(n, actor.Message{Type: AllSlavesReady})
		// we no longer need to check if all slaves are ready
		n.readyTickerStop <- true
	}
}

// broadcast will send the given message to all of the remote slaves.
func (n *MasterNode) broadcast(msg actor.Message) {
	for _, rs := range n.remoteSlaves {
		n.Logger.Debug("Broadcasting message to remote slave", "msg", msg, "rs", rs.GetID())
		n.Send(rs, msg)
	}
}

func (n *MasterNode) startLoadTesting() {
	n.Logger.Info("Starting load testing")
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
	n.Logger.Debug("In slaveFinished()")
	data := msg.Data.(SlaveFinishedMessage)
	id := data.ID
	stats := data.Stats
	if stats.Interactions == nil || len(stats.Requests) == 0 {
		n.Logger.Error("Missing statistics from slave node")
		n.FailAndShutdown(NewError(ErrMissingSlaveStats, nil, id))
		return
	}

	n.Logger.Debug("About to combine stats")
	n.stats.Combine(&stats)
	n.removeRemoteSlave(id)
	n.Logger.Info("Slave completed load testing", "id", id)

	n.finishedCount++
	if n.finishedCount >= n.cfg.Master.ExpectSlaves {
		n.allSlavesFinished()
	}
}

func (n *MasterNode) connectionClosed(msg actor.Message) {
	idMsg, ok := msg.Data.(SlaveIDMessage)
	if ok {
		n.removeRemoteSlave(idMsg.ID)
	} else {
		n.Logger.Error("Got connection closed message from unknown source")
	}
}

func (n *MasterNode) allSlavesFinished() {
	n.Logger.Info("All slaves successfully completed load testing")
	filename := path.Join(n.cfg.Master.ResultsDir, "summary.csv")
	f, err := os.Create(filename)
	if err != nil {
		n.Logger.Error("Failed to create summary output file", "file", filename, "err", err)
	} else {
		if err = n.stats.WriteSummary(f); err != nil {
			n.Logger.Error("Failed to write stats summary to file", "file", filename, "err", err)
		} else {
			n.Logger.Info("Wrote summary stats CSV file", "file", filename)
		}
	}
	n.Shutdown()
}
