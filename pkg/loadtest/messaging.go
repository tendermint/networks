package loadtest

import (
	"encoding/json"
)

// WebSocketsMsgEnvelope is the envelope structure for messages being sent
// between master and slave nodes, and allows for flexible decoding of JSON
// message structures.
type WebSocketsMsgEnvelope struct {
	Type    string
	Message interface{}
}

func parseWebSocketsMsgEnvelope(s string) (*WebSocketsMsgEnvelope, error) {
	var msg json.RawMessage
	env := &WebSocketsMsgEnvelope{
		Message: &msg,
	}
	if err := json.Unmarshal([]byte(s), env); err != nil {
		return nil, err
	}
	e := ReactorEvent{Type: env.Type}
	if e.IsError() {
		var b EventError
		if err := json.Unmarshal(msg, &b); err != nil {
			return nil, err
		}
		env.Message = &b
	} else {
		if !e.ProvidesData() {
			// no need to worry about deserializing a message body
			env.Message = nil
		} else {
			switch env.Type {
			case EvSlaveReady:
				var b EventSlaveReady
				if err := json.Unmarshal(msg, &b); err != nil {
					return nil, err
				}
				env.Message = &b

			case EvSlaveLoadTestStats:
				var b EventSlaveLoadTestStats
				if err := json.Unmarshal(msg, &b); err != nil {
					return nil, err
				}
				env.Message = &b

			default:
				return nil, NewError(ErrUnrecognizedWebSocketsMessage, nil, env.Type)
			}
		}
	}

	return env, nil
}
