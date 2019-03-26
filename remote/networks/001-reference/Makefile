INVENTORY?=../../inventory/hosts
LOGS_DIR?=/tmp
FAST_MODE?=false
GOPATH?=$(shell go env GOPATH)
WITH_CLEVELDB?=false
INSTALL_TENDERMINT_LOCALLY?=false
TENDERMINT_SRC=$(GOPATH)/src/github.com/tendermint/tendermint
TM_NETWORKS_SRC=$(GOPATH)/src/github.com/tendermint/networks
TM_OUTAGE_SIM_SERVER_BIN=$(TM_NETWORKS_SRC)/build/tm-outage-sim-server
NETWORK_CONFIG_SCRIPT?=./scripts/001-default.sh
CONFIG_SCRIPT?=$(NETWORK_CONFIG_SCRIPT)
NETWORK_VALIDATORS?=4
VALIDATORS?=$(NETWORK_VALIDATORS)
UNAME_S?=$(shell uname -s)
.PHONY: deploy fetch-tm-logs status

deploy:
	@if [ "${FAST_MODE}" == "false" ]; then \
		if [ "${WITH_CLEVELDB}" == "true" ]; then \
			if [ "${UNAME_S}" != "Linux" ]; then \
				echo "ERROR: The cleveldb support can only be built from a Linux machine (yours is \"${UNAME_S}\")"; \
				exit 1; \
			fi; \
			echo "Building Tendermint with cleveldb support..."; \
			if [ "${INSTALL_TENDERMINT_LOCALLY}" == "true" ]; then \
				CGO_LDFLAGS="-lsnappy" make -C $(TENDERMINT_SRC) install_c; \
			fi; \
			CGO_LDFLAGS="-lsnappy" make -C $(TENDERMINT_SRC) build_c; \
		else \
			echo "Building Tendermint..."; \
			if [ "${INSTALL_TENDERMINT_LOCALLY}" == "true" ]; then \
				make -C $(TENDERMINT_SRC) install; \
			fi; \
			make -C $(TENDERMINT_SRC) build-linux; \
		fi; \
		echo "Building load testing tools..."; \
		make -C $(TM_NETWORKS_SRC) tools-linux; \
	fi
	@rm -rf /tmp/nodes/
	@echo "Generating Tendermint configuration for ${VALIDATORS} validators..."
	@tendermint testnet \
		--v $(VALIDATORS) \
		--n 0 \
		--o /tmp/nodes/ \
		--hostname-prefix tik \
		--node-dir-prefix tik
	@echo "Updating generated Tendermint node configuration..."
	@$(CONFIG_SCRIPT)
	ansible-playbook -i $(INVENTORY) \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		-e "{\"with_cleveldb\":${WITH_CLEVELDB}}" \
		-e "{\"tm_outage_sim_server_bin\":\"${TM_OUTAGE_SIM_SERVER_BIN}\"}" \
		deploy.yml
	@echo "Checking if Tendermint nodes are up..."
	@./scripts/check-status.sh

fetch-tm-logs:
	ansible-playbook -i $(INVENTORY) fetch-tm-logs.yml

status:
	@SLEEP=0 ./scripts/check-status.sh
