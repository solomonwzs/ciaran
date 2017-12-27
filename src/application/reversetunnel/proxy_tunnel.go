package reversetunnel

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

var (
	_TID      = uint64(time.Now().UnixNano())
	_TID_LOCK = sync.Mutex{}
)

func newTid() uint64 {
	_TID_LOCK.Lock()
	defer _TID_LOCK.Unlock()

	_TID += 1
	return _TID
}

type ptunnelConnReq struct {
	tid uint64
	t   *mProxyTunnel
}

type mProxyTunnel struct {
	clientListener net.Listener
	listenAddr     string
	mAddr          *address
	sAddr          *address
	tunnelCmdPre   []byte

	ch        chan *channelEvent
	agentChan chan *channelEvent

	ptConns map[uint64]*mProxyTunnelConn
}

func newMProxyTunnel(mAddr, sAddr *address, listenAddr string,
	agentChan chan *channelEvent) (pt *mProxyTunnel, err error) {
	pt = &mProxyTunnel{
		listenAddr: listenAddr,
		mAddr:      mAddr,
		sAddr:      sAddr,

		ch:        make(chan *channelEvent, _CHANNEL_SIZE),
		agentChan: agentChan,

		ptConns: map[uint64]*mProxyTunnelConn{},
	}

	buf := new(bytes.Buffer)
	buf.Write([]byte{PROTO_VER, CMD_V1_JOIN})
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

func listenClientConn(l net.Listener, ch chan *channelEvent) {
	for {
		if conn, err := l.Accept(); err != nil {
		} else {
			(&channelEvent{_EVENT_PT_NEW_PTUNNEL_CONN, conn}).sendTo(ch)
		}
	}
}

func (pt *mProxyTunnel) serve() {
	go listenClientConn(pt.clientListener, pt.ch)

	for e := range pt.ch {
		switch e.typ {
		case _EVENT_PT_NEW_PTUNNEL_CONN:
			conn := e.data.(net.Conn)
			c := newMProxyTunnelConn(conn, pt.ch)
			go c.serve()
			pt.ptConns[c.tid] = c
			(&channelEvent{
				_EVENT_SA_NEW_PTUNNEL_CONN,
				&ptunnelConnReq{c.tid, pt}}).sendTo(pt.agentChan)

			buf := new(bytes.Buffer)
			buf.Write(pt.tunnelCmdPre)
			binary.Write(buf, binary.BigEndian, c.tid)
			(&channelEvent{_EVENT_SA_SEND_DATA, buf.Bytes()}).sendTo(
				pt.agentChan)
		case _EVENT_PT_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if c, exist := pt.ptConns[req.tid]; exist {
				(&channelEvent{
					_EVENT_PTC_PTUNNEL_CONN_ACK,
					req.Conn}).sendTo(c.ch)
			} else {
				req.Close()
			}
		case _EVENT_PTC_TERMINATE:
			tid := e.data.(uint64)
			if _, exist := pt.ptConns[tid]; exist {
				delete(pt.ptConns, tid)
			}
		case _EVENT_PT_SHUTDOWN:
			goto end
		}
	}
end:
	pt.terminate()
}

func (pt *mProxyTunnel) terminate() {
	(&channelEvent{_EVENT_PT_TERMINATE, pt}).sendTo(pt.agentChan)
	pt.clientListener.Close()

	e := (&channelEvent{_EVENT_PTC_CLOSE, nil})
	for _, c := range pt.ptConns {
		e.sendTo(c.ch)
	}

	waitForChanClean(pt.ch)
}
