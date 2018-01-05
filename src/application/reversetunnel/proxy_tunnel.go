package reversetunnel

import (
	"bytes"
	"encoding/binary"
	"logger"
	"net"
)

type genCid func() connectionid

type ptunnelConnReq struct {
	cid connectionid
	t   *proxyTunnel
}

type proxyTunnel struct {
	clientListener net.Listener
	listenAddr     string
	mAddr          *address
	sAddr          *address
	tunnelCmdPre   []byte
	gcid           genCid

	ch        chan *channelEvent
	agentChan chan *channelEvent

	ptConns map[connectionid]*proxyTunnelConn
}

func newProxyTunnel(mAddr, sAddr *address, listenAddr string,
	agentChan chan *channelEvent, g genCid) (pt *proxyTunnel, err error) {
	pt = &proxyTunnel{
		listenAddr: listenAddr,
		mAddr:      mAddr,
		sAddr:      sAddr,
		gcid:       g,

		ch:        make(chan *channelEvent, _CHANNEL_SIZE),
		agentChan: agentChan,

		ptConns: map[connectionid]*proxyTunnelConn{},
	}

	buf := new(bytes.Buffer)
	buf.Write([]byte{PROTO_VER, CMD_V1_BUILD_TUNNEL})
	buf.Write([]byte{pt.mAddr.atype})
	buf.Write(pt.mAddr.ip)
	buf.Write(pt.mAddr.port[:])
	buf.Write([]byte{pt.sAddr.atype})
	buf.Write(pt.sAddr.ip)
	buf.Write(pt.sAddr.port[:])
	pt.tunnelCmdPre = buf.Bytes()

	pt.clientListener, err = net.Listen("tcp", listenAddr)
	return
}

func (pt *proxyTunnel) listenClientConn() {
	for {
		if conn, err := pt.clientListener.Accept(); err != nil {
			(&channelEvent{_EVENT_PT_ACCEPT_ERROR, err}).sendTo(pt.ch)
			return
		} else {
			(&channelEvent{_EVENT_PT_NEW_PTUNNEL_CONN, conn}).sendTo(pt.ch)
		}
	}
}

func (pt *proxyTunnel) serve() {
	go pt.listenClientConn()

	for e := range pt.ch {
		switch e.typ {
		case _EVENT_PT_NEW_PTUNNEL_CONN:
			conn := e.data.(net.Conn)
			c := newWaitingProxyTunnelConn(conn, pt.ch, pt.gcid())
			logger.Infof("master: new conn, cid: %d\n", c.cid)
			go c.serve()
			pt.ptConns[c.cid] = c
			(&channelEvent{
				_EVENT_SA_NEW_PTUNNEL_CONN,
				&ptunnelConnReq{c.cid, pt}}).sendTo(pt.agentChan)

			buf := new(bytes.Buffer)
			buf.Write(pt.tunnelCmdPre)
			binary.Write(buf, binary.BigEndian, c.cid)
			(&channelEvent{_EVENT_SA_SEND_DATA, buf.Bytes()}).sendTo(
				pt.agentChan)
		case _EVENT_PT_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if c, exist := pt.ptConns[req.cid]; exist {
				(&channelEvent{
					_EVENT_PTC_PTUNNEL_CONN_ACK,
					req.Conn}).sendTo(c.ch)
			} else {
				req.Close()
			}
		case _EVENT_PTC_TERMINATE:
			cid := e.data.(connectionid)
			if _, exist := pt.ptConns[cid]; exist {
				logger.Infof("master: end conn, cid: %d\n", cid)
				delete(pt.ptConns, cid)
			}
		case _EVENT_PT_ACCEPT_ERROR:
			logger.Error(e.data.(error))
			goto end
		case _EVENT_PT_SHUTDOWN:
			goto end
		default:
		}
	}
end:
	pt.terminate()
}

func (pt *proxyTunnel) terminate() {
	(&channelEvent{_EVENT_PT_TERMINATE, pt}).sendTo(pt.agentChan)
	pt.clientListener.Close()

	e := (&channelEvent{_EVENT_PTC_CLOSE, nil})
	for _, c := range pt.ptConns {
		e.sendTo(c.ch)
	}

	waitForChanClean(pt.ch)
}
