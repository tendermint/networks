package client

import (
	"math/rand"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/tendermint/networks/pkg/loadtest"
	"github.com/tendermint/networks/pkg/loadtest/messages"
	rpclib "github.com/tendermint/tendermint/rpc/lib/client"
)

// KVStoreClientFactory produces KVStoreClient instances.
type KVStoreClientFactory struct{}

// KVStoreClient is a load testing client that interacts with a Tendermint
// network via WebSockets interfaces.
type KVStoreClient struct {
	cfg       *KVStoreClientConfig
	wsClients []*rpclib.WSClient

	stats *messages.CombinedStats
}

// KVStoreClientConfig is to be parsed out of the `clients.additional_config`
// field.
type KVStoreClientConfig struct {
	MaxConns int    `toml:"max_conns,omitempty"` // The maximum number of node connections. If not specified, we'll use the number of target nodes minus 1.
	Endpoint string `toml:"endpoint,omitempty"`  // The WebSockets endpoint path (usually "/websocket").
}

var defaultKVStoreClientConfig = KVStoreClientConfig{
	MaxConns: 1,
	Endpoint: "/websocket",
}

// KVStoreClientFactory implements loadtest.ClientFactory
var _ loadtest.ClientFactory = (*KVStoreClientFactory)(nil)

// KVStoreClient implements loadtest.KVStoreClient
var _ loadtest.Client = (*KVStoreClient)(nil)

func init() {
	loadtest.RegisterClientFactory("kvstore", &KVStoreClientFactory{})
}

// ParseKVStoreConfig will parse the given string assuming it's (1)
// configuration for a `KVStoreClient`, and (2) that its contents are formatted
// as the contents of an inline table in TOML format (but without the outermost
// curly braces). On error, this just returns the default configuration.
func ParseKVStoreConfig(s string) *KVStoreClientConfig {
	cfg := &KVStoreClientConfig{}
	// if we can't parse it, just ignore the error and return a default
	// configuration
	if _, err := toml.Decode("{"+s+"}", &cfg); err != nil {
		return &defaultKVStoreClientConfig
	}
	if cfg.MaxConns < 1 {
		cfg.MaxConns = defaultKVStoreClientConfig.MaxConns
	}
	if len(cfg.Endpoint) == 0 {
		cfg.Endpoint = defaultKVStoreClientConfig.Endpoint
	}
	return cfg
}

//
// KVStoreClientFactory
//

// NewStats creates a fresh new `CombinedStats` object for use in stats
// tracking for this client type.
func (cf *KVStoreClientFactory) NewStats(params loadtest.ClientParams) *messages.CombinedStats {
	return nil
}

// NewClient instantiates a `KVStoreClient` that is connected to several
// Tendermint network WebSockets endpoints.
func (cf *KVStoreClientFactory) NewClient(params loadtest.ClientParams) loadtest.Client {
	client := &KVStoreClient{
		cfg:       ParseKVStoreConfig(params.AdditionalParams),
		wsClients: make([]*rpclib.WSClient, 0),
		stats:     cf.NewStats(params),
	}
	// shuffle the target nodes in the parameters
	rand.Shuffle(len(params.TargetNodes), func(i, j int) {
		params.TargetNodes[i], params.TargetNodes[j] = params.TargetNodes[j], params.TargetNodes[i]
	})
	// connect to the desired number of WebSocket endpoints
	for i := 0; i < client.cfg.MaxConns && i < len(params.TargetNodes); i++ {
		wsClient := rpclib.NewWSClient(
			params.TargetNodes[i],
			client.cfg.Endpoint,
			rpclib.PingPeriod(1*time.Second),
		)
		// this will cause the WebSockets client to dial the Tendermint RPC
		// endpoint and start the WebSockets read/write loop
		if err := wsClient.Start(); err != nil {
			panic(err)
		}
		client.wsClients = append(client.wsClients, wsClient)
	}
	return client
}

//
// KVStoreClient
//

// Interact will attempt to put a value into the kvstore by way of one of its
// WebSocket connections, and wait for it to be committed. After that it will
// validate that the committed value was, indeed, the same as the one we put in.
func (c *KVStoreClient) Interact() {

}

// GetStats will return the current stats object for this client.
func (c *KVStoreClient) GetStats() *messages.CombinedStats {
	return c.stats
}
