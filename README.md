# Tendermint Networks

This repository contains tools and scripts to assist in setting up and testing
[Tendermint](https://tendermint.com) networks.

## Tools
The following tools are provided for use during testing of Tendermint networks:

* [tm-load-test](./cmd/tm-load-test/README.md) - A distributed load testing
  application for Tendermint networks.
* [tm-outage-sim-server](./cmd/tm-outage-sim-server/README.md) - A Tendermint
  node outage simulator, for use in CentOS, Debian/Ubuntu environments.

### Requirements
In order to build and use the tools, you will need:

* Go 1.11.5+
* `make`

### Building
To build the tools:

```bash
> make tools
```

This will build the binaries for `tm-load-test` and `tm-outage-sim-server` in a
`build` directory inside this repository.

### Development
To run the linter and the tests:

```bash
> make lint
> make test
```

## Scripts
There are scripts provided in this repository for:

* [load testing](./scripts/load-testing/README.md) of Tendermint networks
