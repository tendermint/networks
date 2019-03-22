#!/bin/sh
set -e

if [ "${INVENTORY_HOSTNAME}" == "${MASTER_NODE}" ]; then
    tm-load-test -c config.toml -master
else
    tm-load-test -c config.toml -slave
fi
