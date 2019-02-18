package actor

import (
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// Constants relating to actors.
const (
	DefaultActorInboxSize = 1000
)

// Actor is an implementation of the actor pattern.
type Actor interface {
	// Retrieves the unique ID of this actor.
	GetID() string

	// Primary handler method for dealing with incoming messages to the actor's
	// inbox.
	Handle(m Message)

	// Recv allows another actor to send this actor a message by putting it into
	// its inbox.
	Recv(m Message)

	// Lifecycle events
	OnStart() error
	OnShutdown()
}

var _ Actor = (*BaseActor)(nil)

// BaseActor is a convenience class from which we can derive other actors. This
// is effectively an "abstract class" and should be treated as such.
type BaseActor struct {
	impl   Actor // The implementation/derived class for this actor.
	id     string
	logger *logrus.Entry

	inboxChan            chan Message
	shutdownChan         chan struct{}
	shutdownCompleteChan chan struct{}
}

// NewBaseActor instantiates a BaseActor that can be used to provide the basic
// event handling functionality.
func NewBaseActor(impl Actor) *BaseActor {
	id := uuid.NewV4().String()
	return &BaseActor{
		impl: impl,
		id:   id,
		logger: logrus.WithFields(logrus.Fields{
			"ctx": "BaseActor",
			"id":  id,
		}),
		inboxChan:            make(chan Message, DefaultActorInboxSize),
		shutdownChan:         make(chan struct{}),
		shutdownCompleteChan: make(chan struct{}),
	}
}

func (a *BaseActor) OnStart() error { return nil }
func (a *BaseActor) OnShutdown()    {}

// Start will initiate this actor's event loop in a separate goroutine. If the
// impl.OnStart method fails, the error will be returned and the event loop will
// not be started.
func (a *BaseActor) Start() error {
	if a.impl != nil {
		if err := a.impl.OnStart(); err != nil {
			a.logger.WithError(err).Errorln("OnStart event for actor failed")
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
	<-a.shutdownCompleteChan
}

// GetID will retrieve the actor's ID, which should be universally unique.
func (a *BaseActor) GetID() string {
	return a.id
}

// Handle will, by default, just check if we have an implementation actor class
// and calls that implementation's Handle method.
func (a *BaseActor) Handle(m Message) {
	a.logger.WithField("m", m).Debugln("Got incoming message")
	if a.impl != nil {
		a.impl.Handle(m)
	}
}

// Recv allows BaseActor to queue an incoming message to be handled by its event
// loop.
func (a *BaseActor) Recv(m Message) {
	a.logger.WithField("m", m).Debugln("Recv")
	a.inboxChan <- m
}
