package loadtest

import (
	"sort"
	"strings"
)

var testHarnessClientFactoryRegistry = map[string]TestHarnessClientFactory{
	"kvstore_http": KVStoreHTTPTestHarnessClientFactory,
}

// RegisterTestHarnessClientFactory allows developers to register their own test
// harness client factory for testing different kinds of ABCI applications.
func RegisterTestHarnessClientFactory(id string, clientFactory TestHarnessClientFactory) {
	testHarnessClientFactoryRegistry[id] = clientFactory
}

// GetTestHarnessClientFactory will return a pointer to a test harness client
// factory if we recognise it, and if we don't it will return nil.
func GetTestHarnessClientFactory(id string) *TestHarnessClientFactory {
	clientFactory, ok := testHarnessClientFactoryRegistry[id]
	if !ok {
		return nil
	}
	return &clientFactory
}

// GetSupportedTestHarnessClientFactoryTypes simply returns a list of the
// currently registered test harness client factory types.
func GetSupportedTestHarnessClientFactoryTypes() []string {
	result := []string{}
	for id := range testHarnessClientFactoryRegistry {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i], result[j]) < 0
	})
	return result
}
