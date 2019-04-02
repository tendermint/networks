GOPATH?=$(shell go env GOPATH)
INVENTORY?=../../inventory/hosts
FAST_MODE?=false
DEBUG_MODE?=false
FETCH_RESULTS_ONLY?=false
TMLOADTEST_PATH?=$(GOPATH)/src/github.com/tendermint/networks
TMLOADTEST_BINARY?=$(TMLOADTEST_PATH)/build/tm-load-test
MASTER_NODE?=tok0
CLIENTS_TYPE?=tmrpc
CLIENTS_SPAWN?=1000
CLIENTS_SPAWN_RATE?=20.0
CLIENTS_MAX_INTERACTIONS?=10000
CLIENTS_MAX_TEST_TIME?=10m
CLIENTS_REQUEST_WAIT_MIN?=1000ms
CLIENTS_REQUEST_WAIT_MAX?=1000ms
CLIENTS_REQUEST_TIMEOUT?=5s
CLIENTS_INTERACTION_TIMEOUT?=14s
CLIENTS_CONFIG?="{\"clients\":{\
\"type\": \"${CLIENTS_TYPE}\",\
\"spawn\": ${CLIENTS_SPAWN},\
\"spawn_rate\": ${CLIENTS_SPAWN_RATE},\
\"max_interactions\": ${CLIENTS_MAX_INTERACTIONS},\
\"max_test_time\": \"${CLIENTS_MAX_TEST_TIME}\",\
\"request_wait_min\": \"${CLIENTS_REQUEST_WAIT_MIN}\",\
\"request_wait_max\": \"${CLIENTS_REQUEST_WAIT_MAX}\",\
\"request_timeout\": \"${CLIENTS_REQUEST_TIMEOUT}\",\
\"interaction_timeout\": \"${CLIENTS_INTERACTION_TIMEOUT}\",\
}}"
LOCAL_RESULTS_DIR?=/tmp/003-kvstore-loadtest-distributed
.PHONY: execute plot-results

execute:
	@FAST_MODE=$(FAST_MODE) \
		FETCH_RESULTS_ONLY=$(FETCH_RESULTS_ONLY) \
		TMLOADTEST_PATH=$(TMLOADTEST_PATH) \
		./scripts/build-executable.sh
	@mkdir -p $(LOCAL_RESULTS_DIR)
	@rm -rf $(LOCAL_RESULTS_DIR)/*
	ansible-playbook -i $(INVENTORY) \
		-e "{\"local_results_dir\":\"${LOCAL_RESULTS_DIR}\"}" \
		-e "{\"fast_mode\": ${FAST_MODE}}" \
		-e "{\"debug_mode\": ${DEBUG_MODE}}" \
		-e "{\"fetch_results_only\": ${FETCH_RESULTS_ONLY}}" \
		-e "{\"loadtest_master_node\":\"${MASTER_NODE}\"}" \
		-e "{\"binaries\": {\"tmloadtest\": \"${TMLOADTEST_BINARY}\"}}" \
		-e $(CLIENTS_CONFIG) \
		deploy.yml
	
	@$(MAKE) plot-results

plot-results:
	@echo "Plotting load test results in ${LOCAL_RESULTS_DIR}..."
	@go run ./scripts/plot-results.go -in $(LOCAL_RESULTS_DIR)/$(MASTER_NODE) -out $(LOCAL_RESULTS_DIR)/plots
