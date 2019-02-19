package actor_test

import (
	"testing"
	"time"

	"github.com/tendermint/networks/pkg/actor"
)

type testActor struct {
	*actor.BaseActor

	testChan     chan actor.Message
	startupChan  chan struct{}
	shutdownChan chan struct{}
}

var _ actor.Actor = (*testActor)(nil)

func newTestActor() *testActor {
	t := &testActor{
		testChan:     make(chan actor.Message),
		startupChan:  make(chan struct{}),
		shutdownChan: make(chan struct{}),
	}
	t.BaseActor = actor.NewBaseActor(t, "test")
	return t
}

func (t *testActor) OnStart() error {
	close(t.startupChan)
	return nil
}

func (t *testActor) OnShutdown() {
	close(t.shutdownChan)
}

func (t *testActor) Handle(m actor.Message) {
	if m.Type == actor.Ping {
		t.testChan <- actor.Message{Type: actor.Pong}
	}
}

func TestBaseActorLifecycle(t *testing.T) {
	a := newTestActor()
	a.Start()

	select {
	case <-a.startupChan:
		t.Log("Successfully called OnStart()")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for actor to call OnStart()")
	}

	a.Recv(actor.Message{Type: actor.Ping})

	select {
	case m := <-a.testChan:
		if m.Type == actor.Pong {
			t.Log("Successfully received anticipated pong message")
		} else {
			t.Fatalf("Unrecognized message type: %s", m.Type)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for test actor to respond")
	}

	a.Shutdown()
	done := make(chan struct{})
	go func() {
		a.Wait()
		close(done)
	}()

	select {
	case <-a.shutdownChan:
		t.Log("Successfully called OnShutdown()")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for actor to call OnShutdown()")
	}

	select {
	case <-done:
		t.Log("Successfully shut down actor event loop")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for test actor to shut down")
	}
}

func TestPoisonPill(t *testing.T) {
	a := newTestActor()
	a.Start()

	a.Recv(actor.Message{Type: actor.PoisonPill})

	done := make(chan struct{})
	go func() {
		a.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Successfully poisoned actor and triggered shutdown")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for poison pill to kill actor")
	}
}
