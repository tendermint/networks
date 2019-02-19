package loadtest

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
)

// WebSocket server-related constants.
const (
	DefaultWebSocketsReadBufSize  = 1024 * 1024
	DefaultWebSocketsWriteBufSize = 1024 * 1024
)

// WebSocketsClientFactory allows us to spawn/retrieve actors to handle the
// event loop of interacting through a WebSockets client connection.
type WebSocketsClientFactory func(*websocket.Conn) (actor.Actor, error)

// WebSocketsServer is an actor that translates actor messages into WebSockets
// ones, and vice-versa.
type WebSocketsServer struct {
	*actor.BaseActor

	bindAddr      string // The network address to which we must bind.
	clientFactory WebSocketsClientFactory
}

// WebSocketsServer implements actor.Actor
var _ actor.Actor = (*WebSocketsServer)(nil)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  DefaultWebSocketsReadBufSize,
	WriteBufferSize: DefaultWebSocketsWriteBufSize,
}

// NewWebSocketsServer instantiates an actor that handles the running of a
// WebSockets server.
func NewWebSocketsServer(bindAddr string, clientFactory WebSocketsClientFactory) *WebSocketsServer {
	s := &WebSocketsServer{
		bindAddr:      bindAddr,
		clientFactory: clientFactory,
	}
	s.BaseActor = actor.NewBaseActor(s, "websockets-server")
	return s
}

// OnStart will fire up the WebSockets server in a separate goroutine to listen
// for, and handle, incoming connections.
func (s *WebSocketsServer) OnStart() error {
	http.HandleFunc("/", s.transportHandler)

	go func(s_ *WebSocketsServer) {
		http.ListenAndServe(s_.bindAddr, nil)
	}(s)

	return nil
}

// transportHandler is the primary interface that deals with WebSockets connections.
func (s *WebSocketsServer) transportHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.WithError(err).Errorln("Failed to upgrade incoming WebSockets connection")
		return
	}
	client, err := s.clientFactory(conn)
	if err != nil {
		s.Logger.WithError(err).Errorln("Failed to instantiate actor from client factory to deal with WebSockets connection")
		webSocketsClose(conn)
		return
	}
	if err = client.Start(); err != nil {
		s.Logger.WithError(err).Errorln("Failed to start client actor to deal with WebSockets connection")
		webSocketsClose(conn)
		return
	}
	// wait for the client to terminate
	client.Wait()
}
