package reversetunnel

import (
	"bytes"
	"net"
	"time"
)

type slaverAgent struct {
	tunnelAddr  *address
	ctrl        net.Conn
	ch          chan interface{}
	pTunnels    map[string]*mProxyTunnel
	standByConn map[tunnelID]*mProxyTunnelConn
}

func newSlaver(conn net.Conn, tunnelAddr *address) *slaverAgent {
	return &slaverAgent{
		tunnelAddr:  tunnelAddr,
		ctrl:        conn,
		ch:          make(chan interface{}, 10),
		pTunnels:    map[string]*mProxyTunnel{},
		standByConn: map[tunnelID]*mProxyTunnelConn{},
	}
}

func (s *slaverAgent) recvHeartbeat() {
	for {
		s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))

		if cmd, err := parseCommandV1(s.ctrl); err != nil {
			s.ch <- err
			break
		} else if cmd == CMD_V1_HEARTBEAT {
			continue
		} else {
			s.ch <- ErrCommand
			break
		}
	}
}

func (s *slaverAgent) serve() {
	go s.recvHeartbeat()
	for e := range s.ch {
		switch e.(type) {
		case error:
			return
		case *buildTunnelReq:
			req := e.(*buildTunnelReq)
			s.newProxyTunnel(req)
		case tunnelID:
			buf := new(bytes.Buffer)
			buf.Write([]byte{PROTO_VER, CMD_V1_BUILD_TUNNEL,
				s.tunnelAddr.atype})
			buf.Write(s.tunnelAddr.ip)
			buf.Write(s.tunnelAddr.port[:])
		}
	}
}

func (s *slaverAgent) newProxyTunnel(req *buildTunnelReq) {
	sAddr := parseAddr(req.SAddr)
	if sAddr == nil {
		return
	}
	pTunnel, err := newMProxyTunnel(s.tunnelAddr, sAddr, req.MAddr)
	if err != nil {
		return
	}
	s.pTunnels[req.MAddr] = pTunnel
}

func (s *slaverAgent) close() {
	s.ctrl.Close()

	for _, pTunnel := range s.pTunnels {
		pTunnel.close()
	}

	end := time.After(_NETWORK_TIMEOUT)
	for {
		select {
		case <-s.ch:
			break
		case <-end:
			return
		}
	}
}
