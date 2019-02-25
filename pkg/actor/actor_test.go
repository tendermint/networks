package actor_test

import (
	"testing"
	"time"

	"github.com/tendermint/networks/pkg/actor"
)

type testActor struct {
	*actor.BaseActor

	testChan     chan actor.Message
	subsChan     chan actor.Message
	startupChan  chan bool
	shutdownChan chan bool
}

var _ actor.Actor = (*testActor)(nil)

func newTestActor() *testActor {
	t := &testActor{
		testChan:     make(chan actor.Message, 1),
		subsChan:     make(chan actor.Message, 1),
		startupChan:  make(chan bool, 1),
		shutdownChan: make(chan bool, 1),
	}
	t.BaseActor = actor.NewBaseActor(t, "test")
	return t
}

func (t *testActor) OnStart() error {
	t.startupChan <- true
	return nil
}

func (t *testActor) OnShutdown() error {
	t.shutdownChan <- true
	return nil
}

func (t *testActor) Handle(m actor.Message) {
	switch m.Type {
	case actor.Ping:
		t.testChan <- actor.Message{Type: actor.Pong}

	case actor.SubscriptionMessage:
		t.subsChan <- m
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
	if err := a.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
}

func TestPoisonPill(t *testing.T) {
	a := newTestActor()
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}

	a.Recv(actor.Message{Type: actor.PoisonPill})
	if err := a.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
}

func TestPubSub(t *testing.T) {
	pub := newTestActor()
	if err := pub.Start(); err != nil {
		t.Fatal(err)
	}

	sub := newTestActor()
	if err := sub.Start(); err != nil {
		t.Fatal(err)
	}

	pub.Subscribe(sub, actor.Ping)
	sub.Send(pub, actor.Message{Type: actor.Ping})

	select {
	case msg := <-sub.subsChan:
		if msg.Type != actor.SubscriptionMessage {
			t.Errorf("Incorrect message type received. Expected %s, but got %s", actor.SubscriptionMessage, msg.Type)
		} else {
			t.Log("Successfully received subscription message")
			data, ok := msg.Data.(actor.Message)
			if !ok {
				t.Error("Expected msg.Data to be of type actor.Message, but was not")
			} else {
				if data.Type != actor.Ping {
					t.Errorf("Incorrect embedded message type. Expected %s, but got %s", actor.Ping, data.Type)
				} else {
					if data.Sender.GetID() != sub.GetID() {
						t.Errorf("Incorrect sender for original message. Expected ID %s, but got ID %s", sub.GetID(), data.Sender.GetID())
					}
				}
			}
		}
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for message from subscriber")
	}

	// either way we're done with these actors
	pub.Recv(actor.Message{Type: actor.PoisonPill})
	sub.Recv(actor.Message{Type: actor.PoisonPill})

	if err := pub.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
	if err := sub.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
}

func TestProbe(t *testing.T) {
	pub := newTestActor()
	if err := pub.Start(); err != nil {
		t.Fatal(err)
	}

	sub := actor.NewProbe()
	if err := sub.Start(); err != nil {
		t.Fatal(err)
	}

	pub.Subscribe(sub, actor.Ping)
	pub.Recv(actor.Message{Type: actor.Ping})

	err := sub.WaitForCapturedMessages(1, 100*time.Millisecond)
	if err != nil {
		t.Error(err)
	}

	captured := sub.CapturedMessages()
	if len(captured) != 1 {
		t.Errorf("Expected number of captured messages to be 1, but got %d", len(captured))
	} else {
		if captured[0].Type != actor.Ping {
			t.Errorf("Expected captured message to be of type %s, but got %s", actor.Ping, captured[0].Type)
		}
	}

	// shut the actors down
	pub.Recv(actor.Message{Type: actor.PoisonPill})
	sub.Recv(actor.Message{Type: actor.PoisonPill})

	if err := pub.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
	if err := sub.Wait(1 * time.Second); err != nil {
		t.Error(err)
	}
}
