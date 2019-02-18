package actor

import "fmt"

// MessageType allows us to restrict the types of messages actors can send to
// one another.
type MessageType string

// Generic message types for actor communication.
const (
	Ping       MessageType = "ping"
	Pong       MessageType = "pong"
	PoisonPill MessageType = "poison-pill"
)

// Message encapsulates a message that can be sent between actors.
type Message struct {
	Type   MessageType // What type of message is this?
	Data   interface{} // The data/contents of the message, if any.
	Sender Actor       // The actor that sent the message.
}

func (m *Message) String() string {
	return fmt.Sprintf("Message{Type: \"%s\", Data: %v, Sender: \"%v\"}", m.Type, m.Data, m.Sender.GetID())
}

// Error will return an error if the data contained in the message is an error,
// otherwise it returns nil.
func (m *Message) Error() error {
	if err, ok := m.Data.(error); ok {
		return err
	}
	return nil
}
