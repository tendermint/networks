package actor

import (
	"sync"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// Constants relating to actors.
const (
	DefaultActorInboxSize = 1000
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
	Wait() error

	// Lifecycle events
	OnStart() error
	OnShutdown() error
}

var _ Actor = (*BaseActor)(nil)

// BaseActor is a convenience class from which we can derive other actors. This
// is effectively an "abstract class" and should be treated as such.
type BaseActor struct {
	Logger *logrus.Entry

	impl Actor  // The implementation/derived class for this actor.
	ctx  string // The context string for logging.
	id   string // The primary ID for this actor.

	inboxChan            chan Message
	shutdownChan         chan struct{}
	shutdownCompleteChan chan struct{}
	shutdownStarted      bool
	shutdownCompleted    bool
	shutdownErr          error

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
		Logger:               makeActorLogger(ctx, id),
		inboxChan:            make(chan Message, inboxSize),
		shutdownChan:         make(chan struct{}),
		shutdownCompleteChan: make(chan struct{}),
		shutdownStarted:      false,
		shutdownCompleted:    false,
		shutdownErr:          nil,
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
	if a.impl != nil {
		if err := a.impl.OnStart(); err != nil {
			a.Logger.WithError(err).Errorln("OnStart event for actor failed")
			return err
		}
	}

	// kick off the internal event loop
	go a.eventLoop()

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
	close(a.shutdownCompleteChan)
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
		close(a.shutdownChan)
		a.setShutdownStarted(true)
	}
}

// Wait will block the current goroutine and wait until the actor's shutdown
// process is complete.
func (a *BaseActor) Wait() error {
	if !a.isShutdownCompleted() {
		<-a.shutdownCompleteChan
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
	defer a.mtx.Unlock()
	a.id = id
	// reconfigure the logger
	a.Logger = makeActorLogger(a.ctx, id)
}

// Handle will, by default, just check if we have an implementation actor class
// and calls that implementation's Handle method.
func (a *BaseActor) Handle(m Message) {
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

func makeActorLogger(ctx, id string) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"ctx": ctx,
		"id":  id,
	})
}
