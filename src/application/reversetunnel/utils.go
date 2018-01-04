package reversetunnel

import (
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	_EVENT_M_NEW_SLAVER_CONN = iota
	_EVENT_M_NEW_SLAVER_CONN_OK
	_EVENT_M_NEW_SLAVER_CONN_ERR
	_EVENT_M_PTUNNEL_CONN_ACK
	_EVENT_M_BUILD_TUNNEL_REQ

	_EVENT_S_ERROR
	_EVENT_S_CONN_ERROR
	_EVENT_S_CMD_ERROR
	_EVENT_S_RECV_CMD_ERROR
	_EVENT_S_SEND_DATA
	_EVENT_S_PT_CONN_INFO
	_EVENT_S_PT_CONN_SUCC
	_EVENT_S_PT_CONN_FAIL

	_EVENT_SA_EVENT
	_EVENT_SA_ERROR
	_EVENT_SA_BUILD_TUNNEL_REQ
	_EVENT_SA_SEND_DATA
	_EVENT_SA_PTUNNEL_CONN_ACK
	_EVENT_SA_TERMINATE
	_EVENT_SA_SHUTDOWN
	_EVENT_SA_NEW_PTUNNEL_CONN

	_EVENT_PT_NEW_PTUNNEL_CONN
	_EVENT_PT_ACCEPT_ERROR
	_EVENT_PT_PTUNNEL_CONN_ACK
	_EVENT_PT_SHUTDOWN
	_EVENT_PT_TERMINATE

	_EVENT_PTC_PTUNNEL_CONN_ACK
	_EVENT_PTC_CLOSE
	_EVENT_PTC_TRANS_END
	_EVENT_PTC_TERMINATE

	_EVENT_X_SEND_DATA_ERR
)

const _CHANNEL_SIZE = 100

var (
	_HEARTBEAT_DURATION = 2 * time.Second
	_NETWORK_TIMEOUT    = 4 * time.Second
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

type mProxyTunnelConnInfo struct {
	mAddr *address
	sAddr *address
	tid   uint64
}

type channelEvent struct {
	typ  uint8
	data interface{}
}

func (e *channelEvent) sendTo(ch chan *channelEvent) {
	go func() {
		ch <- e
	}()
}

type address struct {
	ip    []byte
	port  [2]byte
	atype byte
}

func (addr *address) String() string {
	port := strconv.Itoa(int(addr.port[0])<<8 | int(addr.port[1]))
	if (addr.atype == ATYP_IPV4 && len(addr.ip) == 4) ||
		(addr.atype == ATYP_IPV6 && len(addr.ip) == 16) {
		return net.JoinHostPort(net.IP(addr.ip).String(), port)
	}
	return ""
}

func waitForChanClean(ch chan *channelEvent) {
	end := time.After(_NETWORK_TIMEOUT + 1*time.Second)
	for {
		select {
		case e := <-ch:
			if d, ok := e.data.(io.Closer); ok {
				d.Close()
			}
		case <-end:
			return
		}
	}
}

func sendData(ctrl net.Conn, bytesCh chan []byte,
	ch chan *channelEvent) {
	for d := range bytesCh {
		ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
		if _, err := ctrl.Write(d); err != nil {
			(&channelEvent{_EVENT_X_SEND_DATA_ERR, err}).sendTo(ch)
		}
	}
}

func parseAddr(s string) (addr *address) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil
	}

	ip := net.ParseIP(host)
	addr = &address{}
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			addr.atype = ATYP_IPV4
			addr.ip = ip[net.IPv6len-net.IPv4len:]
			break
		} else if host[i] == ':' {
			addr.atype = ATYP_IPV6
			addr.ip = ip
			break
		}
	}
	p, _ := strconv.Atoi(port)
	addr.port[0] = byte(uint16(p) >> 8)
	addr.port[1] = byte(uint16(p) & 0xff)

	return
}
