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

type mProxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn
	tid   uint64
}

type ptunnelConnReq struct {
	tid uint64
	t   *mProxyTunnel
}

type mProxyTunnel struct {
	clientListener net.Listener
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
		mAddr: mAddr,
		sAddr: sAddr,

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
			c := &mProxyTunnelConn{
				mConn: conn,
				tid:   newTid(),
			}
			ch <- &channelEvent{_EVENT_PT_NEW_CONN, c}
		}
	}
}

func (pt *mProxyTunnel) serve() {
	go listenClientConn(pt.clientListener, pt.ch)

	for e := range pt.ch {
		switch e.typ {
		case _EVENT_PT_NEW_CONN:
			c := e.data.(*mProxyTunnelConn)
			pt.waitingConns[c.tid] = c
			pt.agentChan <- &channelEvent{
				_EVENT_SA_NEW_PTUNNEL_CONN,
				&ptunnelConnReq{c.tid, pt}}

			buf := new(bytes.Buffer)
			buf.Write(pt.tunnelCmdPre)
			binary.Write(buf, binary.BigEndian, c.tid)
			pt.agentChan <- &channelEvent{_EVENT_SA_SEND_DATA, buf.Bytes()}
		}
	}
}

func (pt *mProxyTunnel) close() {
	pt.clientListener.Close()
}
