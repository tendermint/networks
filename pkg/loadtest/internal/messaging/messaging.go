package messaging

import (
	"encoding/json"

	"github.com/tendermint/networks/pkg/loadtest"
	mt "github.com/tendermint/networks/pkg/loadtest/internal/messaging/types"
)

// Envelope encapsulates any kind of WebSockets-based message that can be sent
// to/from the slave/master.
type Envelope struct {
	Type    string      // What type of message is this? This is to decode the "message" field.
	Message interface{} // The actual message itself.
}

// Error represents an error message to/from the slave/master.
type Error struct {
	Message string // The error message.
}

// NewError creates a new WebSockets error envelope.
func NewError(msg string) *Envelope {
	return &Envelope{
		Type: mt.Error,
		Message: Error{
			Message: msg,
		},
	}
}

// Unmarshal attempts to dynamically unmarshal the given JSON string into one of
// the recognised message types.
func Unmarshal(s string) (*Envelope, error) {
	var msg json.RawMessage
	env := Envelope{
		Message: &msg,
	}
	if err := json.Unmarshal([]byte(s), env); err != nil {
		return nil, err
	}
	switch env.Type {
	case mt.Error:
		var e Error
		if err := json.Unmarshal(msg, &e); err != nil {
			return nil, err
		}
		env.Message = e
		return &env, nil
	default:
		return nil, loadtest.NewError(loadtest.ErrUnrecognizedWebSocketsMessage, nil)
	}
}

// Marshal will assist in marshaling the given envelope to a JSON string.
func (e *Envelope) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
