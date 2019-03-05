#!/bin/bash -x

go test ./cosmos-sdk/cmd/gaia/app -run TestFullGaiaSimulation \
                                  -SimulationEnabled=true \
                                  -SimulationNumBlocks=$BLOCKS \
                                  -SimulationVerbose=true \
                                  -SimulationCommit=true \
                                  -SimulationSeed=$SEED \
                                  -SimulationPeriod=$PERIOD \
                                  -v -timeout 24h
