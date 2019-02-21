package loadtest

import (
	"encoding"
	"fmt"
	"io/ioutil"
	"math/rand"
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
}

// SlaveConfig provides configuration specific to the load testing slaves.
type SlaveConfig struct {
	Master string // The master's external address (host:port).
}

// TestNetworkConfig encapsulates information about the network under test.
type TestNetworkConfig struct {
	RPCPort int // The default Tendermint RPC port.

	EnablePrometheus       bool              // Should we enable collections of Prometheus stats during testing?
	PrometheusPort         int               // The default Prometheus port.
	PrometheusPollInterval ParseableDuration // How often should we poll the Prometheus endpoint?
	PrometheusPollTimeout  ParseableDuration // At what point do we consider a Prometheus polling operation a failure?

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
	Type               string            // The type of client to spawn.
	Spawn              int               // The number of clients to spawn, per slave.
	SpawnRate          float32           // The rate at which to spawn clients, per second, on each slave.
	MaxInteractions    int               // The maximum number of interactions emanating from each client.
	MaxTestTime        ParseableDuration // The maximum duration of the test, beyond which this client must be stopped.
	RequestWaitMin     ParseableDuration // The minimum wait period before each request before sending another one.
	RequestWaitMax     ParseableDuration // The maximum wait period before each request before sending another one.
	RequestTimeout     ParseableDuration // The maximum time allowed before considering a request to have timed out.
	InteractionTimeout ParseableDuration // The maximum time allowed for an overall interaction.
}

// ParseableDuration represents a time.Duration that implements
// encoding.TextUnmarshaler.
type ParseableDuration time.Duration

// ParseableDuration implements encoding.TextUnmarshaler
var _ encoding.TextUnmarshaler = (*ParseableDuration)(nil)

// ParseConfig will parse the configuration from the given string.
func ParseConfig(data string) (*Config, error) {
	var cfg Config
	if _, err := toml.Decode(data, &cfg); err != nil {
		return nil, NewError(ErrFailedToDecodeConfig, err)
	}
	// validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
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
func (d *ParseableDuration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err == nil {
		*d = ParseableDuration(dur)
	}
	return err
}

//
// Config
//

// Validate does a deep check on the configuration to make sure it makes sense.
func (c *Config) Validate() error {
	if err := c.Master.Validate(); err != nil {
		return err
	}
	if err := c.Slave.Validate(); err != nil {
		return err
	}
	if err := c.TestNetwork.Validate(); err != nil {
		return err
	}
	if err := c.Clients.Validate(); err != nil {
		return err
	}
	return nil
}

//
// MasterConfig
//

func (m *MasterConfig) Validate() error {
	if len(m.Bind) == 0 {
		return NewError(ErrInvalidConfig, nil, "master bind address must be specified")
	}
	if m.ExpectSlaves < 1 {
		return NewError(ErrInvalidConfig, nil, "master must expect at least one slave")
	}
	if len(m.ResultsDir) == 0 {
		return NewError(ErrInvalidConfig, nil, "master results output directory must be specified")
	}
	return nil
}

//
// SlaveConfig
//

func (s *SlaveConfig) Validate() error {
	if len(s.Master) == 0 {
		return NewError(ErrInvalidConfig, nil, "slave address for master must be explicitly specified")
	}
	return nil
}

//
// TestNetworkConfig
//

func (c *TestNetworkConfig) Validate() error {
	if c.PrometheusPort < 1 {
		return NewError(ErrInvalidConfig, nil, "test network prometheus port is invalid")
	}
	if c.RPCPort < 1 {
		return NewError(ErrInvalidConfig, nil, "test network RPC port is invalid")
	}
	if len(c.Targets) == 0 {
		return NewError(ErrInvalidConfig, nil, "test network must have at least one target")
	}
	for i, target := range c.Targets {
		if err := target.Validate(i); err != nil {
			return err
		}
	}
	return nil
}

// RandomTarget allows us to pick a Tendermint node at random from the test
// network configuration.
func (c *TestNetworkConfig) RandomTarget() *TestNetworkTargetConfig {
	return &c.Targets[int(rand.Int31())%len(c.Targets)]
}

//
// TestNetworkTargetConfig
//

func (c *TestNetworkTargetConfig) Validate(i int) error {
	if len(c.Host) == 0 {
		return NewError(ErrInvalidConfig, nil, fmt.Sprintf("test network target %d is missing a host address", i))
	}
	return nil
}

//
// ClientConfig
//

func (c *ClientConfig) Validate() error {
	if clientFactory := GetTestHarnessClientFactory(c.Type); clientFactory != nil {
		return NewError(
			ErrInvalidConfig,
			nil,
			fmt.Sprintf("client type is unrecognized (supported: %s)", GetSupportedTestHarnessClientFactoryTypes()),
		)
	}
	if c.Spawn < 1 {
		return NewError(ErrInvalidConfig, nil, "client spawn count must be greater than 0")
	}
	if c.SpawnRate <= 0 {
		return NewError(ErrInvalidConfig, nil, "client spawn rate must be a positive floating point number")
	}
	if c.MaxInteractions == -1 {
		if c.MaxTestTime == 0 {
			return NewError(ErrInvalidConfig, nil, "if client max interactions is -1, max test time must be set")
		}
	} else if c.MaxInteractions < 1 {
		return NewError(ErrInvalidConfig, nil, "client maximum interactions must be -1, or greater than 1")
	}
	if c.RequestTimeout <= 0 {
		return NewError(ErrInvalidConfig, nil, "client request timeout cannot be 0")
	}
	return nil
}
