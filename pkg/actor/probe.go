package actor

import (
	"fmt"
	"sync"
	"time"
)

// Probe is a specialised actor that allows us to intercept messages that are
// sent to a particular actor and report back on those messages.
type Probe struct {
	*BaseActor

	capturedMessages   []Message
	notifyWhenCaptured int
	notifyc            chan int
	mtx                *sync.Mutex
}

// NewProbe instantiates a probe actor.
func NewProbe() *Probe {
	p := &Probe{
		capturedMessages:   make([]Message, 0),
		notifyWhenCaptured: -1,
		notifyc:            make(chan int, 1),
		mtx:                &sync.Mutex{},
	}
	p.BaseActor = NewBaseActor(p, "probe")
	return p
}

// Handle buffers any messages to which we've subscribed.
func (p *Probe) Handle(msg Message) {
	switch msg.Type {
	case SubscriptionMessage:
		orig, ok := msg.Data.(Message)
		if !ok {
			p.Logger.Error("Failed to decode subscription message body as actor.Message")
		} else {
			p.captureMessage(orig)
		}
	}
}

// CapturedMessages will return a copy of the buffered messages to which we've
// subscribed.
func (p *Probe) CapturedMessages() []Message {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return append([]Message(nil), p.capturedMessages...)
}

// ClearCapturedMessages will empty out the internal captured messages buffer.
func (p *Probe) ClearCapturedMessages() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.capturedMessages = nil
}

// WaitForCapturedMessages will wait until the probe has either received the
// given number of messages, or the timeout has expired.
func (p *Probe) WaitForCapturedMessages(count int, timeout time.Duration) error {
	p.Logger.Debug("Waiting for captured messages", "count", count, "timeout", timeout)
	// no need to wait if we've already got the requisite number of messages
	if len(p.CapturedMessages()) == count {
		return nil
	}

	p.mtx.Lock()
	p.notifyWhenCaptured = count
	p.mtx.Unlock()

	select {
	case c := <-p.notifyc:
		if c != count {
			return fmt.Errorf("Expected %d captured messages, but was notified when we received %d", count, c)
		}
	case <-time.After(timeout):
		return fmt.Errorf("Timed out waiting for %d captured messages", count)
	}
	return nil
}

func (p *Probe) captureMessage(msg Message) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.capturedMessages = append(p.capturedMessages, msg)
	if p.notifyWhenCaptured > 0 && len(p.capturedMessages) == p.notifyWhenCaptured {
		p.notifyc <- len(p.capturedMessages)
		// reset the notification counter
		p.notifyWhenCaptured = -1
	}
}
