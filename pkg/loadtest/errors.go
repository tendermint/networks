package loadtest

import "fmt"

// ErrorCode allows us to encapsulate specific failure codes for the node
// processes.
type ErrorCode int

// Error/exit codes for load testing-related errors.
const (
	NoError ErrorCode = iota
	ErrFailedToDecodeConfig
	ErrFailedToReadConfigFile
	ErrInvalidConfig
	ErrUnrecognizedWebSocketsMessage
	ErrUnrecognizedReactorMessageType
	ErrSlaveFailed
	ErrFailedToConnectToMaster
	ErrSlaveRejected
	ErrUnsupportedWebSocketsMessageType
	ErrWebSocketsConnClosed
	ErrTooManySlaves
	ErrAlreadySeenSlave
	ErrClientFailed
	ErrLongerThanTimeout
	ErrFailedToStartTestHarness
	ErrMissingMessageField
	ErrMissingSlaveStats
	ErrRemoteSlavesShutdownFailed
	ErrMasterKilled

	ErrKVStoreClientPutFailed
	ErrKVStoreClientGetFailed
)

// Error is a way of wrapping the meaningful exit code we want to provide on
// failure.
type Error struct {
	Code     ErrorCode
	Message  string
	Upstream error
}

// Error implements error.
var _ error = (*Error)(nil)

// NewError allows us to create new Error structures from the given code and
// upstream error (can be nil).
func NewError(code ErrorCode, upstream error, additionalInfo ...string) *Error {
	return &Error{
		Code:     code,
		Message:  ErrorMessageForCode(code, additionalInfo...),
		Upstream: upstream,
	}
}

// Error implements error.
func (e Error) Error() string {
	if e.Upstream != nil {
		return fmt.Sprintf("%s. Caused by: %s", e.Message, e.Upstream.Error())
	}
	return e.Message
}

// ErrorMessageForCode translates the given error code into a human-readable,
// English message.
func ErrorMessageForCode(code ErrorCode, additionalInfo ...string) string {
	var result string
	switch code {
	case NoError:
		result = "No error"
	case ErrFailedToDecodeConfig:
		result = "Failed to decode TOML configuration"
	case ErrFailedToReadConfigFile:
		result = "Failed to read configuration file"
	case ErrUnrecognizedWebSocketsMessage:
		result = "Unrecognized WebSockets message type"
	case ErrUnrecognizedReactorMessageType:
		result = "Unrecognized reactor message type"
	case ErrSlaveFailed:
		result = "Slave failed"
	case ErrFailedToConnectToMaster:
		result = "Failed to connect to master"
	case ErrSlaveRejected:
		result = "Slave rejected by master"
	case ErrUnsupportedWebSocketsMessageType:
		result = "Unsupported WebSockets message type (must be text)"
	case ErrWebSocketsConnClosed:
		result = "WebSockets connection closed"
	case ErrTooManySlaves:
		result = "Too many slaves connected"
	case ErrAlreadySeenSlave:
		result = "Another slave with the same ID is connected to the master"
	case ErrClientFailed:
		result = "Load testing client failed"
	case ErrLongerThanTimeout:
		result = "Time given is longer than timeout"
	case ErrFailedToStartTestHarness:
		result = "Failed to start test harness"
	case ErrInvalidConfig:
		result = "Invalid configuration"
	case ErrMissingMessageField:
		result = "Message is missing a required field"
	case ErrMissingSlaveStats:
		result = "Missing statistics from slave node"
	case ErrRemoteSlavesShutdownFailed:
		result = "Remote slaves shutdown failed"
	case ErrMasterKilled:
		result = "Master node was killed"

	case ErrKVStoreClientPutFailed:
		result = "KVStore client \"put\" request failed"
	case ErrKVStoreClientGetFailed:
		result = "KVStore client \"get\" request failed"
	default:
		return "Unrecognized error"
	}
	if len(additionalInfo) > 0 {
		result = fmt.Sprintf("%s: %s", result, additionalInfo[0])
	}
	return result
}

// IsErrorCode is a convenience function that attempts to cast the given error
// to an Error struct and checks its error code against the given code.
func IsErrorCode(err error, code ErrorCode) bool {
	e, ok := err.(Error)
	if ok {
		return e.Code == code
	}
	return false
}
