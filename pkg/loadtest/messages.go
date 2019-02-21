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

	SpawnClients         actor.MessageType = "spawn-clients"
	ClientFailed         actor.MessageType = "client-failed"
	ClientFailedShutdown actor.MessageType = "client-failed-shutdown"
	ClientFinished       actor.MessageType = "client-finished"
	ClientStats          actor.MessageType = "client-stats"
	TestHarnessFinished  actor.MessageType = "test-harness-finished"
)

type SlaveIDMessage struct {
	ID string `json:"id"`
}

type SlaveFinishedMessage struct {
	ID    string `json:"id"`
	Stats ClientSummaryStats
}

type RecvMessageConfig struct {
	Timeout time.Duration `json:"timeout"`
}

type ClientIDMessage struct {
	ID string
}

type ClientStatsMessage struct {
	ID    string
	Stats *ClientSummaryStats
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
	actor.RegisterMessageParser(SlaveFinished, ParseSlaveFinishedMessage)
}

func ParseSlaveIDMessage(data json.RawMessage) (interface{}, error) {
	msg := SlaveIDMessage{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func ParseSlaveFinishedMessage(data json.RawMessage) (interface{}, error) {
	msg := SlaveFinishedMessage{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}
