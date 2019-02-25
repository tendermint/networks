package loadtest

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
)

// remoteSlave allows us to translate messages from the master into WebSockets
// messages, and vice-versa, for communication with a remote slave node.
type remoteSlave struct {
	*actor.BaseActor

	conn   *websocket.Conn
	master *MasterNode
	state  SlaveState
	mtx    *sync.RWMutex
}

func newRemoteSlave(conn *websocket.Conn, master *MasterNode) *remoteSlave {
	s := &remoteSlave{
		conn:   conn,
		master: master,
		state:  SlaveStarting,
		mtx:    &sync.RWMutex{},
	}
	s.BaseActor = actor.NewBaseActor(s, "remote-slave")
	return s
}

func (s *remoteSlave) OnStart() error {
	s.conn.SetCloseHandler(s.connCloseHandler)
	// tell the master we're up
	s.Send(s.master, actor.Message{Type: RemoteSlaveStarted})
	return nil
}

func (s *remoteSlave) connCloseHandler(code int, text string) error {
	s.Logger.Info("Remote side closed the connection")
	s.Send(s, actor.Message{Type: ConnectionClosed})
	return nil
}

func (s *remoteSlave) OnShutdown() error {
	// try to close the websockets connection
	if err := webSocketsClose(s.conn); err != nil {
		s.Logger.Error("Failed to send WebSockets close message", "err", err)
		return err
	}
	return nil
}

func (s *remoteSlave) Handle(msg actor.Message) {
	switch msg.Type {
	case ConnectionClosed:
		// inform the master that the connection was closed
		s.Send(s.master, msg)

	case RecvMessage:
		res := s.recvMessage(msg)

		// peek into the incoming message
		switch res.Type {
		case SlaveReady:
			s.SetID(res.Data.(SlaveIDMessage).ID)
			s.Logger.Info("Slave ready")

		case SlaveFinished:
			s.Logger.Info("Slave finished")
			s.setState(SlaveCompleting)
			s.Shutdown()
			return

		case SlaveFailed:
			s.Logger.Error("Slave failed")
			s.setState(SlaveFailing)
			s.Shutdown()
			return
		}

	default:
		s.sendMessage(msg)

		// peek into the message being sent
		switch msg.Type {
		case TooManySlaves, AlreadySeenSlave, SlaveFailed:
			s.setState(SlaveFailing)
			// kill the connection from the master's side
			s.Shutdown()

		case SlaveAccepted:
			s.setState(SlaveWaiting)

		case StartLoadTest:
			s.setState(SlaveLoadTesting)
		}
	}
}

// func (s *remoteSlave) getState() SlaveState {
// 	s.mtx.RLock()
// 	defer s.mtx.RUnlock()
// 	return s.state
// }

func (s *remoteSlave) setState(newState SlaveState) {
	s.Logger.Debug("Remote slave changing state", "state", newState)
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.state = newState
}

func (s *remoteSlave) recvMessage(src actor.Message) *actor.Message {
	res, err := webSocketsRecv(s.conn)
	if err != nil {
		s.Logger.Error("Failed to recv incoming WebSockets message", "err", err)
		res = nil
	} else {
		res.Sender = s
		s.Send(src.Sender, *res)
	}
	return res
}

func (s *remoteSlave) sendMessage(msg actor.Message) {
	if err := webSocketsSend(s.conn, msg); err != nil {
		s.Logger.Error("Failed to send WebSockets message", "err", err)
	}
}
