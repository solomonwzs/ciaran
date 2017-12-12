package reversetunnel

import (
	"logger"
	"net"
	"time"
)

type slaverAgent struct {
	name string

	tunnelAddr *address
	ctrl       net.Conn

	pTunnels       map[string]*mProxyTunnel
	waitingTunnels map[uint64]*mProxyTunnel

	ch         chan *channelEvent
	masterChan chan *channelEvent
}

func newSlaverAgent(name string, conn net.Conn, tunnelAddr *address,
	ch chan *channelEvent) *slaverAgent {

	return &slaverAgent{
		name: name,

		tunnelAddr: tunnelAddr,
		ctrl:       conn,

		pTunnels:       map[string]*mProxyTunnel{},
		waitingTunnels: map[uint64]*mProxyTunnel{},

		ch:         make(chan *channelEvent, _CHANNEL_SIZE),
		masterChan: ch,
	}
}

func recvHeartbeat(ctrl net.Conn, ch chan *channelEvent) {
	for {
		ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))

		if cmd, err := parseCommandV1(ctrl); err != nil {
			ch <- &channelEvent{_EVENT_SA_ERROR, err}
			break
		} else if cmd == CMD_V1_HEARTBEAT {
			continue
		} else {
			ch <- &channelEvent{_EVENT_SA_ERROR, ErrCommand}
			break
		}
	}
}

func (s *slaverAgent) serve() {
	go recvHeartbeat(s.ctrl, s.ch)

	for e := range s.ch {
		switch e.typ {
		case _EVENT_SA_ERROR:
			goto terminate
		case _EVENT_SA_BUILD_TUNNEL_REQ:
			req := e.data.(*buildTunnelReq)
			s.newProxyTunnel(req)
		case _EVENT_SA_SEND_DATA:
			s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if _, err := s.ctrl.Write(e.data.([]byte)); err != nil {
				logger.Error(err)
			}
		case _EVENT_SA_NEW_PTUNNEL_CONN:
			req := e.data.(ptunnelConnReq)
			s.waitingTunnels[req.tid] = req.t
		case _EVENT_SA_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if t, exist := s.waitingTunnels[req.tid]; exist {
				delete(s.waitingTunnels, req.tid)
				t.ch <- &channelEvent{_EVENT_PT_CONN_ACK, req}
			}
		}
	}
terminate:
	s.terminate()
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

func (s *slaverAgent) terminate() {
	s.masterChan <- &channelEvent{_EVENT_SA_TERMINATE, s.name}
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
