#!/bin/bash

go test ./cmd/gaia/app -run TestFullGaiaSimulation \
                       -SimulationEnabled=true \
                       -SimulationNumBlocks=${BLOCKS} \
                       -SimulationVerbose=true \
                       -SimulationCommit=true \
                       -SimulationSeed=${SEED} \
                       -SimulationPeriod=${PERIOD} \
                       -v -timeout 24h

if [[ $? -ne 0 ]] && [[ -z "${SLACK_URL}" ]]; then
    go run notify_slack.go "${SLACK_URL}" 1
else
    go run notify_slack.go "${SLACK_URL}" 0
fi
