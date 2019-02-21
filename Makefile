GOPATH?=$(shell go env GOPATH)
SRC_DIR?=$(GOPATH)/src/github.com/tendermint/networks
BUILD_DIR?=$(SRC_DIR)/build
.PHONY: build-tm-outage-sim-server build-tm-outage-sim-server-linux \
	build build-linux \
	clean \
	test

build-tm-outage-sim-server:
	go build -o $(BUILD_DIR)/tm-outage-sim-server \
		$(SRC_DIR)/cmd/tm-outage-sim-server/main.go

build-tm-outage-sim-server-linux: build-dir
	GOOS=linux GOARCH=amd64 \
		go build -o $(BUILD_DIR)/tm-outage-sim-server \
		$(SRC_DIR)/cmd/tm-outage-sim-server/main.go

build: build-tm-outage-sim-server

build-linux: build-tm-outage-sim-server-linux

test:
	go list ./... | grep -v /vendor/ | xargs go test -cover -race

clean:
	rm -rf $(BUILD_DIR)
