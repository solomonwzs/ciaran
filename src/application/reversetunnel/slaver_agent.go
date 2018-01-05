package reversetunnel

import (
	"logger"
	"net"
	"sync"
	"time"
)

type slaverAgent struct {
	name string

	tunnelAddr *address
	ctrl       net.Conn

	pTunnels       map[string]*proxyTunnel
	waitingTunnels map[connectionid]*proxyTunnel

	ch         chan *channelEvent
	masterChan chan *channelEvent
	bytesCh    chan []byte

	currentCid connectionid
	cidLock    *sync.Mutex
}

func newSlaverAgent(name string, conn net.Conn, tunnelAddr *address,
	ch chan *channelEvent) *slaverAgent {

	return &slaverAgent{
		name: name,

		tunnelAddr: tunnelAddr,
		ctrl:       conn,

		pTunnels:       map[string]*proxyTunnel{},
		waitingTunnels: map[connectionid]*proxyTunnel{},

		ch:         make(chan *channelEvent, _CHANNEL_SIZE),
		bytesCh:    make(chan []byte, _CHANNEL_SIZE),
		masterChan: ch,

		currentCid: 1,
		cidLock:    &sync.Mutex{},
	}
}

func (sa *slaverAgent) newCid() connectionid {
	sa.cidLock.Lock()
	defer sa.cidLock.Unlock()

	cid := sa.currentCid
	sa.currentCid += 1
	return cid
}

func (sa *slaverAgent) recvHeartbeat() {
	for {
		sa.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))

		if cmd, err := parseCommandV1(sa.ctrl); err != nil {
			(&channelEvent{_EVENT_SA_ERROR, err}).sendTo(sa.ch)
			break
		} else if cmd == CMD_V1_HEARTBEAT {
			continue
		} else {
			(&channelEvent{_EVENT_SA_ERROR, ErrCommand}).sendTo(sa.ch)
			break
		}
	}
}

func (sa *slaverAgent) serve() {
	go sa.recvHeartbeat()
	go sendData(sa.ctrl, sa.bytesCh, sa.ch)

	for e := range sa.ch {
		switch e.typ {
		case _EVENT_SA_ERROR:
			goto end
		case _EVENT_SA_SHUTDOWN:
			goto end
		case _EVENT_SA_BUILD_TUNNEL_REQ:
			req := e.data.(*buildTunnelReq)
			sa.newProxyTunnel(req)
		case _EVENT_SA_SEND_DATA:
			data := e.data.([]byte)
			go func() { sa.bytesCh <- data }()
		case _EVENT_X_SEND_DATA_ERR:
			err := e.data.(error)
			logger.Error(err)
			goto end
		case _EVENT_SA_NEW_PTUNNEL_CONN:
			req := e.data.(*ptunnelConnReq)
			sa.waitingTunnels[req.cid] = req.t
		case _EVENT_SA_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if t, exist := sa.waitingTunnels[req.cid]; exist {
				delete(sa.waitingTunnels, req.cid)
				(&channelEvent{_EVENT_PT_PTUNNEL_CONN_ACK, req}).sendTo(t.ch)
			}
		case _EVENT_PT_TERMINATE:
			pt := e.data.(*proxyTunnel)
			delete(sa.pTunnels, pt.listenAddr)
			for cid, _ := range pt.ptConns {
				delete(sa.waitingTunnels, cid)
			}
		default:
		}
	}
end:
	logger.Infof("master: slaver [%s] left\n", sa.name)
	sa.terminate()
}

func (sa *slaverAgent) newProxyTunnel(req *buildTunnelReq) {
	sAddr := parseAddr(req.SAddr)
	if sAddr == nil {
		return
	}
	pTunnel, err := newProxyTunnel(sa.tunnelAddr, sAddr, req.MAddr,
		sa.ch, sa.newCid)
	if err != nil {
		return
	}
	go pTunnel.serve()
	sa.pTunnels[req.MAddr] = pTunnel

	logger.Infof("master: new tunnel: [master:%s] <-> [%s:%s]\n",
		req.MAddr, req.SlaverName, req.SAddr)
}

func (sa *slaverAgent) terminate() {
	(&channelEvent{_EVENT_SA_TERMINATE, sa.name}).sendTo(sa.masterChan)
	sa.ctrl.Close()

	close(sa.bytesCh)

	shutdownEvent := &channelEvent{_EVENT_PT_SHUTDOWN, nil}
	for _, pTunnel := range sa.pTunnels {
		shutdownEvent.sendTo(pTunnel.ch)
	}

	waitForChanClean(sa.ch)
}
