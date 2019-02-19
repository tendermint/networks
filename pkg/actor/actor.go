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
	Shutdown()
	Wait()

	// Lifecycle events
	OnStart() error
	OnShutdown()
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
	shutdownCompleted    bool

	mtx *sync.RWMutex
}

// NewBaseActor instantiates a BaseActor that can be used to provide the basic
// event handling functionality.
func NewBaseActor(impl Actor, ctx string) *BaseActor {
	id := uuid.NewV4().String()
	a := &BaseActor{
		impl:                 impl,
		ctx:                  ctx,
		id:                   id,
		Logger:               makeActorLogger(ctx, id),
		inboxChan:            make(chan Message, DefaultActorInboxSize),
		shutdownChan:         make(chan struct{}),
		shutdownCompleteChan: make(chan struct{}),
		shutdownCompleted:    false,
		mtx:                  &sync.RWMutex{},
	}
	return a
}

func (a *BaseActor) OnStart() error { return nil }
func (a *BaseActor) OnShutdown()    {}

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

// Shutdown initiates the shutdown process (asynchronously) for this actor. If
// you want to know once the shutdown process is complete, use the Wait()
// function.
func (a *BaseActor) Shutdown() {
	if a.impl != nil {
		a.impl.OnShutdown()
	}
	close(a.shutdownChan)
}

// Wait will block the current goroutine and wait until the actor's shutdown
// process is complete.
func (a *BaseActor) Wait() {
	if !a.shutdownCompleted {
		<-a.shutdownCompleteChan
		a.shutdownCompleted = true
	}
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
	a.Logger.WithField("m", m).Debugln("Got incoming message")

	switch m.Type {
	case PoisonPill:
		a.Logger.Debugln("Got poison pill, shutting down")
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
	a.Logger.WithField("msg", msg).Debugln("Recv")
	a.inboxChan <- msg
}

// Send is a convenience function to send a message to the given actor with this
// actor as the sender.
func (a *BaseActor) Send(other Actor, msg Message) {
	msg.Sender = a
	if other != nil {
		a.Logger.WithFields(logrus.Fields{
			"msg":   msg,
			"other": other.GetID(),
		}).Debugln("Sending message to actor")
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
