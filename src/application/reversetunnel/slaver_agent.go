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
			(&channelEvent{_EVENT_SA_ERROR, err}).sendTo(ch)
			break
		} else if cmd == CMD_V1_HEARTBEAT {
			continue
		} else {
			(&channelEvent{_EVENT_SA_ERROR, ErrCommand}).sendTo(ch)
			break
		}
	}
}

func (s *slaverAgent) serve() {
	go recvHeartbeat(s.ctrl, s.ch)

	for e := range s.ch {
		switch e.typ {
		case _EVENT_SA_ERROR:
			goto end
		case _EVENT_SA_SHUTDOWN:
			goto end
		case _EVENT_SA_BUILD_TUNNEL_REQ:
			req := e.data.(*buildTunnelReq)
			s.newProxyTunnel(req)
		case _EVENT_SA_SEND_DATA:
			s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			logger.Debug(e.data.([]byte))
			if _, err := s.ctrl.Write(e.data.([]byte)); err != nil {
				logger.Error(err)
			}
		case _EVENT_SA_NEW_PTUNNEL_CONN:
			req := e.data.(*ptunnelConnReq)
			s.waitingTunnels[req.tid] = req.t
		case _EVENT_SA_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if t, exist := s.waitingTunnels[req.tid]; exist {
				delete(s.waitingTunnels, req.tid)
				(&channelEvent{_EVENT_PT_PTUNNEL_CONN_ACK, req}).sendTo(t.ch)
			}
		case _EVENT_PT_TERMINATE:
			pt := e.data.(*mProxyTunnel)
			delete(s.pTunnels, pt.listenAddr)
			for tid, _ := range pt.ptConns {
				delete(s.waitingTunnels, tid)
			}
		}
	}
end:
	logger.Infof("master: slaver [%s] left\n", s.name)
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
	go pTunnel.serve()
	s.pTunnels[req.MAddr] = pTunnel
}

func (s *slaverAgent) terminate() {
	(&channelEvent{_EVENT_SA_TERMINATE, s.name}).sendTo(s.masterChan)
	s.ctrl.Close()

	shutdownEvent := &channelEvent{_EVENT_PT_SHUTDOWN, nil}
	for _, pTunnel := range s.pTunnels {
		shutdownEvent.sendTo(pTunnel.ch)
	}

	waitForChanClean(s.ch)
}
