package loadtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// webSocketsClient allows us to encapsulate a connected client in such a way
// that we can use a single event loop to interact with the WebSocket
// connection, because it's not thread-safe.
type webSocketsClient struct {
	*BaseReactor

	id     string
	conn   *websocket.Conn
	logger *logrus.Entry
	master *MasterNode // So we can communicate with the master node.
	state  SlaveState  // What's the current state of the client in its state machine?
}

// WebSocketsReactor is a kind of reactor that deals exclusively with the
// management of a WebSockets connection and its lifecycle.
type WebSocketsReactor struct {
	*BaseReactor

	conn *websocket.Conn // The WebSockets connection.
}

// Networking deadlines for WebSockets-related communications.
const (
	DefaultWebSocketsReadDeadline      = time.Duration(3) * time.Second
	DefaultWebSocketsWriteDeadline     = time.Duration(3) * time.Second
	DefaultWebSocketsReadWriteDeadline = DefaultWebSocketsReadDeadline + DefaultWebSocketsWriteDeadline
)

type webSocketsLoopHandler func(*webSocketsClient) bool

var _ Reactor = (*webSocketsClient)(nil)
var _ Reactor = (*WebSocketsReactor)(nil)

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
	loop:
		for {
			if !client.handleWebSockets() {
				client.Shutdown()
				break loop
			}
		}
		m.Logger.Debugln("Terminated client WebSockets handler loop")
		// wait for the client reactor loop to finish shutting down
		client.Wait()
		if err = conn.Close(); err != nil {
			m.Logger.WithError(err).Errorln("Failed to close WebSockets connection")
		}
		m.Logger.Debugln("Closed WebSockets connection")
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
		id:     "no-id-yet",
		conn:   conn,
		logger: logrus.WithField("ctx", "webSocketsClient"),
		master: m,
	}
	c.BaseReactor = NewBaseReactor(c, "websockets")
	return c
}

func (c *webSocketsClient) OnShutdown() {
	if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		c.logger.WithError(err).Errorln("Failed to write close message to WebSockets client")
	}
}

func (c *webSocketsClient) GetState() SlaveState {
	r := c.RecvAndAwait(ReactorEvent{Type: EvGetWebSocketsClientState})
	return r.Data.(SlaveState)
}

func (c *webSocketsClient) SetState(state SlaveState) {
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
	ev := c.RecvAndAwait(ReactorEvent{Type: EvRecvWebSocketsMsg, Timeout: timeout}, timeout)
	// pass the resulting event on to the client event loop
	c.Recv(ev)
}

// This routine translates WebSockets-based interactions into ReactorEvents and
// vice-versa.
func (c *webSocketsClient) handleWebSockets() bool {
	c.logger.WithField("state", c.GetState()).Debugln("handleWebSockets()")
	switch c.GetState() {
	// if the client has just connected
	case SSConnected:
		return c.HandleInitialSlaveConnect()

	// if we're waiting for the master to give us the go-ahead.
	case SSAccepted:
		return true

	// if the client is busy with load testing, continue.
	case SSTesting:
		// check if more stats have come through
		c.HandleSlaveStatsCollection()
		return true
	}
	c.logger.Debugln("Terminating WebSockets client connection")
	// terminate the WebSockets connection
	return false
}

//
// WebSocketsReactor public methods
//

// NewWebSocketsReactor will instantiate a new reactor that deals exclusively
// with WebSockets connections and communication. This offers thread safety in
// the use of the WebSockets connection, which is inherently not thread-safe.
func NewWebSocketsReactor(conn *websocket.Conn) *WebSocketsReactor {
	r := &WebSocketsReactor{
		conn: conn,
	}
	r.BaseReactor = NewBaseReactor(r, "websockets")
	return r
}

// Handle will handle the internal event loop for the WebSockets reactor.
func (r *WebSocketsReactor) Handle(e ReactorEvent) {
	switch e.Type {
	case EvRecvWebSocketsMsg:
		r.recvWebSocketsMsg(e)

	case EvSendWebSocketsMsg:
		r.sendWebSocketsMsg(e)

	case EvWebSocketsCloseConn:
		r.closeConn(e)

	case EvWebSocketsConnClosed:
		r.Shutdown()
	}
}

// RecvMsg will wait for an incoming message over the WebSockets connection.
func (r *WebSocketsReactor) RecvMsg(timeouts ...time.Duration) (*WebSocketsMsgEnvelope, error) {
	timeout := DefaultWebSocketsReadDeadline
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}
	e := r.RecvAndAwait(ReactorEvent{Type: EvRecvWebSocketsMsg, Timeout: timeout})
	if e.IsError() {
		return nil, e.Data.(error)
	}
	return &WebSocketsMsgEnvelope{Type: e.Type, Message: e.Data}, nil
}

// SendMsg will attempt to send the given message over the WebSockets
// connection.
func (r *WebSocketsReactor) SendMsg(msg *WebSocketsMsgEnvelope, timeouts ...time.Duration) error {
	timeout := DefaultWebSocketsWriteDeadline
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}
	msgEv := ReactorEvent{Type: msg.Type, Data: msg.Message}
	e := r.RecvAndAwait(ReactorEvent{Type: EvSendWebSocketsMsg, Data: msgEv, Timeout: timeout})
	if e.IsError() {
		return e.Data.(error)
	}
	return nil
}

//
// WebSocketsReactor private methods
//

func (r *WebSocketsReactor) recvWebSocketsMsg(e ReactorEvent) {
	timeout := DefaultWebSocketsReadDeadline
	if e.Timeout > 0 {
		timeout = e.Timeout
	}
	r.conn.SetReadDeadline(time.Now().Add(timeout))

	// we expect the client to speak to us first
	mt, p, err := r.conn.ReadMessage()
	if err != nil {
		e.Respond(ReactorEvent{Type: EvWebSocketsReadFailed, Data: err})
	}

	switch mt {
	case websocket.TextMessage:
		// try to decode the incoming message
		env, err := parseWebSocketsMsgEnvelope(string(p))
		if err != nil {
			e.Respond(ReactorEvent{Type: EvParsingFailed, Data: err})
		}
		e.Respond(ReactorEvent{Type: env.Type, Data: env.Message})

	case websocket.CloseMessage:
		res := ReactorEvent{Type: EvWebSocketsConnClosed}
		e.Respond(res)
		r.Recv(res)

	default:
		e.Respond(ReactorEvent{Type: EvInvalidWebSocketsMsgFormat, Data: fmt.Errorf("Invalid WebSockets message format: %d", mt)})
	}
}

func (r *WebSocketsReactor) sendWebSocketsMsg(e ReactorEvent) {
	raw, err := json.Marshal(&WebSocketsMsgEnvelope{Type: e.Type, Message: e.Data})
	if err == nil {
		timeout := DefaultWebSocketsWriteDeadline
		if e.Timeout > 0 {
			timeout = e.Timeout
		}
		r.conn.SetWriteDeadline(time.Now().Add(timeout))
		err = r.conn.WriteMessage(websocket.TextMessage, raw)
	}
	if err != nil {
		e.Respond(ReactorEvent{Type: EvWebSocketsWriteFailed, Data: err})
	} else {
		e.Respond(ReactorEvent{Type: EvOK})
	}
}

func (r *WebSocketsReactor) closeConn(e ReactorEvent) {
	r.logger.Debugln("Closing WebSockets reactor connection")
	if err := r.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		r.logger.WithError(err).Errorln("Failed to close WebSockets connection")
		e.Respond(ReactorEvent{Type: EvWebSocketsWriteFailed, Data: err})
	} else {
		r.logger.Debugln("Successfully closed WebSockets connection")
		e.Respond(ReactorEvent{Type: EvOK})
	}
	r.Shutdown()
}
