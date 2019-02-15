package loadtest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type webSocketsClientState string

// webSocketsClient allows us to encapsulate a connected client in such a way
// that we can use a single event loop to interact with the WebSocket
// connection, because it's not thread-safe.
type webSocketsClient struct {
	*BaseReactor

	conn   *websocket.Conn
	logger *logrus.Entry
	master *MasterNode           // So we can communicate with the master node.
	state  webSocketsClientState // What's the current state of the client in its state machine?
}

const (
	csConnected webSocketsClientState = "connected" // The client has just connected to the master.
	csRejected  webSocketsClientState = "rejected"  // The client has been rejected for some reason.
	csAccepted  webSocketsClientState = "accepted"  // The client has been accepted and the master is waiting for all slaves to connect.
	csFailed    webSocketsClientState = "failed"    // The client failed in some way.
	csTesting   webSocketsClientState = "testing"   // Load testing is underway.
	csFinished  webSocketsClientState = "finished"  // Load testing is finished.
)

// Networking deadlines for WebSockets-related communications.
const (
	DefaultWebSocketsReadDeadline      = time.Duration(3) * time.Second
	DefaultWebSocketsWriteDeadline     = time.Duration(3) * time.Second
	DefaultWebSocketsReadWriteDeadline = DefaultWebSocketsReadDeadline + DefaultWebSocketsWriteDeadline
)

type webSocketsLoopHandler func(*webSocketsClient) bool

var _ Reactor = (*webSocketsClient)(nil)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024,
	WriteBufferSize: 1024 * 1024,
}

// Creates an HTTP handler function to drive the master node's WebSockets server.
func makeNodeWebSocketsHandler(m *MasterNode) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.Logger.WithError(err).Errorln("Failed to upgrade incoming connection")
			return
		}

		client := newWebSocketsClient(conn, m)
		client.Start()
		for {
			if !client.handleWebSockets() {
				client.Shutdown()
				break
			}
		}
		client.Wait()
	}
}

func sendWebSocketsMsgEnvelope(conn *websocket.Conn, env *WebSocketsMsgEnvelope) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, []byte(raw))
}

//
// webSocketsClient implementation
//

func newWebSocketsClient(conn *websocket.Conn, m *MasterNode) *webSocketsClient {
	c := &webSocketsClient{
		BaseReactor: NewBaseReactor(nil),
		conn:        conn,
		logger:      logrus.WithField("ctx", "webSocketsClient"),
		master:      m,
	}
	c.BaseReactor.impl = c
	return c
}

func (c *webSocketsClient) Handle(e ReactorEvent) {
	switch e.Type {
	// internal events to the WebSockets client
	case EvRecvWebSocketsMsg:
		c.recvWebSocketsMsg(e)
	case EvSendWebSocketsMsg:
		ev, ok := e.Data.(ReactorEvent)
		if ok {
			c.sendWebSocketsMsg(ev)
		} else {
			c.logger.WithField("e.Type", e.Type).Errorln("Failed to convert reactor event data to reactor event")
		}
	case EvGetWebSocketsClientState:
		e.Respond(ReactorEvent{Type: EvGetWebSocketsClientState, Data: c.state})
	case EvSetWebSocketsClientState:
		c.state = e.Data.(webSocketsClientState)
		e.Respond(ReactorEvent{Type: EvOK})

	case EvSlaveReady:
		r := c.master.RecvAndAwait(e)
		e.Respond(r)
		if r.Type == EvSlaveAccepted {
			c.state = csAccepted
		} else {
			c.state = csRejected
		}

	case EvStartLoadTest:
		// pass the test start message on to the client
		c.sendWebSocketsMsg(e)

	case EvSlaveStartedLoadTest:
		c.state = csTesting

	case EvSlaveFailed:
		c.state = csFailed
		// indicate to the master that this slave failed, so it can stop the
		// load test across the other slaves
		c.master.RecvAndAwait(e)

	default:
		e.Respond(ReactorEvent{Type: EvUnrecognizedEventType, Data: e.Type})
	}
}

func (c *webSocketsClient) GetState() webSocketsClientState {
	r := c.RecvAndAwait(ReactorEvent{Type: EvGetWebSocketsClientState})
	return r.Data.(webSocketsClientState)
}

func (c *webSocketsClient) SetState(state webSocketsClientState) {
	c.RecvAndAwait(ReactorEvent{Type: EvSetWebSocketsClientState, Data: state})
}

func (c *webSocketsClient) HandleInitialSlaveConnect() bool {
	// we assume that the client needs to send an event message first
	ev := c.RecvAndAwait(ReactorEvent{Type: EvRecvWebSocketsMsg})
	if ev.IsError() {
		// no point in continuing with this slave
		return false
	}
	// pass the message on to the client's reactor for handling
	res := c.RecvAndAwait(ReactorEvent{Type: ev.Type, Data: ev.Data})
	if res.IsError() {
		// we can still continue interacting with the client
		return true
	}
	// pass the message to the client reactor to be sent out via the WebSockets connection
	c.Recv(ReactorEvent{Type: EvSendWebSocketsMsg, Data: res})
	return true
}

func (c *webSocketsClient) HandleSlaveStatsCollection() {
	// we want a longer timeout here, as we won't get stats very frequently from the client
	timeout := time.Duration(60) * time.Second
	ev := c.RecvAndAwait(ReactorEvent{Type: EvRecvWebSocketsMsg, Timeout: &timeout}, timeout)
	// pass the resulting event on to the client event loop
	c.Recv(ev)
}

// This routine translates WebSockets-based interactions into ReactorEvents and
// vice-versa.
func (c *webSocketsClient) handleWebSockets() bool {
	switch c.GetState() {
	// if the client has just connected
	case csConnected:
		return c.HandleInitialSlaveConnect()

	// if we're waiting for the master to give us the go-ahead.
	case csAccepted:
		return true

	// if the client is busy with load testing, continue.
	case csTesting:
		// check if more stats have come through
		c.HandleSlaveStatsCollection()
		return true
	}
	// terminate the WebSockets connection
	return false
}

func (c *webSocketsClient) recvWebSocketsMsg(e ReactorEvent) {
	timeout := DefaultWebSocketsReadDeadline
	if e.Timeout != nil {
		timeout = *e.Timeout
	}
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	// we expect the client to speak to us first
	mt, p, err := c.conn.ReadMessage()
	if err != nil {
		c.logger.WithError(err).Errorln("Failed to read message from WebSockets client")
		e.Respond(ReactorEvent{Type: EvWebSocketsReadFailed, Data: err})
		return
	}

	if mt != websocket.TextMessage {
		c.logger.Errorln("Client sent non-text message")
		e.Respond(ReactorEvent{Type: EvInvalidWebSocketsMsgFormat})
		return
	}

	// try to decode the incoming message
	env, err := parseWebSocketsMsgEnvelope(string(p))
	if err != nil {
		c.logger.WithError(err).WithField("env", string(p)).Errorln("Failed to parse incoming WebSockets message envelope")
		e.Respond(ReactorEvent{Type: EvParsingFailed, Data: err})
		return
	}

	// respond with the envelope
	e.Respond(ReactorEvent{Type: env.Type, Data: env.Message})
}

func (c *webSocketsClient) sendWebSocketsMsg(e ReactorEvent) {
	raw, err := json.Marshal(&WebSocketsMsgEnvelope{Type: e.Type, Message: e.Data})
	if err == nil {
		timeout := DefaultWebSocketsWriteDeadline
		if e.Timeout != nil {
			timeout = *e.Timeout
		}
		c.conn.SetReadDeadline(time.Now().Add(timeout))

		err = c.conn.WriteMessage(websocket.TextMessage, raw)
	}
	if err != nil {
		e.Respond(ReactorEvent{Type: EvWebSocketsWriteFailed, Data: err})
	} else {
		e.Respond(ReactorEvent{Type: EvOK})
	}
}
