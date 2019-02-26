GOPATH?=$(shell go env GOPATH)
GOBIN?=$(GOPATH)/bin
SRC_DIR?=$(GOPATH)/src/github.com/tendermint/networks
BUILD_DIR?=$(SRC_DIR)/build
.PHONY: build-tm-outage-sim-server build-tm-outage-sim-server-linux \
	build build-linux \
	clean test lint \
	get-deps

$(GOBIN)/dep:
	go get -u github.com/golang/dep/cmd/dep

$(GOBIN)/golangci-lint:
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

get-deps: $(GOBIN)/dep
	dep ensure

get-linter: $(GOBIN)/golangci-lint

build-tm-outage-sim-server: get-deps
	go build -o $(BUILD_DIR)/tm-outage-sim-server \
		$(SRC_DIR)/cmd/tm-outage-sim-server/main.go

build-tm-outage-sim-server-linux: get-deps
	GOOS=linux GOARCH=amd64 \
		go build -o $(BUILD_DIR)/tm-outage-sim-server \
		$(SRC_DIR)/cmd/tm-outage-sim-server/main.go

build-tm-load-test: get-deps
	go build -o $(BUILD_DIR)/tm-load-test \
		$(SRC_DIR)/cmd/tm-load-test/main.go

build-tm-load-test-linux: get-deps
	GOOS=linux GOARCH=amd64 \
		go build -o $(BUILD_DIR)/tm-load-test \
		$(SRC_DIR)/cmd/tm-load-test/main.go

build: build-tm-outage-sim-server build-tm-load-test

build-linux: build-tm-outage-sim-server-linux build-tm-load-test-linux

lint: get-deps get-linter
	golangci-lint run ./...

test: get-deps
	go list ./... | grep -v /vendor/ | xargs go test -cover -race

clean:
	rm -rf $(BUILD_DIR)
