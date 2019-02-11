package loadtest

import "fmt"

// Error/exit codes for load testing-related errors.
const (
	NoError int = iota
	ErrFailedToDecodeConfig
	ErrFailedToReadConfigFile
	ErrUnrecognizedWebSocketsMessage
)

// Error is a way of wrapping the meaningful exit code we want to provide on
// failure.
type Error struct {
	Code     int
	Message  string
	Upstream error
}

// Error implements error.
var _ error = (*Error)(nil)

// NewError allows us to create new Error structures from the given code and
// upstream error (can be nil).
func NewError(code int, upstream error) *Error {
	return &Error{
		Code:     code,
		Message:  ErrorMessageForCode(code),
		Upstream: upstream,
	}
}

// Error implements error.
func (e *Error) Error() string {
	if e.Upstream != nil {
		return fmt.Sprintf("%s. Caused by: %s", e.Message, e.Upstream.Error())
	}
	return e.Message
}

// ErrorMessageForCode translates the given error code into a human-readable,
// English message.
func ErrorMessageForCode(code int) string {
	switch code {
	case NoError:
		return "No error"
	case ErrFailedToDecodeConfig:
		return "Failed to decode TOML configuration"
	case ErrFailedToReadConfigFile:
		return "Failed to read configuration file"
	case ErrUnrecognizedWebSocketsMessage:
		return "Unrecognized WebSockets message type"
	default:
		return "Unrecognized error"
	}
}
