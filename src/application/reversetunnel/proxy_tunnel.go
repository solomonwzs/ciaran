package reversetunnel

import (
	"bytes"
	"encoding/binary"
	"io"
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

type mProxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn
	tChan chan *channelEvent
	tid   uint64
}

func (c *mProxyTunnelConn) Close() error {
	if c.mConn != nil {
		c.mConn.Close()
	}
	if c.sConn != nil {
		c.sConn.Close()
	}
	return nil
}

func (c *mProxyTunnelConn) serve() {
	go io.Copy(c.mConn, c.sConn)
	io.Copy(c.sConn, c.mConn)
	(&channelEvent{_EVENT_PT_CONN_CLOSE, c.tid}).sendTo(c.tChan)
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

	waitingConns map[uint64]*mProxyTunnelConn
	runningConns map[uint64]*mProxyTunnelConn
}

func newMProxyTunnel(mAddr, sAddr *address, listenAddr string,
	agentChan chan *channelEvent) (pt *mProxyTunnel, err error) {
	pt = &mProxyTunnel{
		listenAddr: listenAddr,
		mAddr:      mAddr,
		sAddr:      sAddr,

		ch:        make(chan *channelEvent, _CHANNEL_SIZE),
		agentChan: agentChan,

		waitingConns: map[uint64]*mProxyTunnelConn{},
		runningConns: map[uint64]*mProxyTunnelConn{},
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
			(&channelEvent{_EVENT_PT_NEW_CONN, conn}).sendTo(ch)
		}
	}
}

func (pt *mProxyTunnel) serve() {
	go listenClientConn(pt.clientListener, pt.ch)

	for e := range pt.ch {
		switch e.typ {
		case _EVENT_PT_NEW_CONN:
			conn := e.data.(net.Conn)
			c := &mProxyTunnelConn{
				mConn: conn,
				tChan: pt.ch,
				tid:   newTid(),
			}
			pt.waitingConns[c.tid] = c
			(&channelEvent{
				_EVENT_SA_NEW_PTUNNEL_CONN,
				&ptunnelConnReq{c.tid, pt}}).sendTo(pt.agentChan)

			buf := new(bytes.Buffer)
			buf.Write(pt.tunnelCmdPre)
			binary.Write(buf, binary.BigEndian, c.tid)
			(&channelEvent{_EVENT_SA_SEND_DATA, buf.Bytes()}).sendTo(
				pt.agentChan)
		case _EVENT_PT_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if c, exist := pt.waitingConns[req.tid]; exist {
				c.sConn = req.Conn
				delete(pt.waitingConns, req.tid)
				pt.runningConns[req.tid] = c
			}
		case _EVENT_PT_CONN_CLOSE:
			tid := e.data.(uint64)
			if c, exist := pt.runningConns[tid]; exist {
				delete(pt.runningConns, tid)
				c.Close()
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

	for _, c := range pt.waitingConns {
		c.Close()
	}
	for _, c := range pt.runningConns {
		c.Close()
	}

	waitForChanClear(pt.ch)
}
