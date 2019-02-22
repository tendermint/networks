package loadtest

import (
	"math"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/tendermint/networks/pkg/actor"
)

// TestHarnessShutdownTimeLimit is the maximum amount of time we'll wait for all
// clients to completely shut down when the test harness is shutting down.
const TestHarnessShutdownTimeLimit = 1 * time.Minute

// TestHarness is an actor that will manage the load testing process. When the
// test harness actor is started, it will immediately start instantiating
// clients and doing the actual load testing.
type TestHarness struct {
	*actor.BaseActor

	Cfg *Config // Global configuration. Must be publicly accessible to allow for other implementations of TestHarnessClientFactory.

	parent              *SlaveNode
	clientFactory       TestHarnessClientFactory
	clientSpawnTicker   *time.Ticker
	clientSpawnStopChan chan bool
	clientSpawnRate     int // The calculated rate at which we should be spawning clients each tick.
	clientsSpawned      int // A counter to keep track of how many clients we've spawned.
	clients             map[string]*TestHarnessClient

	stats *ClientSummaryStats // For keeping track of the summary stats from the clients.

	mtx *sync.RWMutex
}

// NewTestHarness allows one to instantiate a new test harness using the given
// client factory.
func NewTestHarness(parent *SlaveNode, clientFactory TestHarnessClientFactory) *TestHarness {
	// TODO: Find a more rational/scientific way of calculating a good inbox
	// size for the test harness' client messaging channel.
	inboxSize := actor.DefaultActorInboxSize
	minInboxSize := int(float64(parent.cfg.Clients.Spawn) * 1.5)
	if minInboxSize > inboxSize {
		inboxSize = minInboxSize
	}

	th := &TestHarness{
		Cfg:                 parent.cfg,
		parent:              parent,
		clientFactory:       clientFactory,
		clientSpawnTicker:   nil,
		clientSpawnStopChan: make(chan bool, 2),
		clientSpawnRate:     0,
		clientsSpawned:      0,
		clients:             make(map[string]*TestHarnessClient),
		stats: &ClientSummaryStats{
			Interactions: NewSummaryStats(time.Duration(parent.cfg.Clients.InteractionTimeout)),
			Requests:     make(map[string]*SummaryStats),
		},
		mtx: &sync.RWMutex{},
	}
	th.BaseActor = actor.NewBaseActor(th, "test-harness", inboxSize)
	return th
}

// OnStart kicks off the ticker that will periodically spawn a number of clients
// according to the configuration.
func (th *TestHarness) OnStart() error {
	tickerInterval := 1
	if th.Cfg.Clients.SpawnRate < 1.0 {
		tickerInterval = int(math.Round(1.0 / float64(th.Cfg.Clients.SpawnRate)))
		th.clientSpawnRate = 1
	} else {
		th.clientSpawnRate = int(math.Round(float64(th.Cfg.Clients.SpawnRate)))
	}
	th.Logger.Info("Test harness starting up", "tickerInterval", tickerInterval, "spawnRate", th.clientSpawnRate)
	th.clientSpawnTicker = time.NewTicker(time.Duration(tickerInterval) * time.Second)
	go th.clientSpawnLoop()
	return nil
}

// OnShutdown stops the client spawn process, if it is currently running.
func (th *TestHarness) OnShutdown() error {
	if th.clientSpawnTicker != nil {
		th.clientSpawnTicker.Stop()
		th.clientSpawnStopChan <- true
	}

	// wait for all of the clients to shut down
	th.waitForAllClients()

	return nil
}

// Handle will handle incoming messages in the actor's event loop.
func (th *TestHarness) Handle(msg actor.Message) {
	switch msg.Type {
	case SpawnClients:
		th.spawnClients()

	case ClientStats:
		th.clientStats(msg)

	case ClientFinished:
		th.clientFinished(msg)

	case ClientFailed:
		th.clientFailed(msg)
		th.FailAndShutdown(NewError(ErrClientFailed, nil))

	case TestHarnessFinished:
		th.Logger.Info("Test harness completed successfully")
		th.Send(th.parent, msg)
		th.Shutdown()
	}
}

func (th *TestHarness) clientSpawnLoop() {
	for {
		select {
		case <-th.clientSpawnTicker.C:
			th.Send(th, actor.Message{Type: SpawnClients})

		case <-th.clientSpawnStopChan:
			return
		}
	}
}

func (th *TestHarness) spawnClients() {
	if th.clientsSpawned < th.Cfg.Clients.Spawn {
		toSpawn := th.clientSpawnRate
		// we want to have precisely `th.cfg.Clients.Spawn` clients spawned
		if (th.clientsSpawned + toSpawn) > th.Cfg.Clients.Spawn {
			toSpawn = th.Cfg.Clients.Spawn - th.clientsSpawned
		}
		for i := 0; i < toSpawn; i++ {
			client := th.clientFactory(th)
			th.addClient(client)
			if err := client.Start(); err != nil {
				th.Logger.Error("Failed to start client", "err", err)
				th.Shutdown()
			}
		}
		th.clientsSpawned += toSpawn
		th.Logger.Info("Spawned clients", "clientsSpawned", th.clientsSpawned)
	}
}

// addClient will add the given test harness client to our internal map of
// clients, making sure it has a unique ID.
func (th *TestHarness) addClient(c *TestHarnessClient) {
	clientID := c.GetID()
	// make sure we don't have an ID collision
	for th.hasClient(clientID) {
		// generate a new ID for the client
		clientID = uuid.NewV4().String()
		c.SetID(clientID)
	}

	th.mtx.Lock()
	defer th.mtx.Unlock()
	th.clients[clientID] = c
}

// broadcast allows us to broadcast a message to all of the clients we've
// created.
func (th *TestHarness) broadcast(msg actor.Message) {
	th.mtx.Lock()
	defer th.mtx.Unlock()

	th.Logger.Debug("Broadcasting message to all clients", "msg", msg)
	for _, client := range th.clients {
		th.Send(client, msg)
	}
}

func (th *TestHarness) clientFinished(msg actor.Message) {
	data := msg.Data.(ClientStatsMessage)
	id := data.ID
	th.stats.Merge(data.Stats)
	th.removeClient(id)
	// if all the clients have finished
	if th.clientCount() == 0 {
		th.Send(th, actor.Message{Type: TestHarnessFinished, Data: *th.stats})
	}
}

func (th *TestHarness) hasClient(id string) bool {
	th.mtx.RLock()
	defer th.mtx.RUnlock()
	_, ok := th.clients[id]
	return ok
}

func (th *TestHarness) removeClient(id string) {
	th.mtx.Lock()
	defer th.mtx.Unlock()
	_, ok := th.clients[id]
	if ok {
		delete(th.clients, id)
	}
}

func (th *TestHarness) clientCount() int {
	th.mtx.RLock()
	defer th.mtx.RUnlock()
	return len(th.clients)
}

func (th *TestHarness) clientFailed(msg actor.Message) {
	th.Logger.Error("Client failed, shutting down test harness", "id", msg.Sender.GetID())
	th.broadcast(actor.Message{Type: ClientFailedShutdown})
}

func (th *TestHarness) waitForAllClients() {
	// wait for all of the clients to have shut down
	allClientsGone := make(chan bool, 1)
	go func() {
		for th.clientCount() > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		allClientsGone <- true
	}()

	select {
	case <-allClientsGone:
		th.Logger.Info("All clients successfully shut down")

	case <-time.After(TestHarnessShutdownTimeLimit):
		th.Logger.Error("Failed to shut down all clients before stopping test harness")
	}
}

func (th *TestHarness) clientStats(msg actor.Message) {
	stats := msg.Data.(ClientStatsMessage).Stats
	th.stats.Merge(stats)
}
