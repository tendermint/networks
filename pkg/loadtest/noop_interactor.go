package loadtest

import "time"

// NoopInteractor is a test harness client interactor that does nothing when
// executed. This is useful for testing.
type NoopInteractor struct{}

// NoopInteractor implements TestHarnessClientInteractor
var _ TestHarnessClientInteractor = (*NoopInteractor)(nil)

// Init does nothing.
func (i *NoopInteractor) Init() error { return nil }

// Interact does nothing.
func (i *NoopInteractor) Interact() {}

// GetStats returns an empty stats map.
func (i *NoopInteractor) GetStats() map[string]*SummaryStats {
	return map[string]*SummaryStats{
		"noop": NewSummaryStats(1 * time.Second),
	}
}

// Shutdown does nothing.
func (i *NoopInteractor) Shutdown() error { return nil }
