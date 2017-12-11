package reversetunnel

import (
	"fmt"
	"logger"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	_EVENT_SA_EVENT = iota
	_EVENT_SA_ERROR
	_EVENT_SA_BUILD_TUNNEL_REQ
	_EVENT_SA_SEND_DATA

	_EVENT_S_ERROR

	_EVENT_M_BUILD_TUNNEL_REQ
)

var (
	_HEARTBEAT_DURATION = 2 * time.Second
	_NETWORK_TIMEOUT    = 4 * time.Second
)

type channelEvent struct {
	typ  uint8
	data interface{}
}

type address struct {
	ip    []byte
	port  [2]byte
	atype byte
}

type commandEvent struct {
	typ  byte
	cmd  byte
	data interface{}
}

type masterServer struct {
	client net.Listener
	ctrl   net.Listener
	tunnel net.Listener

	tunnelAddr *address

	slavers     map[string]*slaverAgent
	slaversLock sync.RWMutex

	name string
	ch   chan *channelEvent
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

func newMasterServer(conf *config) *masterServer {
	m := new(masterServer)

	m.tunnelAddr = parseAddr(conf.TunnelAddr)
	if m.tunnelAddr == nil {
		panic(fmt.Sprintf("error: tunnel addr: %s", conf.TunnelAddr))
	}

	clientListener, err := net.Listen("tcp", conf.ClientAddr)
	if err != nil {
		panic(err)
	}
	m.client = clientListener

	ctrlListener, err := net.Listen("tcp", conf.CtrlAddr)
	if err != nil {
		panic(err)
	}
	m.ctrl = ctrlListener

	tunnelListener, err := net.Listen("tcp", conf.TunnelAddr)
	if err != nil {
		panic(err)
	}
	m.tunnel = tunnelListener

	m.slavers = map[string]*slaverAgent{}
	m.name = conf.Name
	m.ch = make(chan *channelEvent, 100)

	return m
}

func (m *masterServer) run() {
	go http.Serve(m.client, m)
	go m.listenSlaverJoin()

	for e := range m.ch {
		switch e.typ {
		case _EVENT_M_BUILD_TUNNEL_REQ:
			m.buildTunnel(e.data.(*buildTunnelReq))
		}
	}
}

func (m *masterServer) listenSlaverJoin() {
	for {
		conn, err := m.ctrl.Accept()
		if err != nil {
			logger.Error(err)
			continue
		}
		go func() {
			err := m.newSlaver(conn)
			if err != nil {
				logger.Error(err)
				conn.Close()
			}
		}()
	}
}

func (m *masterServer) newSlaver(conn net.Conn) (err error) {
	conn.SetDeadline(time.Now().Add(_NETWORK_TIMEOUT))
	defer conn.SetDeadline(time.Time{})

	var cmd byte
	if cmd, err = parseCommandV1(conn); err != nil {
		return
	}
	if cmd != CMD_V1_JOIN {
		return ErrCommand
	}

	var name string
	if name, err = parseJoinV1(conn); err != nil {
		return
	}

	if _, exist := m.slavers[name]; exist {
		conn.Write([]byte{PROTO_VER, CMD_V1_JOIN_ACK,
			REP_ERR_DUP_SLAVER_NAME})
		err = ErrDupicateSlaverName
	} else {
		if _, err = conn.Write([]byte{PROTO_VER, CMD_V1_JOIN_ACK,
			REP_SUCCEEDS}); err != nil {
			return
		}

		s := newSlaver(conn, m.tunnelAddr)
		m.slaversLock.Lock()
		m.slavers[name] = s
		m.slaversLock.Unlock()

		go func() {
			logger.Infof("slaver: %s join\n", name)
			s.serve()
			logger.Infof("slaver: %s left\n", name)

			m.slaversLock.Lock()
			delete(m.slavers, name)
			m.slaversLock.Unlock()

			s.close()
		}()
	}
	return
}

func (m *masterServer) buildTunnel(req *buildTunnelReq) (err error) {
	m.slaversLock.RLock()
	defer m.slaversLock.RUnlock()

	var (
		sa    *slaverAgent
		exist bool
	)

	if sa, exist = m.slavers[req.SlaverName]; !exist {
		return ErrSlaverNotExist
	}
	sa.ch <- &channelEvent{_EVENT_SA_BUILD_TUNNEL_REQ, req}

	return
}
