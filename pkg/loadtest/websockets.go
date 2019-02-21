package loadtest

import (
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
)

// Default configuration for WebSockets interactions.
const (
	DefaultWebSocketsReadDeadline  = 3 * time.Second
	DefaultWebSocketsWriteDeadline = 3 * time.Second
)

func webSocketsRecv(conn *websocket.Conn, timeouts ...time.Duration) (*actor.Message, error) {
	timeout := DefaultWebSocketsReadDeadline
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	mt, r, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	switch mt {
	case websocket.TextMessage:
		return actor.ParseJSONMessage(string(r))

	case websocket.CloseMessage:
		return nil, NewError(ErrWebSocketsConnClosed, nil)

	default:
		return nil, NewError(ErrUnsupportedWebSocketsMessageType, nil)
	}
}

func webSocketsSend(conn *websocket.Conn, msg actor.Message, timeouts ...time.Duration) error {
	timeout := DefaultWebSocketsWriteDeadline
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	data, err := msg.ToJSON()
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, []byte(data))
}

func webSocketsClose(conn *websocket.Conn) error {
	return conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

func ensureWebSocketsAddr(addr string) string {
	parts := strings.Split(addr, "://")
	if len(parts) == 1 {
		return fmt.Sprintf("ws://%s", addr)
	}
	return addr
}
