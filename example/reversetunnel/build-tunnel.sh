#!/bin/bash
set -euo pipefail

MASTER="http://127.0.0.1:18082"
METHOD="POST"
DIR=$(dirname $0)

curl -X${METHOD} -v -T"${DIR}/build-tunnel.json" \
    "${MASTER}/tunnel"
