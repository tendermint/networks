package loadtest

import (
	"sync"
	"time"

	"github.com/tendermint/networks/pkg/actor"
)

// SlaveState helps in managing the state machine associated with the slave.
type SlaveState string

// SlaveStartCheckInterval specifies how often to keep checking whether the
// master has indicated for us to start the load testing.
const SlaveStartCheckInterval = 1 * time.Second

// SlaveStartCheckRecvTimeout is the read deadline for checking for new messages
// from the master when the slave is waiting to start the load test.
const SlaveStartCheckRecvTimeout = SlaveStartCheckInterval - (100 * time.Millisecond)

// The various states in which a slave can be.
const (
	SlaveStarting    SlaveState = "starting"
	SlaveConnecting  SlaveState = "connecting"
	SlaveWaiting     SlaveState = "waiting"
	SlaveLoadTesting SlaveState = "load-testing"
	SlaveFailing     SlaveState = "failing"
	SlaveCompleting  SlaveState = "completing"
)

// SlaveNode is an actor that provides the actual load testing functionality,
// but receiving instructions and coordination from the master node.
type SlaveNode struct {
	*actor.BaseActor

	cfg    *Config       // Overall load test configuration.
	master *remoteMaster // The remote master node to which this slave is connected.
	state  SlaveState    // The current state of this slave node.
	mtx    *sync.RWMutex

	startCheckTicker *time.Ticker
	startCheckChan   chan struct{}

	shutdownErr error // An error to provide if the slave node shuts down improperly.
}

// NewSlaveNode instantiates a new slave node, but does not start the actor.
func NewSlaveNode(cfg *Config) *SlaveNode {
	n := &SlaveNode{
		cfg:              cfg,
		master:           nil,
		state:            SlaveStarting,
		mtx:              &sync.RWMutex{},
		startCheckTicker: nil,
		startCheckChan:   make(chan struct{}),
		shutdownErr:      nil,
	}
	n.master = newRemoteMaster(cfg.Slave.Master, n)
	n.BaseActor = actor.NewBaseActor(n, "slave")
	return n
}

func (n *SlaveNode) OnStart() error {
	n.setState(SlaveConnecting)
	if err := n.master.Start(); err != nil {
		n.Logger.WithError(err).Errorln("Failed to connect to remote master")
		n.setState(SlaveFailing)
		n.shutdownErr = err
		return err
	}
	// indicate to the remote master that this slave is ready
	n.slaveReady()
	return nil
}

func (n *SlaveNode) OnShutdown() {
	if n.startCheckTicker != nil {
		n.startCheckTicker.Stop()
		close(n.startCheckChan)
	}
	n.master.Shutdown()
	n.master.Wait()
}

// GetShutdownError will retrieve any error that occurred during the slave
// node's lifecycle.
func (n *SlaveNode) GetShutdownError() error {
	return n.shutdownErr
}

// Handle is the primary handler for incoming messages.
func (n *SlaveNode) Handle(msg actor.Message) {
	switch msg.Type {
	case actor.Ping:
		n.Send(msg.Sender, actor.Message{Type: actor.Pong})

	case SlaveAccepted:
		n.slaveAccepted(msg)

	case StartLoadTest:
		n.startLoadTesting()

	case SlaveFinished:
		n.Send(n.master, msg)
		n.shutdownErr = nil
		n.Shutdown()

	case TooManySlaves:
		n.errorAndShutdown(ErrTooManySlaves)

	case AlreadySeenSlave:
		n.errorAndShutdown(ErrAlreadySeenSlave)

	case SlaveFailed:
		n.errorAndShutdown(ErrSlaveFailed)

	case ConnectionClosed:
		n.errorAndShutdown(ErrWebSocketsConnClosed)
	}
}

func (n *SlaveNode) slaveReady() {
	n.Logger.Infoln("Sending slave ready notification")
	n.Send(n.master, actor.Message{Type: SlaveReady, Data: SlaveIDMessage{ID: n.GetID()}})

	// we need to hear a SlaveAccepted (or error) message back from the master
	n.Send(n.master, actor.Message{Type: RecvMessage})
}

func (n *SlaveNode) setState(newState SlaveState) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	n.state = newState
	n.Logger.WithField("state", newState).Debugln("Slave state changed")
}

func (n *SlaveNode) getState() SlaveState {
	n.mtx.RLock()
	defer n.mtx.RUnlock()
	return n.state
}

func (n *SlaveNode) errorAndShutdown(err ErrorCode) {
	n.Logger.Errorln(ErrorMessageForCode(err))
	n.setState(SlaveFailing)
	n.shutdownErr = NewError(err, nil)
	n.Shutdown()
}

func (n *SlaveNode) slaveAccepted(src actor.Message) {
	n.Logger.Infoln("Slave accepted by master")
	n.setState(SlaveWaiting)
	n.startCheckTicker = time.NewTicker(SlaveStartCheckInterval)
	go n.startCheckLoop()
}

func (n *SlaveNode) startCheckLoop() {
	for {
		select {
		case <-n.startCheckChan:
			return

		case <-n.startCheckTicker.C:
			n.Send(
				n.master,
				actor.Message{
					Type: RecvMessage,
					Data: RecvMessageConfig{Timeout: SlaveStartCheckRecvTimeout},
				},
			)
		}
	}
}

func (n *SlaveNode) startLoadTesting() {
	if n.startCheckTicker != nil {
		n.startCheckTicker.Stop()
	}
	n.Logger.Infoln("Starting load testing")

	// TODO: Load testing here

	n.Logger.Infoln("Load testing complete")
	n.Send(n, actor.Message{Type: SlaveFinished, Data: SlaveIDMessage{ID: n.GetID()}})
}
