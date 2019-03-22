INVENTORY?=../../inventory/hosts
LOGS_DIR?=/tmp
FAST_MODE?=false
GOPATH?=$(shell go env GOPATH)
TENDERMINT_SRC=$(GOPATH)/src/github.com/tendermint/tendermint
TM_NETWORKS_SRC=$(GOPATH)/src/github.com/tendermint/networks
TM_OUTAGE_SIM_SERVER_BIN=$(TM_NETWORKS_SRC)/build/tm-outage-sim-server
.PHONY: deploy fetch-tm-logs status

deploy:
	if [ "${FAST_MODE}" == "false" ]; then \
		make -C $(TENDERMINT_SRC) install; \
		make -C $(TENDERMINT_SRC) build-linux; \
		make -C $(TM_NETWORKS_SRC) tools-linux; \
	fi
	rm -rf /tmp/nodes/
	tendermint testnet \
		--v 4 \
		--n 0 \
		--o /tmp/nodes/ \
		--hostname-prefix tik \
		--node-dir-prefix tik
	./scripts/update-node-config.sh
	ansible-playbook -i $(INVENTORY) \
		-e "{\"fast_mode\":${FAST_MODE}}" \
		-e "{\"tm_outage_sim_server_bin\":\"${TM_OUTAGE_SIM_SERVER_BIN}\"}" \
		deploy.yml
	./scripts/check-status.sh

fetch-tm-logs:
	ansible-playbook -i $(INVENTORY) fetch-tm-logs.yml

status:
	@SLEEP=0 ./scripts/check-status.sh
