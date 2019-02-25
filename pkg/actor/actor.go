package actor

import (
	"fmt"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/tendermint/networks/internal/logging"
)

// Constants relating to actors.
const (
	DefaultActorInboxSize = 5
)

// Actor is an implementation of the actor pattern.
type Actor interface {
	GetID() string
	SetID(id string)

	// Primary handler method for dealing with incoming messages to the actor's
	// inbox.
	Handle(m Message)

	// Recv allows another actor to send this actor a message by putting it into
	// its inbox.
	Recv(m Message)

	Start() error
	FailAndShutdown(err error)
	Shutdown()
	Wait(timeouts ...time.Duration) error

	// Lifecycle events
	OnStart() error
	OnShutdown() error

	// Pub/Sub functionality
	Subscribe(sub Actor, msgTypes ...MessageType)
	Unsubscribe(sub Actor, msgTypes ...MessageType)
}

var _ Actor = (*BaseActor)(nil)

// BaseActor is a convenience class from which we can derive other actors. This
// is effectively an "abstract class" and should be treated as such.
type BaseActor struct {
	Logger logging.Logger

	impl Actor  // The implementation/derived class for this actor.
	ctx  string // The context string for logging.
	id   string // The primary ID for this actor.

	inboxChan            chan Message
	shutdownChan         chan bool
	shutdownCompleteChan chan bool
	shutdownStarted      bool
	shutdownCompleted    bool
	shutdownErr          error

	subscriptions map[MessageType]map[string]Actor // MessageType -> Actor.GetID() -> Actor

	mtx *sync.RWMutex
}

// NewBaseActor instantiates a BaseActor that can be used to provide the basic
// event handling functionality. The `inboxSizes` variable is optional, and if
// not specified the actor's inbox will default to `DefaultActorInboxSize`.
func NewBaseActor(impl Actor, ctx string, inboxSizes ...int) *BaseActor {
	inboxSize := DefaultActorInboxSize
	if len(inboxSizes) > 0 {
		inboxSize = inboxSizes[0]
	}
	id := uuid.NewV4().String()
	a := &BaseActor{
		impl:                 impl,
		ctx:                  ctx,
		id:                   id,
		Logger:               logging.NewLogrusLogger(ctx, "id", id),
		inboxChan:            make(chan Message, inboxSize),
		shutdownChan:         make(chan bool, 2),
		shutdownCompleteChan: make(chan bool, 2),
		shutdownStarted:      false,
		shutdownCompleted:    false,
		shutdownErr:          nil,
		subscriptions:        make(map[MessageType]map[string]Actor),
		mtx:                  &sync.RWMutex{},
	}
	return a
}

// OnStart is called prior to starting the actor's internal event loop.
func (a *BaseActor) OnStart() error { return nil }

// OnShutdown is called when Shutdown is called. If OnShutdown returns an error,
// this will override any other error set by FailAndShutdown.
func (a *BaseActor) OnShutdown() error { return nil }

// Start will initiate this actor's event loop in a separate goroutine. If the
// impl.OnStart method fails, the error will be returned and the event loop will
// not be started.
func (a *BaseActor) Start() error {
	a.Logger.Debug("Starting up", "id", a.GetID())
	if a.impl != nil {
		if err := a.impl.OnStart(); err != nil {
			a.Logger.Error("OnStart event for actor failed", "err", err)
			return err
		}
	}

	// kick off the internal event loop
	go a.eventLoop()

	a.Logger.Debug("Actor started up")

	return nil
}

func (a *BaseActor) eventLoop() {
loop:
	for {
		select {
		case m := <-a.inboxChan:
			a.Handle(m)
		case <-a.shutdownChan:
			break loop
		}
	}
	a.shutdownCompleteChan <- true
}

// FailAndShutdown allows us to shut the actor down with the given error. This
// helps to understand why exactly an actor shut down.
func (a *BaseActor) FailAndShutdown(err error) {
	a.setShutdownErr(err)
	a.Shutdown()
}

func (a *BaseActor) setShutdownErr(err error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.shutdownErr = err
}

func (a *BaseActor) getShutdownErr() error {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.shutdownErr
}

// Shutdown initiates the shutdown process (asynchronously) for this actor. If
// you want to know once the shutdown process is complete, use the Wait()
// function.
func (a *BaseActor) Shutdown() {
	if !a.isShutdownStarted() {
		if a.impl != nil {
			if err := a.impl.OnShutdown(); err != nil {
				a.setShutdownErr(err)
			}
		}
		a.shutdownChan <- true
		a.setShutdownStarted(true)
	}
}

// Wait will block the current goroutine and wait until the actor's shutdown
// process is complete.
func (a *BaseActor) Wait(timeouts ...time.Duration) error {
	if !a.isShutdownCompleted() {
		if len(timeouts) > 0 {
			a.Logger.Debug("Waiting for actor to shut down", "timeout", timeouts[0])
			select {
			case <-a.shutdownCompleteChan:
				a.Logger.Debug("Actor successfully shut down")

			case <-time.After(timeouts[0]):
				a.setShutdownErr(fmt.Errorf("Timed out (%s) waiting for actor to shut down", timeouts[0]))
			}
		} else {
			a.Logger.Debug("Waiting for actor to shut down")
			<-a.shutdownCompleteChan
			a.Logger.Debug("Actor successfully shut down")
		}
		a.setShutdownCompleted(true)
	}
	return a.getShutdownErr()
}

func (a *BaseActor) isShutdownStarted() bool {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.shutdownStarted
}

func (a *BaseActor) setShutdownStarted(newVal bool) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.shutdownStarted = newVal
}

func (a *BaseActor) isShutdownCompleted() bool {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.shutdownCompleted
}

func (a *BaseActor) setShutdownCompleted(newVal bool) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.shutdownCompleted = newVal
}

// GetID will retrieve the actor's ID, which should be universally unique.
func (a *BaseActor) GetID() string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.id
}

// SetID allows us to override the ID of the actor.
func (a *BaseActor) SetID(id string) {
	a.mtx.Lock()
	a.id = id
	a.mtx.Unlock()
	// reconfigure the logger
	a.Logger.SetField("id", id)
}

// Handle will, by default, just check if we have an implementation actor class
// and calls that implementation's Handle method.
func (a *BaseActor) Handle(m Message) {
	a.publishToSubscribers(m)

	switch m.Type {
	case PoisonPill:
		a.Shutdown()

	default:
		if a.impl != nil {
			a.impl.Handle(m)
		}
	}
}

// Recv allows BaseActor to queue an incoming message to be handled by its event
// loop.
func (a *BaseActor) Recv(msg Message) {
	a.inboxChan <- msg
}

// Send is a convenience function to send a message to the given actor with this
// actor as the sender.
func (a *BaseActor) Send(other Actor, msg Message) {
	msg.Sender = a
	if other != nil {
		other.Recv(msg)
	} else {
		RouteToDeadLetterInbox(msg)
	}
}

// Subscribe will set up a subscription such that, when this actor receives
// messages of the given types, they will be published to `sub`.
func (a *BaseActor) Subscribe(sub Actor, msgTypes ...MessageType) {
	a.Logger.Debug("Subscribing actor to incoming message types", "id", sub.GetID(), "msgTypes", msgTypes)
	a.mtx.Lock()
	defer a.mtx.Unlock()
	for _, mt := range msgTypes {
		if _, ok := a.subscriptions[mt]; !ok {
			a.subscriptions[mt] = make(map[string]Actor)
		}
		a.subscriptions[mt][sub.GetID()] = sub
	}
}

// Unsubscribe will remove any subscriptions to the given message types by the
// given subscriber.
func (a *BaseActor) Unsubscribe(sub Actor, msgTypes ...MessageType) {
	a.Logger.Debug("Unsubscribing actor from incoming message types", "id", sub.GetID(), "msgTypes", msgTypes)
	a.mtx.Lock()
	defer a.mtx.Unlock()
	for _, mt := range msgTypes {
		if _, ok := a.subscriptions[mt]; ok {
			delete(a.subscriptions[mt], sub.GetID())
		}
	}
}

func (a *BaseActor) publishToSubscribers(msg Message) {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	if _, ok := a.subscriptions[msg.Type]; ok {
		for _, sub := range a.subscriptions[msg.Type] {
			a.Send(sub, Message{Type: SubscriptionMessage, Data: msg})
		}
	}
}
