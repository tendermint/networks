package loadtest

import (
	"fmt"
	"time"
)

// These event codes double as both internal reactor events, as well as
// WebSockets message envelope types.
const (
	EvOK         = "ok"
	EvPoisonPill = "poison-pill"

	// Errors
	EvInternalServerError    = "internal-server-error"
	EvUnrecognizedEventType  = "unrecognized-event-type"
	EvTimeout                = "timeout"
	EvUnsupportedMessageType = "unsupported-message-type"
	EvParsingFailed          = "parsing-failed"

	// WebSockets client-related messaging
	EvRecvWebSocketsMsg          = "recv-websockets-msg"
	EvSendWebSocketsMsg          = "send-websockets-msg"
	EvWebSocketsReadFailed       = "websockets-read-failed"
	EvWebSocketsWriteFailed      = "websockets-write-failed"
	EvInvalidWebSocketsMsgFormat = "invalid-websockets-msg-format"
	EvGetWebSocketsClientState   = "get-websockets-client-state"
	EvSetWebSocketsClientState   = "set-websockets-client-state"
	EvWebSocketsCloseConn        = "websockets-close-conn"
	EvWebSocketsConnClosed       = "websockets-conn-closed"

	// Slave-related messaging
	EvSlaveReady            = "slave-ready"
	EvSlaveAccepted         = "slave-accepted"
	EvSlaveAlreadySeen      = "slave-already-seen"
	EvTooManySlaves         = "too-many-slaves"
	EvStartLoadTest         = "start-load-test"
	EvSlaveStartedLoadTest  = "slave-started-load-test"
	EvSlaveFinishedLoadTest = "slave-finished-load-test"
	EvSlaveFailed           = "slave-failed"
	EvSlaveLoadTestStats    = "slave-load-test-stats"
)

var errorEvents = map[string]struct{}{
	EvInternalServerError:        {},
	EvUnrecognizedEventType:      {},
	EvTimeout:                    {},
	EvUnsupportedMessageType:     {},
	EvParsingFailed:              {},
	EvWebSocketsReadFailed:       {},
	EvWebSocketsWriteFailed:      {},
	EvInvalidWebSocketsMsgFormat: {},
	EvSlaveAlreadySeen:           {},
	EvTooManySlaves:              {},
	EvSlaveFailed:                {},
	EvWebSocketsConnClosed:       {},
}

var noDataEvents = map[string]struct{}{
	EvSlaveAccepted:        {},
	EvStartLoadTest:        {},
	EvSlaveStartedLoadTest: {},
}

type EventError struct {
	Error string
}

type EventSlaveReady struct {
	ID string
}

type EventSlaveLoadTestStats struct {
	Timestamp time.Time
}

func eventDescription(t string, additionalInfo ...string) string {
	var result string
	switch t {
	case EvOK:
		result = "OK"
	case EvPoisonPill:
		result = "Poison pill (internal event to shut down reactor)"

	case EvInternalServerError:
		result = "Internal server error"
	case EvUnrecognizedEventType:
		result = "Unrecognized event type"
	case EvTimeout:
		result = "Operation timed out"
	case EvUnsupportedMessageType:
		result = "Unsupported message type"
	case EvParsingFailed:
		result = "Failed to parse incoming message"

	case EvRecvWebSocketsMsg:
		result = "Receive message over WebSockets connection from client"
	case EvSendWebSocketsMsg:
		result = "Send message over WebSockets connection to client"
	case EvWebSocketsReadFailed:
		result = "Failed to read from WebSockets connection"
	case EvWebSocketsWriteFailed:
		result = "Failed to write to WebSockets connection"
	case EvInvalidWebSocketsMsgFormat:
		result = "Invalid WebSockets message format (non-text)"
	case EvGetWebSocketsClientState:
		result = "Get WebSockets client state"
	case EvSetWebSocketsClientState:
		result = "Set WebSockets client state"
	case EvWebSocketsCloseConn:
		result = "Close WebSockets connection"
	case EvWebSocketsConnClosed:
		result = "WebSockets connection closed"

	case EvSlaveReady:
		result = "Slave is ready to start load test"
	case EvSlaveAccepted:
		result = "Slave has been accepted and will be instructed to start load test once all slaves are ready"
	case EvSlaveAlreadySeen:
		result = "Slave with that ID already seen"
	case EvTooManySlaves:
		result = "Too many slaves connected - rejecting connection"
	case EvStartLoadTest:
		result = "Master broadcast message to start load testing across slaves"
	case EvSlaveStartedLoadTest:
		result = "Slave started load test"
	case EvSlaveFinishedLoadTest:
		result = "Slave successfully finished load test"
	case EvSlaveFailed:
		result = "Slave failed"
	case EvSlaveLoadTestStats:
		result = "Slave load test statistics"

	default:
		return "Unrecognized event type"
	}
	if len(additionalInfo) > 0 {
		result = fmt.Sprintf("%s: %s", result, additionalInfo[0])
	}
	return result
}

// IsError is a convenience function to aid in checking whether a particular
// event type is actually an error of some sort.
func (e ReactorEvent) IsError() bool {
	_, ok := errorEvents[e.Type]
	return ok
}

// ProvidesData is a convenience function to check if a particular event
// actually provides data in the Data field. Otherwise one can expect the Data
// field to be empty, and the corresponding WebSockets message's Message field
// too.
func (e ReactorEvent) ProvidesData() bool {
	_, ok := noDataEvents[e.Type]
	return !ok
}
