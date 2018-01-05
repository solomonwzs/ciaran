package reversetunnel

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"time"
)

const (
	_PTC_STATUS_WAITING = iota
	_PTC_STATUS_READY
	_PTC_STATUS_TRANSPORT
	_PTC_STATUS_CLOSE
)

type proxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn

	cid    connectionid
	status int

	superiorChan chan *channelEvent
	ch           chan *channelEvent
}

func newWaitingProxyTunnelConn(conn net.Conn, ptChan chan *channelEvent,
	cid connectionid) (c *proxyTunnelConn) {
	return &proxyTunnelConn{
		mConn:        conn,
		cid:          cid,
		superiorChan: ptChan,
		ch:           make(chan *channelEvent, _CHANNEL_SIZE),
		status:       _PTC_STATUS_WAITING,
	}
}

func asyncNewReadyProxyTunnelConn(info *proxyTunnelConnInfo,
	slaverName string, ch chan *channelEvent) {
	var err error = nil
	c := &proxyTunnelConn{
		cid:          info.cid,
		ch:           make(chan *channelEvent, _CHANNEL_SIZE),
		superiorChan: ch,
		status:       _PTC_STATUS_READY,
	}
	defer func() {
		if err != nil {
			c.Close()
			(&channelEvent{_EVENT_PTC_READY_FAIL, err}).sendTo(ch)
		} else {
			(&channelEvent{_EVENT_PTC_READY, c}).sendTo(ch)
		}
	}()

	if c.mConn, err = net.Dial("tcp", info.mAddr.String()); err != nil {
		return
	}

	buf := new(bytes.Buffer)
	buf.Write([]byte{PROTO_VER, CMD_V1_BUILD_TUNNEL_ACK})
	binary.Write(buf, binary.BigEndian, byte(len(slaverName)))
	buf.Write([]byte(slaverName))
	binary.Write(buf, binary.BigEndian, c.cid)

	if c.sConn, err = net.Dial("tcp", info.sAddr.String()); err != nil {
		buf.WriteByte(REP_ERR_CONN_REFUSED)
	} else {
		buf.WriteByte(REP_SUCCEEDS)
	}

	c.mConn.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
	defer c.mConn.SetWriteDeadline(time.Time{})

	if _, err0 := c.mConn.Write(buf.Bytes()); err == nil && err0 != nil {
		err = err0
	}
}

func (c *proxyTunnelConn) Close() error {
	if c.mConn != nil {
		c.mConn.Close()
	}
	if c.sConn != nil {
		c.sConn.Close()
	}
	return nil
}

func (c *proxyTunnelConn) dataTransport() {
	go io.Copy(c.mConn, c.sConn)
	io.Copy(c.sConn, c.mConn)
	(&channelEvent{_EVENT_PTC_TRANS_END, nil}).sendTo(c.ch)
}

func (c *proxyTunnelConn) serve() {
	for e := range c.ch {
		switch e.typ {
		case _EVENT_PTC_PTUNNEL_CONN_ACK:
			if c.status == _PTC_STATUS_WAITING {
				if e.data == nil {
					goto end
				}

				c.sConn = e.data.(net.Conn)
				c.status = _PTC_STATUS_TRANSPORT
				go c.dataTransport()
			}
		case _EVENT_PTC_TRANS_START:
			if c.status == _PTC_STATUS_READY {
				c.status = _PTC_STATUS_TRANSPORT
				go c.dataTransport()
			}
		case _EVENT_PTC_TRANS_END:
			if c.status == _PTC_STATUS_TRANSPORT {
				c.status = _PTC_STATUS_CLOSE
			}
			goto end
		case _EVENT_PTC_CLOSE:
			if c.status == _PTC_STATUS_TRANSPORT {
				c.status = _PTC_STATUS_CLOSE
			}
			goto end
		}
	}
end:
	c.terminate()
}

func (c *proxyTunnelConn) terminate() {
	c.Close()
	(&channelEvent{_EVENT_PTC_TERMINATE, c.cid}).sendTo(c.superiorChan)
	waitForChanClean(c.ch)
}
