package pwnat

import (
	"bytes"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/solomonwzs/goxutil/net/network"
)

func clientRun(remoteHost string) {
	var localIP net.IP
	if ip := net.ParseIP(remoteHost); ip == nil {
		panic("not ipv4 host")
	} else if ipv4 := ip.To4(); ipv4 != nil {
		localIP = ipv4
	} else {
		panic("not ipv4 host")
	}

	ipH := &network.IPv4Header{
		Version:    4,
		TOS:        0,
		Length:     network.SIZEOF_IPV4_HEADER + uint16(len(_FAKE_ICMP_BYTES)),
		Id:         0,
		Flags:      network.IPV4_FLAG_DONT_FRAG,
		FragOffset: 0,
		TTL:        64,
		Protocol:   syscall.IPPROTO_ICMP,
		SrcAddr:    localIP,
		DstAddr:    _UNREACHABLE_IP,
	}
	fakePacket, _ := ipH.Marshal()
	fakePacket = append(fakePacket, _FAKE_ICMP_BYTES...)

	conn, err := net.Dial("ip4:icmp", remoteHost)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	icmp := &network.Icmp{
		Type: network.ICMP_CT_TIME_EXCEEDED,
		Code: 0,
	}
	p, _ := icmp.Marshal()

	buf := new(bytes.Buffer)
	buf.Write(p)
	buf.Write([]byte{0, 0, 0, 0})
	buf.Write(fakePacket)
	msg := buf.Bytes()

	for {
		_, err = conn.Write(msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		time.Sleep(1 * time.Second)
	}
}
