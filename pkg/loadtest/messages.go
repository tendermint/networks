package loadtest

import (
	"encoding/json"
	"time"

	"github.com/tendermint/networks/pkg/actor"
)

// Message types for the master/slave interaction.
const (
	RecvMessage        actor.MessageType = "recv-message"
	RemoteSlaveStarted actor.MessageType = "remote-slave-started"
	SlaveReady         actor.MessageType = "slave-ready"
	TooManySlaves      actor.MessageType = "too-many-slaves"
	AlreadySeenSlave   actor.MessageType = "already-seen-slave"
	SlaveAccepted      actor.MessageType = "slave-accepted"
	SlaveFailed        actor.MessageType = "slave-failed"
	AllSlavesReady     actor.MessageType = "all-slaves-ready"
	StartLoadTest      actor.MessageType = "start-load-test"
	SlaveFinished      actor.MessageType = "slave-finished"
	ConnectionClosed   actor.MessageType = "connection-closed"
)

type SlaveIDMessage struct {
	ID string `json:"id"`
}

type RecvMessageConfig struct {
	Timeout time.Duration `json:"timeout"`
}

func init() {
	actor.RegisterMessageParser(RemoteSlaveStarted, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(SlaveReady, ParseSlaveIDMessage)
	actor.RegisterMessageParser(TooManySlaves, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(AlreadySeenSlave, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(SlaveAccepted, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(SlaveFailed, ParseSlaveIDMessage)
	actor.RegisterMessageParser(AllSlavesReady, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(StartLoadTest, actor.ParseMessageWithNoData)
	actor.RegisterMessageParser(SlaveFinished, ParseSlaveIDMessage)
}

func ParseSlaveIDMessage(data json.RawMessage) (interface{}, error) {
	msg := SlaveIDMessage{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}
