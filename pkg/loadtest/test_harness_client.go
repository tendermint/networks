package loadtest

import (
	"time"

	"github.com/tendermint/networks/pkg/actor"
)

// ClientStatsReportingFrequency specifies the number of interactions after
// which we'll send a stats update back to the test harness.
const ClientStatsReportingFrequency = 100

// TestHarnessClient is the entity responsible for interacting with the remote
// Tendermint network.
type TestHarnessClient struct {
	*actor.BaseActor

	parent            *TestHarness
	interactor        TestHarnessClientInteractor
	clientStopChan    chan bool
	clientStoppedChan chan bool
}

// TestHarnessClientFactory provides a way for us to spawn clients for a
// particular test harness.
type TestHarnessClientFactory func(*TestHarness) *TestHarnessClient

// TestHarnessClientInteractor defines the required interface for an object that
// is called to interact with remote Tendermint nodes.
type TestHarnessClientInteractor interface {
	// Init allows us to set up the interactor. If Init fails, it will cause the
	// entire load test to fail.
	Init() error

	// Interact must contain the necessary logic to perform a single interaction
	// with a foreign Tendermint node. This interaction can be a series of
	// request/response interactions, and thus allows for returning a series of
	// request statistics (one for each request performed during an
	// interaction). Each request must be named, where the keys of the map
	// returned must be a unique identifier for the request.
	Interact()

	// GetStats must return a mapping of request IDs to summary statistics for
	// each of those requests.
	GetStats() map[string]*SummaryStats

	// Shutdown allows the interactor to clean up as the client is being shut
	// down.
	Shutdown() error
}

// NewTestHarnessClient allows us to instantiate a new test harness client.
func NewTestHarnessClient(parent *TestHarness, interactor TestHarnessClientInteractor) *TestHarnessClient {
	c := &TestHarnessClient{
		parent:            parent,
		interactor:        interactor,
		clientStopChan:    make(chan bool, 2),
		clientStoppedChan: make(chan bool, 2),
	}
	c.BaseActor = actor.NewBaseActor(c, "test-harness-client")
	return c
}

// OnStart effectively just calls the interactor's Init function.
func (c *TestHarnessClient) OnStart() error {
	if err := c.interactor.Init(); err != nil {
		return err
	}
	// spawn the client interaction loop immediately
	go c.clientInteractionLoop()
	return nil
}

// OnShutdown calls the interactor's Shutdown function.
func (c *TestHarnessClient) OnShutdown() error {
	// tell the client loop to stop
	c.clientStopChan <- true
	// wait for it to stop
	<-c.clientStoppedChan
	return c.interactor.Shutdown()
}

// Handle deals with messages coming into the test harness client's inbox.
func (c *TestHarnessClient) Handle(msg actor.Message) {
	switch msg.Type {
	// We finished load testing successfully.
	case ClientFinished:
		c.Send(c.parent, msg)
		c.Shutdown()

	// We failed.
	case ClientFailed:
		c.Send(c.parent, msg)
		c.FailAndShutdown(NewError(ErrClientFailed, nil))

	// Another client failed, but we need to shut down.
	case ClientFailedShutdown:
		c.FailAndShutdown(NewError(ErrClientFailed, nil))
	}
}

func (c *TestHarnessClient) clientInteractionLoop() {
	istats := NewSummaryStats(time.Duration(c.parent.Cfg.Clients.InteractionTimeout))
	maxInt := int64(c.parent.Cfg.Clients.MaxInteractions)

	testStopTime := time.Now().Add(time.Duration(c.parent.Cfg.Clients.MaxTestTime)).UnixNano()
	completed := true

	istats.Count = 0

loop:
	for {
		if maxInt != -1 && istats.Count >= maxInt {
			break loop
		}

		startTime := time.Now().UnixNano()
		c.interactor.Interact()
		now := time.Now().UnixNano()
		timeTaken := now - startTime
		istats.AddNano(timeTaken)

		// if we've run for longer than the maximum test time
		if now >= testStopTime {
			break loop
		}

		// periodically report back on stats
		if (istats.Count % ClientStatsReportingFrequency) == 0 {
			stats := &ClientSummaryStats{
				Interactions: &(*istats),
				Requests:     c.interactor.GetStats(),
			}
			stats.Compute()
			c.Send(c, actor.Message{Type: ClientStats, Data: ClientStatsMessage{ID: c.GetID(), Stats: stats}})
		}

		// check if we've gotten a signal from outside to exit
		select {
		case <-c.clientStopChan:
			completed = false
			break loop
		default:
		}
	}

	stats := &ClientSummaryStats{
		Interactions: &(*istats),
		Requests:     c.interactor.GetStats(),
	}
	stats.Compute()

	if completed {
		c.Send(c, actor.Message{Type: ClientFinished, Data: ClientStatsMessage{ID: c.GetID(), Stats: stats}})
	} else {
		c.Send(c, actor.Message{Type: ClientStats, Data: ClientStatsMessage{ID: c.GetID(), Stats: stats}})
	}
	c.clientStoppedChan <- true
}
