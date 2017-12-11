package reversetunnel

import (
	"logger"
	"net"
	"time"
)

type slaverAgent struct {
	tunnelAddr  *address
	ctrl        net.Conn
	ch          chan *channelEvent
	pTunnels    map[string]*mProxyTunnel
	standByConn map[uint64]*mProxyTunnelConn
}

const ()

func newSlaver(conn net.Conn, tunnelAddr *address) *slaverAgent {
	return &slaverAgent{
		tunnelAddr:  tunnelAddr,
		ctrl:        conn,
		ch:          make(chan *channelEvent, 10),
		pTunnels:    map[string]*mProxyTunnel{},
		standByConn: map[uint64]*mProxyTunnelConn{},
	}
}

func (s *slaverAgent) recvHeartbeat() {
	for {
		s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))

		if cmd, err := parseCommandV1(s.ctrl); err != nil {
			s.ch <- &channelEvent{_EVENT_SA_ERROR, err}
			break
		} else if cmd == CMD_V1_HEARTBEAT {
			continue
		} else {
			s.ch <- &channelEvent{_EVENT_SA_ERROR, ErrCommand}
			break
		}
	}
}

func (s *slaverAgent) serve() {
	go s.recvHeartbeat()
	for e := range s.ch {
		switch e.typ {
		case _EVENT_SA_ERROR:
			return
		case _EVENT_SA_BUILD_TUNNEL_REQ:
			req := e.data.(*buildTunnelReq)
			s.newProxyTunnel(req)
		case _EVENT_SA_SEND_DATA:
			s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if _, err := s.ctrl.Write(e.data.([]byte)); err != nil {
				logger.Error(err)
			}
		}
	}
}

func (s *slaverAgent) newProxyTunnel(req *buildTunnelReq) {
	sAddr := parseAddr(req.SAddr)
	if sAddr == nil {
		return
	}
	pTunnel, err := newMProxyTunnel(s.tunnelAddr, sAddr, req.MAddr, s.ch)
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
