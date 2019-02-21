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

func (t *testActor) OnShutdown() error {
	close(t.shutdownChan)
	return nil
}

func (t *testActor) Handle(m actor.Message) {
	if m.Type == actor.Ping {
		t.testChan <- actor.Message{Type: actor.Pong}
	}
}

func TestBaseActorLifecycle(t *testing.T) {
	a := newTestActor()
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}

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
	done := make(chan error)
	go func() {
		done <- a.Wait()
	}()

	select {
	case <-a.shutdownChan:
		t.Log("Successfully called OnShutdown()")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for actor to call OnShutdown()")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		} else {
			t.Log("Successfully shut down actor event loop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for test actor to shut down")
	}
}

func TestPoisonPill(t *testing.T) {
	a := newTestActor()
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}

	a.Recv(actor.Message{Type: actor.PoisonPill})

	done := make(chan error)
	go func() {
		done <- a.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		} else {
			t.Log("Successfully poisoned actor and triggered shutdown")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for poison pill to kill actor")
	}
}
