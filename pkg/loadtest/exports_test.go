package loadtest

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/tendermint/networks/pkg/actor"
)

type ABCIQueryHTTPResponse = abciQueryHTTPResponse
type ABCIQueryResult = abciQueryResult
type ABCIQueryResponse = abciQueryResponse

func WebSocketsRecv(conn *websocket.Conn, timeouts ...time.Duration) (*actor.Message, error) {
	return webSocketsRecv(conn, timeouts...)
}

func WebSocketsSend(conn *websocket.Conn, msg actor.Message, timeouts ...time.Duration) error {
	return webSocketsSend(conn, msg, timeouts...)
}

func WebSocketsClose(conn *websocket.Conn) error {
	return webSocketsClose(conn)
}
