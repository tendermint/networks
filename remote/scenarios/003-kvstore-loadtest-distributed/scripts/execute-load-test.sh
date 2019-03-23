#!/bin/sh
set -e

VERBOSE=""
if [ "${DEBUG_MODE}" == true ]; then
    VERBOSE="-v"
fi

if [ "${INVENTORY_HOSTNAME}" == "${MASTER_NODE}" ]; then
    tm-load-test -c config.toml -master ${VERBOSE}
else
    tm-load-test -c config.toml -slave ${VERBOSE}
fi
