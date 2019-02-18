package actor

import (
	"testing"
	"time"
)

type testActor struct {
	*BaseActor

	testChan     chan Message
	startupChan  chan struct{}
	shutdownChan chan struct{}
}

var _ Actor = (*testActor)(nil)

func newTestActor() *testActor {
	t := &testActor{
		testChan:     make(chan Message),
		startupChan:  make(chan struct{}),
		shutdownChan: make(chan struct{}),
	}
	t.BaseActor = NewBaseActor(t)
	return t
}

func (t *testActor) OnStart() error {
	close(t.startupChan)
	return nil
}

func (t *testActor) OnShutdown() {
	close(t.shutdownChan)
}

func (t *testActor) Handle(m Message) {
	if m.Type == Ping {
		t.testChan <- Message{Type: Pong}
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

	a.Recv(Message{Type: Ping})

	select {
	case m := <-a.testChan:
		if m.Type == Pong {
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
