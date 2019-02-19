package loadtest

// SlaveState helps in managing the state machine associated with the slave.
type SlaveState string

// The various states in which a slave can be.
const (
	SlaveStarting    SlaveState = "starting"
	SlaveConnecting  SlaveState = "connecting"
	SlaveWaiting     SlaveState = "waiting"
	SlaveLoadTesting SlaveState = "load-testing"
	SlaveFailing     SlaveState = "failing"
	SlaveCompleting  SlaveState = "completing"
)
