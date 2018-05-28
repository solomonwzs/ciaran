package pwnat

import (
	"flag"
	"fmt"

	"github.com/solomonwzs/goxutil/logger"
	"github.com/solomonwzs/goxutil/net/network"
)

var (
	_FAKE_ICMP_BYTES []byte
)

func init() {
	logger.NewLogger(func(r *logger.Record) {
		fmt.Printf("%s", r)
	})

	icmp := &network.Icmp{
		Type: network.ICMP_CT_ECHO_REQUEST,
		Code: 0,
		Data: &network.IcmpEcho{
			Id:     456,
			SeqNum: 789,
		},
	}
	_FAKE_ICMP_BYTES, _ = icmp.Marshal()
}

func Main() {
	var (
		serverMode bool
		remoteHost string
	)

	flag.BoolVar(&serverMode, "s", false, "server mode")
	flag.StringVar(&remoteHost, "c", "", "remote host")
	flag.Parse()

	if serverMode {
		serverRun()
	} else {
		clientRun(remoteHost)
	}
}
