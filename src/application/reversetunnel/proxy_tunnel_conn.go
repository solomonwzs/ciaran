package reversetunnel

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

const (
	_PTC_STATUS_WAITING = iota
	_PTC_STATUS_TRANSPORT
	_PTC_STATUS_CLOSE
)

type mProxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn

	tid    uint64
	status int

	tChan chan *channelEvent
	ch    chan *channelEvent
}

type sProxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn
	tid   uint64
	sChan chan *channelEvent
}

func newMProxyTunnelConn(conn net.Conn, ptChan chan *channelEvent) (
	c *mProxyTunnelConn) {
	return &mProxyTunnelConn{
		mConn:  conn,
		tid:    newTid(),
		tChan:  ptChan,
		ch:     make(chan *channelEvent, _CHANNEL_SIZE),
		status: _PTC_STATUS_WAITING,
	}
}

func newSProxyTunnelConn(info *mProxyTunnelConnInfo,
	s *slaverServer) (c *sProxyTunnelConn, err error) {
	c = &sProxyTunnelConn{
		tid:   info.tid,
		sChan: s.ch,
	}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	if c.mConn, err = net.Dial("tcp", info.mAddr.String()); err != nil {
		return
	}

	buf := new(bytes.Buffer)
	buf.Write([]byte{PROTO_VER, CMD_V1_BUILD_TUNNEL_ACK})
	binary.Write(buf, binary.BigEndian, byte(len(s.name)))
	buf.Write([]byte(s.name))
	binary.Write(buf, binary.BigEndian, c.tid)

	if c.sConn, err = net.Dial("tcp", info.sAddr.String()); err != nil {
		buf.WriteByte(REP_ERR_CONN_REFUSED)
		return
	} else {
		buf.WriteByte(REP_SUCCEEDS)
	}

	return
}

func (c *sProxyTunnelConn) dataTransport() {
	go io.Copy(c.mConn, c.sConn)
	io.Copy(c.sConn, c.mConn)
	c.terminate()
}

func (c *sProxyTunnelConn) Close() error {
	if c.mConn != nil {
		c.mConn.Close()
	}
	if c.sConn != nil {
		c.sConn.Close()
	}
	return nil
}

func (c *sProxyTunnelConn) terminate() {
	c.Close()
	(&channelEvent{_EVENT_PTC_TERMINATE, nil}).sendTo(c.sChan)
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

func (c *mProxyTunnelConn) dataTransport() {
	go io.Copy(c.mConn, c.sConn)
	io.Copy(c.sConn, c.mConn)
	(&channelEvent{_EVENT_PTC_TRANS_END, nil}).sendTo(c.ch)
}

func (c *mProxyTunnelConn) serve() {
	for e := range c.ch {
		switch e.typ {
		case _EVENT_PTC_PTUNNEL_CONN_ACK:
			if c.status == _PTC_STATUS_WAITING {
				c.sConn = e.data.(net.Conn)
				c.status = _PTC_STATUS_TRANSPORT
				go c.dataTransport()
			}
		case _EVENT_PTC_TRANS_END:
			if c.status == _PTC_STATUS_TRANSPORT {
				c.status = _PTC_STATUS_CLOSE
				(&channelEvent{_EVENT_PTC_TERMINATE, c.tid}).sendTo(
					c.tChan)
			}
			goto end
		case _EVENT_PTC_CLOSE:
			if c.status == _PTC_STATUS_TRANSPORT {
				c.status = _PTC_STATUS_CLOSE
				c.Close()
			}
			goto end
		}
	}
end:
	c.terminate()
}

func (c *mProxyTunnelConn) terminate() {
	c.Close()
	(&channelEvent{_EVENT_PTC_TERMINATE, nil}).sendTo(c.tChan)
	waitForChanClean(c.ch)
}
