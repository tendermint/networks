package loadtest

import (
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	webSocketsReadBufferSize  = 10 * 1024 * 1024
	webSocketsWriteBufferSize = 1 * 1024 * 1024
)

func runMaster(logger *log.Entry, config *Config) error {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  webSocketsReadBufferSize,
		WriteBufferSize: webSocketsWriteBufferSize,
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.WithError(err).Errorln("Failed to upgrade incoming connection")
			return
		}
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				logger.WithError(err).Errorln("Failed to read incoming WebSockets message")
			} else {
				if messageType != websocket.TextMessage {
					logger.Errorln("Failed to read incoming message: messages must be text, not binary")
				} else {
					msg := string(p)
					logger.WithField("msg", msg).Debugln("Got incoming WebSockets message")
					err := handleMessage(msg, conn)
					if err != nil {
						logger.WithError(err).Errorln("Failed to handle incoming WebSockets message")
					}
				}
			}
		}
	})
	http.ListenAndServe(config.Master.Bind, nil)
	return nil
}

func handleMessage(msg string, conn *websocket.Conn) error {
	return nil
}
