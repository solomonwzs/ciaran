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
	mConn     net.Conn
	sConn     net.Conn
	id        uint32
	agentChan chan interface{}
}

type mProxyTunnel struct {
	clientListener net.Listener
	mAddr          *address
	sAddr          *address
	tunnelCmdPre   []byte
}

func newMProxyTunnel(mAddr, sAddr *address, listenAddr string,
	agentChan chan *channelEvent) (pt *mProxyTunnel, err error) {
	pt = &mProxyTunnel{
		mAddr: mAddr,
		sAddr: sAddr,
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

func (pt *mProxyTunnel) serv() {
	for {
		if _, err := pt.clientListener.Accept(); err != nil {
			return
		} else {
			buf := new(bytes.Buffer)
			buf.Write(pt.tunnelCmdPre)
			tid := newTid()
			binary.Write(buf, binary.BigEndian, tid)
		}
	}
}

func (pt *mProxyTunnel) close() {
	pt.clientListener.Close()
}
