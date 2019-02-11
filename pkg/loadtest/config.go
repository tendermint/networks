package loadtest

import (
	"encoding"
	"io/ioutil"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the central configuration structure for our load testing, from both
// the master and slaves' perspectives.
type Config struct {
	Master      MasterConfig      // The master's load testing configuration.
	Slave       SlaveConfig       // The slaves' load testing configuration.
	TestNetwork TestNetworkConfig // The test network layout/configuration.
	Clients     ClientConfig      // Load testing client-related configuration.
}

// MasterConfig provides the configuration for the load testing master.
type MasterConfig struct {
	Bind         string // The address to which to bind the master (host:port).
	ExpectSlaves int    // The number of slaves to expect to connect before starting the load test.
	ResultsDir   string // The root of the results output directory.

	configName string // This is extracted from the filename of the configuration file.
}

// SlaveConfig provides configuration specific to the load testing slaves.
type SlaveConfig struct {
	Master string // The master's external address (host:port).
}

// TestNetworkConfig encapsulates information about the network under test.
type TestNetworkConfig struct {
	RPCPort int // The default Tendermint RPC port.

	EnablePrometheus       bool     // Should we enable collections of Prometheus stats during testing?
	PrometheusPort         int      // The default Prometheus port.
	PrometheusPollInterval duration // How often should we poll the Prometheus endpoint?
	PrometheusPollTimeout  duration // At what point do we consider a Prometheus polling operation a failure?

	Targets []TestNetworkTargetConfig // Configuration for each of the Tendermint nodes in the network.
}

// TestNetworkTargetConfig encapsulates the configuration for each node in the
// Tendermint test network.
type TestNetworkTargetConfig struct {
	ID   string // A short, descriptive identifier for this node.
	Host string // The host address for this node.

	RPCPort        int    `toml:"rpc_port,omitempty"`        // Override for the default Tendermint RPC port for this node.
	PrometheusPort int    `toml:"prometheus_port,omitempty"` // Override for the default Prometheus port for this node.
	Outages        string `toml:"outages,omitempty"`         // Specify an outage schedule to try to affect for this host.
}

// ClientConfig contains the configuration for clients being spawned on slaves.
type ClientConfig struct {
	Spawn           int      // The number of clients to spawn, per slave.
	SpawnRate       float32  // The rate at which to spawn clients, per second, on each slave.
	MaxInteractions int      // The maximum number of interactions emanating from each client.
	MaxTestTime     duration // The maximum duration of the test, beyond which this client must be stopped.
	RequestWaitMin  duration // The minimum wait period after each request before sending another one.
	RequestWaitMax  duration // The maximum wait period after each request before sending another one.
	RequestTimeout  duration // The maximum time allowed before considering a request to have timed out.
}

type duration time.Duration

// duration implements encoding.TextUnmarshaler
var _ encoding.TextUnmarshaler = (*duration)(nil)

// ParseConfig will parse the configuration from the given string.
func ParseConfig(data string) (*Config, error) {
	var cfg Config
	if _, err := toml.Decode(data, &cfg); err != nil {
		return nil, NewError(ErrFailedToDecodeConfig, err)
	}
	return &cfg, nil
}

// LoadConfig will attempt to load configuration from the given file.
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, NewError(ErrFailedToReadConfigFile, err)
	}
	return ParseConfig(string(data))
}

// UnmarshalText allows us a convenient way to unmarshal durations.
func (d *duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err == nil {
		*d = duration(dur)
	}
	return err
}
