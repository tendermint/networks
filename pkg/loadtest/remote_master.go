package loadtest

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
)

// remoteMaster encapsulates a master node to which we're connecting via a
// WebSockets connection. This actor handles the translation between actor
// messages and WebSockets messages.
type remoteMaster struct {
	*actor.BaseActor

	addr  string          // The remote address of the master node
	conn  *websocket.Conn // The WebSockets connection.
	slave *SlaveNode      // The parent slave node that instantiated this remote master.
}

func newRemoteMaster(addr string, slave *SlaveNode) *remoteMaster {
	m := &remoteMaster{
		addr:  ensureWebSocketsAddr(addr),
		conn:  nil,
		slave: slave,
	}
	m.BaseActor = actor.NewBaseActor(m, "remoteMaster")
	return m
}

func (m *remoteMaster) OnStart() error {
	m.Logger.WithField("addr", m.addr).Infoln("Attempting to connect to master")
	conn, _, err := websocket.DefaultDialer.Dial(m.addr, nil)
	if err != nil {
		return err
	}
	m.conn = conn
	m.conn.SetCloseHandler(m.connCloseHandler)
	return nil
}

func (m *remoteMaster) connCloseHandler(code int, text string) error {
	m.Logger.Infoln("Remote side closed the connection")
	m.Send(m, actor.Message{Type: ConnectionClosed})
	return nil
}

func (m *remoteMaster) Handle(msg actor.Message) {
	switch msg.Type {
	// If the connection was closed by the remote side
	case ConnectionClosed:
		// tell the slave node that the connection was closed
		m.Send(m.slave, msg)

	// Attempt to receive a message from the remote master
	case RecvMessage:
		timeout := DefaultWebSocketsReadDeadline
		cfg, ok := msg.Data.(RecvMessageConfig)
		if ok {
			timeout = cfg.Timeout
		}
		res := m.recvMessage(msg, timeout)

		// peek into the response from the master
		switch res.Type {
		case TooManySlaves, AlreadySeenSlave, SlaveFailed:
			m.Logger.WithField("type", res.Type).Errorln("Master notified us of failure")
			m.Shutdown()
			return
		}

	// Send the given message to the remote master
	default:
		m.sendMessage(msg)

		// peek into the message being sent
		switch msg.Type {
		case SlaveFinished:
			if err := webSocketsClose(m.conn); err != nil {
				m.Logger.WithError(err).Errorln("Failed to cleanly close WebSockets connection")
			}
			m.Shutdown()
			return

		case SlaveFailed:
			m.Logger.Errorln("Slave failed")
			m.Shutdown()
			return
		}
	}
}

func (m *remoteMaster) recvMessage(src actor.Message, timeouts ...time.Duration) *actor.Message {
	res, err := webSocketsRecv(m.conn, timeouts...)
	if err != nil {
		m.Logger.WithError(err).Errorln("Failed to recv incoming WebSockets message")
		res = nil
	} else {
		res.Sender = m
		src.Reply(*res)
	}
	return res
}

func (m *remoteMaster) sendMessage(msg actor.Message) {
	if err := webSocketsSend(m.conn, msg); err != nil {
		m.Logger.WithError(err).Errorln("Failed to send WebSockets message")
	} else {
		m.Logger.WithField("msg", msg).Debugln("Sent message")
	}
}
