package pwnat

import (
	"net"
	"time"
)

var (
	_UNREACHABLE_IP = net.IPv4(3, 3, 3, 3)
)

func serverRun() {
	conn, err := net.Dial("ip4:icmp", _UNREACHABLE_IP.String())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		_, err = conn.Write(_PSEUDO_ICMP_BYTES)
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
}
