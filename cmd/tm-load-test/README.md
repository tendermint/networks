# tm-load-test

A tool for distributed load testing on [Tendermint](https://tendermint.com)
networks.

## Overview
`tm-load-test` is both a tool for distributed load testing, as well as an
example as to how to make use of the [`loadtest`
package](../../pkg/loadtest/README.md) to do your own load testing on a
Tendermint network. `tm-load-test` assumes that the ABCI application under test
is the `kvstore`, and is more interested in the performance of the underlying
Tendermint network than the ABCI application itself.

`tm-load-test` runs in a master/slave configuration, where the master controls
the number and distribution of tasks across multiple Tendermint nodes.

## Quick Start
Create some configuration for your master node (`load-test.toml`):

```toml
[master]
# The host IP/port to which to bind the master
bind = "192.168.1.1:35000"

# How many slaves to expect to connect before starting the load test
expect_slaves = 2

# The folder where tm-load-test will dump results. A sub-folder will be created
# in this folder according to the name of this configuration file: so if this
# file is called "load-test.toml", a folder called "load-test" will be created
# in the results directory.
# NOTE: If a relative path is supplied (starting with a "."), the resulting
# path will be relative to this configuration file.
results_dir = "./results"

[slave]
# IP address and port through which to reach the master (could be different
# from the bind address above, e.g. if the master is bound to 0.0.0.0:35000,
# we still need an external address through which to access the master).
master = "192.168.1.1:35000"

[test-network]
# Do we want to collect Prometheus stats during the load testing too?
enable_prometheus = true

# Defaults for RPC and Prometheus ports on the targets (these can be overridden
# in each target's section below).
rpc_port = 26657
prometheus_port = 26660

# How often to poll the Prometheus endpoint for each host
prometheus_poll_interval = "10s"
# At what point do we consider a Prometheus polling operation a failure?
prometheus_poll_timeout = "1s"

    [[test-network.targets]]
    # A short, descriptive identifier for this host
    id = "tm1"
    host = "192.168.2.1"

    # To override the default RPC/Prometheus ports
    #rpc_port = 26657
    #prometheus_port = 26660

    # If tm-outage-sim-server is running on this host, tm-load-test will
    # attempt to bring it down after 5 minutes, and then back up after 8
    # minutes.
    #outages = "5m:down,8m:up"

    [[test-network.targets]]
    id = "tm2"
    host = "192.168.2.2"

    [[test-network.targets]]
    id = "tm3"
    host = "192.168.2.3"

[clients]
# The number of clients to spawn per slave.
spawn = 500

# The rate at which new clients should be spawned on each slave, per second.
# A value of 10 would mean that every second, 10 new clients will be spawned on
# each slave. A value of 0.1 would mean that only 1 client will be spawned
# every 10 seconds.
spawn_rate = 10

# The maximum number of interactions to send, per client. Since tm-load-test
# assumes the kvstore ABCI application is running, an interaction is defined
# as 2 separate requests: a single PUT operation for a key/value pair, followed
# by a GET operation for that same key, ensuring the stored value matches the
# returned value. Set to -1 to make this "infinite", but then the
# `max_test_time` parameter MUST be set.
max_interactions = 10000

# The maximum amount of time to allow for load testing. If this is exceeded,
# the test will be brought to and end.
max_test_time = "10m"

# The maximum time to wait between each subsequent request in an interaction.
# The actual wait times will be uniformly randomly distributed between
# `request_wait_min` and `request_wait_max`.
request_wait_min = "50ms"

# The minimum time to wait between each subsequent request in an interaction.
request_wait_max = "100ms"

# After how long do we consider a request to have "timed out"?
request_timeout = "5s"
```

To run the master:

```bash
tm-load-test -c load-test.toml -master
```

To run the slaves:

```bash
# Run the first slave from one host
tm-load-test -c load-test.toml -slave

# Run the second slave (from a different host to the previous command)
tm-load-test -c load-test.toml -slave
```

## Statistics
The following statistics will be available through the results created in the
`results_dir`. Where relevant, each file will have a corresponding `*-dist.csv`
file which will show a distribution (i.e. histogram) of the results.

### Tendermint-specific statistics
* Consensus-specific statistics
    * `consensus-rounds.csv` - The number of consensus rounds over time.
    * `consensus-txs-per-block.csv` - The number of transactions per block, over
      time.
    * `consensus-block-interval.csv` - The block interval (in seconds) over
      time.
* Mempool-related statistics
    * `mempool-size.csv` - The size of the mempool in transactions, over time.
    * `mempool-failed-txs.csv` - The number of failed mempool transactions over
      time.
    * `mempool-rechecks.csv` - The number of mempool rechecks over time.
* Summary statistics (min, max, mean, std. deviation for each of the above, in
  `tendermint-summary.csv`).

### Request-related statistics
* Request-related statistics
    * `request-times.csv` - The min/max/average request times over time.
    * `request-rates.csv` - The min/max/average request rates over time.
    * `request-failures.csv` - The number of request failures over time.
* Interaction-related statistics (1 interaction = multiple requests)
    * `interaction-times.csv` - The min/max/average request times over time.
    * `interaction-rates.csv` - The min/max/average interaction rates over time.
    * `interaction-failures.csv` - The number of interaction failures over time.
* Summary statistics for the test (min, max, mean, std. deviation for each of
  the above, in `request-summary.csv`).
