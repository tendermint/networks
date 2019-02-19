package actor

import (
	"encoding/json"
	"fmt"
)

// MessageType allows us to restrict the types of messages actors can send to
// one another.
type MessageType string

// Generic message types for actor communication.
const (
	Ping       MessageType = "ping"
	Pong       MessageType = "pong"
	PoisonPill MessageType = "poison-pill"
)

// MaxDeadLetterMessages puts a maximum size on the dead letter inbox.
const MaxDeadLetterMessages = 1000

type baseMessage struct {
	Type MessageType `json:"type"`           // What type of message is this?
	Data interface{} `json:"data,omitempty"` // The data/contents of the message, if any.
}

// Message encapsulates a message that can be sent between actors.
type Message struct {
	Type   MessageType // What type of message is this?
	Data   interface{} // The data/contents of the message, if any.
	Sender Actor       // The actor that sent the message.
}

// MessageParser is a callback function that allows us to parse a specific
// message type.
type MessageParser func(data json.RawMessage) (interface{}, error)

// MessageParserRegistry keeps track of the various different message types' parsers.
type MessageParserRegistry struct {
	Parsers map[MessageType]MessageParser
}

var messageParserRegistry = &MessageParserRegistry{
	Parsers: make(map[MessageType]MessageParser),
}

var deadLetterInbox = make(chan Message, MaxDeadLetterMessages)

func init() {
	RegisterMessageParser(Ping, ParseMessageWithNoData)
	RegisterMessageParser(Pong, ParseMessageWithNoData)
	RegisterMessageParser(PoisonPill, ParseMessageWithNoData)
}

// DeadLetterInbox provides access to the global dead letter inbox for
// undelivered messages.
func DeadLetterInbox() chan Message {
	return deadLetterInbox
}

// RouteToDeadLetterInbox is a convenience function that will route a message to
// the dead letter inbox unless it's full, in which case it will drop the
// message.
func RouteToDeadLetterInbox(msg Message) {
	select {
	case deadLetterInbox <- msg:
	default:
		// Drop the message because the dead letter inbox is full
	}
}

func (m Message) String() string {
	return fmt.Sprintf("Message{Type: \"%s\", Data: %v, Sender: \"%v\"}", m.Type, m.Data, m.Sender.GetID())
}

// Error will return an error if the data contained in the message is an error,
// otherwise it returns nil.
func (m Message) Error() error {
	if err, ok := m.Data.(error); ok {
		return err
	}
	return nil
}

// ParseJSONMessage will attempt to parse the given JSON string as a message.
func ParseJSONMessage(s string) (*Message, error) {
	var raw json.RawMessage
	m := baseMessage{
		Data: &raw,
	}
	if err := json.Unmarshal([]byte(s), m); err != nil {
		return nil, err
	}
	parser, ok := messageParserRegistry.Parsers[m.Type]
	if !ok {
		return nil, fmt.Errorf("Unrecognized message type: %s", m.Type)
	}
	d, err := parser(raw)
	if err != nil {
		return nil, err
	}
	return &Message{Type: m.Type, Data: d}, nil
}

// ToJSON is a utility function that assists in converting the message to a JSON
// string.
func (m Message) ToJSON() (string, error) {
	data, err := json.Marshal(baseMessage{Type: m.Type, Data: m.Data})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Reply is a convenience function that will first check if this message has a
// sender, and if it does, will pass the given message on to its Recv method. If
// there is no sender attached to this message, then the message is sent to the
// dead letter inbox.
func (m Message) Reply(msg Message) {
	if m.Sender != nil {
		m.Sender.Recv(msg)
	} else {
		RouteToDeadLetterInbox(msg)
	}
}

// RegisterMessageParser registers (adds or overrides) the parser for the given
// message type.
func RegisterMessageParser(mt MessageType, p MessageParser) {
	messageParserRegistry.Parsers[mt] = p
}

// ParseMessageWithNoData assumes that the Message.Data field is of no
// consequence, and thus returns nil for the data field.
func ParseMessageWithNoData(data json.RawMessage) (interface{}, error) {
	return nil, nil
}
