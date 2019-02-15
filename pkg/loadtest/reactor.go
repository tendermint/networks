package loadtest

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Reactor-related constants
const (
	DefaultReactorRecvTimeout  = time.Duration(3) * time.Second
	DefaultReactorEventBufSize = 1000
)

// ReactorEvent encapsulates an event that can be sent between reactors.
type ReactorEvent struct {
	Type    string            // What type of event is this?
	Data    interface{}       // Additional data related to the event.
	ResChan chan ReactorEvent // A channel on which we can send responses, if relevant.
	Timeout *time.Duration    // If provided, this will determine certain deadlines within the reactor event loop.
}

// Reactor is a generic interface for managing an event loop.
type Reactor interface {
	OnStart() error        // Called during startup.
	Handle(e ReactorEvent) // Event handler for inside the event loop.
	OnShutdown()           // Handles graceful shutdown of the reactor.
}

//
// BaseReactor
//

// BaseReactor provides the scaffolding for implementing a reactor.
type BaseReactor struct {
	impl   Reactor
	logger *logrus.Entry

	eventChan        chan ReactorEvent
	quitChan         chan struct{}
	quitCompleteChan chan struct{}
	shutdownError    error
}

// BaseReactor implements Reactor.
var _ Reactor = (*BaseReactor)(nil)

// NewBaseReactor instantiates a BaseReactor with the given implementation
// class.
func NewBaseReactor(impl Reactor) *BaseReactor {
	return &BaseReactor{
		impl:             impl,
		logger:           logrus.WithField("ctx", "reactor"),
		eventChan:        make(chan ReactorEvent, DefaultReactorEventBufSize),
		quitChan:         make(chan struct{}),
		quitCompleteChan: make(chan struct{}),
		shutdownError:    nil,
	}
}

// OnStart does nothing for the base reactor.
func (r *BaseReactor) OnStart() error { return nil }

// OnShutdown does nothing for the base reactor.
func (r *BaseReactor) OnShutdown() {}

// Start will kick off the reactor's internal event handling loop.
func (r *BaseReactor) Start() {
	if r.impl != nil {
		if err := r.impl.OnStart(); err != nil {
			r.logger.WithError(err).Errorln("Failed to start reactor")
			return
		}
	}

	go r.eventLoop()
}

// eventLoop is the internal event loop that handles all state management for
// the reactor.
func (r *BaseReactor) eventLoop() {
loop:
	for {
		select {
		case e := <-r.eventChan:
			r.Handle(e)
		case <-r.quitChan:
			break loop
		}
	}
	if r.impl != nil {
		r.impl.OnShutdown()
	}
	close(r.quitCompleteChan)
}

// Handle is the primary event handler for the reactor.
func (r *BaseReactor) Handle(e ReactorEvent) {
	switch e.Type {
	case EvPoisonPill:
		r.Shutdown()
	default:
		if r.impl != nil {
			r.impl.Handle(e)
		}
	}
}

// Recv will facilitate us receiving an event into the event loop.
func (r *BaseReactor) Recv(e ReactorEvent) {
	r.eventChan <- e
}

// RecvAndAwait passes the given event into the event loop and awaits a
// response. It will time out after the given timeout period.
func (r *BaseReactor) RecvAndAwait(e ReactorEvent, timeouts ...time.Duration) ReactorEvent {
	timeout := DefaultReactorRecvTimeout
	if len(timeouts) > 0 {
		timeout = timeouts[0]
	}

	if e.ResChan == nil {
		e.ResChan = make(chan ReactorEvent)
	}
	r.Recv(e)
	select {
	case res := <-e.ResChan:
		return res
	case <-time.After(timeout):
		return ReactorEvent{Type: EvTimeout}
	}
}

// Wait will cause us to wait until the reactor has finished shutting down.
func (r *BaseReactor) Wait() {
	<-r.quitCompleteChan
}

// Shutdown will trigger a shutdown operation.
func (r *BaseReactor) Shutdown() {
	close(r.quitChan)
}

// RunAndWait function starts the given reactor, sets up signal notifications
// for Ctrl+Break and SIGTERM, and then waits for the reactor to either shut
// down of its own accord or be killed by one of the signals.
func (r *BaseReactor) RunAndWait() error {
	// Fire up the event loop
	r.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func(r_ Reactor) {
		<-sigChan
		r.Shutdown()
	}(r)

	// Wait until either the user kills the reactor (Ctrl+Break or SIGTERM) or
	// it finishes of its own accord.
	r.Wait()
	return r.shutdownError
}

//
// ReactorEvent
//

// Respond is a convenience function that, if a response channel is defined for
// the subject event, res will be sent to it, otherwise it will be ignored.
func (e ReactorEvent) Respond(res ReactorEvent) {
	if e.ResChan != nil {
		e.ResChan <- res
	}
}
