package loadtest

import (
	"context"
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

	mux           *http.ServeMux
	httpServer    *http.Server
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
		httpServer:    nil,
		bindAddr:      bindAddr,
		clientFactory: clientFactory,
	}
	s.BaseActor = actor.NewBaseActor(s, "websockets-server")
	return s
}

// OnStart will fire up the WebSockets server in a separate goroutine to listen
// for, and handle, incoming connections.
func (s *WebSocketsServer) OnStart() error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/", s.transportHandler)
	s.httpServer = &http.Server{
		Addr:    s.bindAddr,
		Handler: s.mux,
	}

	go func(s_ *WebSocketsServer) {
		s_.Logger.Info("Starting up WebSockets server", "addr", s_.bindAddr)
		if err := s_.httpServer.ListenAndServe(); err != nil {
			s_.Logger.Info("WebSockets server shut down", "err", err)
		}
	}(s)

	return nil
}

// OnShutdown will attempt to cleanly shut down the WebSockets server.
func (s *WebSocketsServer) OnShutdown() error {
	s.Logger.Debug("Shutting down WebSockets server")
	if err := s.httpServer.Shutdown(context.Background()); err != nil {
		s.Logger.Error("Failed to cleanly shut down WebSockets server", "err", err)
	} else {
		s.Logger.Info("WebSockets server successfully shut down")
	}
	return nil
}

// transportHandler is the primary interface that deals with WebSockets connections.
func (s *WebSocketsServer) transportHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.Error("Failed to upgrade incoming WebSockets connection", "err", err)
		return
	}
	client, err := s.clientFactory(conn)
	if err != nil {
		s.Logger.Error("Failed to instantiate actor from client factory to deal with WebSockets connection", "err", err)
		webSocketsClose(conn)
		return
	}
	if err = client.Start(); err != nil {
		s.Logger.Error("Failed to start client actor to deal with WebSockets connection", "err", err)
		webSocketsClose(conn)
		return
	}
	// wait for the client to terminate
	if err = client.Wait(); err != nil {
		s.Logger.Error("Failed when waiting for client to shut down", "err", err)
	}
}
