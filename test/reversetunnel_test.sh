#!/bin/bash
# set -euo pipefail

echo -e "\033[01;32m[Test]\033[0m Reversetunnel Test ... "

MASTER_ADDR="http://127.0.0.1:18082"
METHOD="POST"
DIR=$(dirname $0)

BIN="${DIR}/../bin/reversetunnel.goc"
MASTER_CONF="${DIR}/../conf/reversetunnel-master-example.json"
SLAVER_CONF="${DIR}/../conf/reversetunnel-slaver-example.json"

nc -klp 3801 &
npid=$!

"$BIN" -f "$MASTER_CONF" &
mpid=$!
sleep 1

"$BIN" -f "$SLAVER_CONF" &
spid=$!
sleep 1

function end() {
    kill $npid
    kill $spid
    kill $mpid
}

trap "end; exit" SIGINT SIGTERM

function build_tunnel_req() {
    curl -X${METHOD} \
        -T"${DIR}/../example/reversetunnel/build-tunnel.json" \
        "${MASTER_ADDR}/tunnel"
}

build_tunnel_req
echo ">> hello" | nc 127.0.0.1 3800 -q 0
echo ">> world" | nc 127.0.0.1 3800 -q 0

echo -ne "\033[01;32m[Test]\033[0m Press any keys to exit ... "
read -r -n 1 -s var

end
