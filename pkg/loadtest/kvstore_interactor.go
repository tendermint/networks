package loadtest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// KVStoreHTTPInteractor implements TestHarnessClientInteractor for
type KVStoreHTTPInteractor struct {
	cfg *Config

	httpClient http.Client // The configured HTTP client we will be using for our requests.
	targetURLs []string    // A cached set of base URLs for each target Tendermint node.

	lastKey   string
	lastValue string
	counter   int

	stats map[string]*SummaryStats

	mtx *sync.RWMutex
}

type abciQueryHTTPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  abciQueryResult `json:"result"`
}

type abciQueryResult struct {
	Response abciQueryResponse `json:"response"`
}

type abciQueryResponse struct {
	Log   string `json:"log"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KVStoreHTTPInteractor implements TestHarnessClientInteractor
var _ TestHarnessClientInteractor = (*KVStoreHTTPInteractor)(nil)

// KVStoreHTTPTestHarnessClientFactory allows us to spawn clients that can test the
// `kvstore` Tendermint proxy app.
func KVStoreHTTPTestHarnessClientFactory(th *TestHarness) *TestHarnessClient {
	return NewTestHarnessClient(th, NewKVStoreHTTPInteractor(th.Cfg))
}

// NewKVStoreHTTPInteractor builds a new interactor for the `kvstore` Tendermint
// proxy application.
func NewKVStoreHTTPInteractor(cfg *Config) *KVStoreHTTPInteractor {
	targetURLs := []string{}
	for _, target := range cfg.TestNetwork.Targets {
		host := target.Host
		rpcPort := cfg.TestNetwork.RPCPort
		// allow for overriding of target RPC port
		if target.RPCPort > 0 {
			rpcPort = target.RPCPort
		}
		targetURLs = append(targetURLs, fmt.Sprintf("http://%s:%d", host, rpcPort))
	}
	return &KVStoreHTTPInteractor{
		cfg: cfg,
		httpClient: http.Client{
			Timeout: time.Duration(cfg.Clients.RequestTimeout),
		},
		targetURLs: targetURLs,
		counter:    0,
		stats: map[string]*SummaryStats{
			"broadcast_tx_sync": NewSummaryStats(time.Duration(cfg.Clients.RequestTimeout)),
			"abci_query":        NewSummaryStats(time.Duration(cfg.Clients.RequestTimeout)),
		},
		mtx: &sync.RWMutex{},
	}
}

// Init does nothing for this particular interactor.
func (i *KVStoreHTTPInteractor) Init() error {
	return nil
}

// Interact is responsible for performing two requests:
// 1. Put a value into the `kvstore`.
// 2. Retrieve that value back out again and make sure it's the same as the
//    value we put in.
func (i *KVStoreHTTPInteractor) Interact() {
	t, err := i.putValue()
	i.addStats("broadcast_tx_sync", t, err)

	t, err = i.getValue()
	i.addStats("abci_query", t, err)
}

// GetStats returns a summary of the requests' stats so far.
func (i *KVStoreHTTPInteractor) GetStats() map[string]*SummaryStats {
	i.mtx.RLock()
	defer i.mtx.RUnlock()
	// make copies of the stats
	broadcastTxSync := *i.stats["broadcast_tx_sync"]
	abciQuery := *i.stats["abci_query"]
	return map[string]*SummaryStats{
		"broadcast_tx_sync": &broadcastTxSync,
		"abci_query":        &abciQuery,
	}
}

// Shutdown does nothing for this particular interactor.
func (i *KVStoreHTTPInteractor) Shutdown() error {
	return nil
}

func (i *KVStoreHTTPInteractor) addStats(reqID string, t int64, err error) {
	i.mtx.Lock()
	defer i.mtx.Unlock()
	i.stats[reqID].AddNano(t, err)
}

func (i *KVStoreHTTPInteractor) putValue() (int64, error) {
	// generate a random key
	i.lastKey, i.lastValue = i.generateRandomKVPair()

	params := url.PathEscape(fmt.Sprintf("tx=\"%s=%s\"", i.lastKey, i.lastValue))
	host := i.randomHost()
	startTime := time.Now().UnixNano()
	res, err := i.httpClient.Get(fmt.Sprintf("%s/broadcast_tx_sync?%s", host, params))
	timeTaken := time.Now().UnixNano() - startTime
	if err != nil {
		return timeTaken, err
	}

	if res.StatusCode >= 400 {
		return timeTaken, NewError(ErrKVStoreClientPutFailed, nil, fmt.Sprintf("got status code %d", res.StatusCode))
	}
	return timeTaken, nil
}

func (i *KVStoreHTTPInteractor) getValue() (int64, error) {
	host := i.randomHost()
	params := url.PathEscape(fmt.Sprintf("data=\"%s\"", i.lastKey))
	startTime := time.Now().UnixNano()
	res, err := i.httpClient.Get(fmt.Sprintf("%s/abci_query?%s", host, params))
	timeTaken := time.Now().UnixNano() - startTime
	if err != nil {
		return timeTaken, err
	}

	if res.StatusCode >= 400 {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, nil, fmt.Sprintf("got status code %d", res.StatusCode))
	}
	// try to parse the body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, err, "failed to read from response body")
	}
	var kr abciQueryHTTPResponse
	if err := json.Unmarshal(body, &kr); err != nil {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, err)
	}

	keyDecoded, err := base64.StdEncoding.DecodeString(kr.Result.Response.Key)
	if err != nil {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, nil, "failed to decode key in response")
	}
	valueDecoded, err := base64.StdEncoding.DecodeString(kr.Result.Response.Value)
	if err != nil {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, nil, "failed to decode value in response")
	}

	key, value := string(keyDecoded), string(valueDecoded)
	if key != i.lastKey {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, nil, "key does not match requested key")
	}
	if value != i.lastValue {
		return timeTaken, NewError(ErrKVStoreClientGetFailed, nil, "value does not match value from previous request")
	}

	return 0, nil
}

func (i *KVStoreHTTPInteractor) randomHost() string {
	return i.targetURLs[int(rand.Int31())%len(i.targetURLs)]
}

func (i *KVStoreHTTPInteractor) generateRandomKVPair() (string, string) {
	keyBytes := make([]byte, 16)
	valueBytes := make([]byte, 16)

	if _, err := rand.Read(keyBytes); err != nil {
		keyBytes = i.generateNonRandomBytes()
	}

	if _, err := rand.Read(valueBytes); err != nil {
		valueBytes = i.generateNonRandomBytes()
	}

	return hex.EncodeToString(keyBytes), hex.EncodeToString(valueBytes)
}

func (i *KVStoreHTTPInteractor) generateNonRandomBytes() []byte {
	s := make([]byte, 8)
	for b := 0; b < 8; b++ {
		s[b] = byte(i.counter + b)
	}
	i.counter++
	if i.counter > 255 {
		i.counter = 0
	}
	return s
}
