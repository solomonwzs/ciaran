package reversetunnel

import (
	"fmt"
	"logger"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	_EVENT_SA_EVENT = iota
	_EVENT_SA_ERROR
	_EVENT_SA_BUILD_TUNNEL_REQ
	_EVENT_SA_SEND_DATA
	_EVENT_SA_PTUNNEL_CONN_ACK
	_EVENT_SA_TERMINATE
	_EVENT_SA_SHUTDOWN
	_EVENT_SA_NEW_PTUNNEL_CONN

	_EVENT_S_ERROR

	_EVENT_M_NEW_SLAVER_CONN
	_EVENT_M_PTUNNEL_CONN_ACK
	_EVENT_M_BUILD_TUNNEL_REQ

	_EVENT_PT_NEW_CONN
	_EVENT_PT_CONN_ACK
	_EVENT_PT_CONN_CLOSE
	_EVENT_PT_SHUTDOWN
	_EVENT_PT_TERMINATE
)

const _CHANNEL_SIZE = 100

var (
	_HEARTBEAT_DURATION = 2 * time.Second
	_NETWORK_TIMEOUT    = 4 * time.Second
)

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

type tunnelConnAckReq struct {
	tid       uint64
	agentName string
	conn      net.Conn
}

type masterServer struct {
	client net.Listener
	ctrl   net.Listener
	tunnel net.Listener

	tunnelAddr *address

	slavers map[string]*slaverAgent

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
	m.ch = make(chan *channelEvent, _CHANNEL_SIZE)

	return m
}

func (m *masterServer) serve() {
	go http.Serve(m.client, &masterHttp{m.ch})
	go listenSlaverJoin(m.ctrl, m.ch)
	go listenTunnelConn(m.tunnel, m.ch)

	for e := range m.ch {
		switch e.typ {
		case _EVENT_M_BUILD_TUNNEL_REQ:
			req := e.data.(*buildTunnelReq)
			if sa, exist := m.slavers[req.SlaverName]; exist {
				(&channelEvent{_EVENT_SA_BUILD_TUNNEL_REQ, req}).sendTo(
					sa.ch)
			}
		case _EVENT_SA_TERMINATE:
			name := e.data.(string)
			delete(m.slavers, name)
		case _EVENT_M_NEW_SLAVER_CONN:
			conn := e.data.(net.Conn)
			if err := m.startSlaverAgent(conn); err != nil {
				conn.Close()
			}
		case _EVENT_M_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if sa, exist := m.slavers[req.agentName]; exist {
				(&channelEvent{_EVENT_SA_PTUNNEL_CONN_ACK, req}).sendTo(
					sa.ch)
			}
		}
	}
}

func listenTunnelConn(tunnel net.Listener, ch chan *channelEvent) {
	for {
		if conn, err := tunnel.Accept(); err != nil {
			logger.Error(err)
		} else {
			conn.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if cmd, err := parseCommandV1(conn); err != nil {
				logger.Error(err)
			} else if cmd != CMD_V1_BUILD_TUNNEL_ACK {
				logger.Error("error: command")
			} else if name, tid, err := parseBuildTunnelAckV1(
				conn); err != nil {
				logger.Error(err)
			} else {
				req := &tunnelConnAckReq{
					tid:       tid,
					agentName: name,
					conn:      conn,
				}
				(&channelEvent{_EVENT_M_PTUNNEL_CONN_ACK, req}).sendTo(ch)
			}
		}
	}
}

func listenSlaverJoin(ctrl net.Listener, ch chan *channelEvent) {
	for {
		if conn, err := ctrl.Accept(); err != nil {
			logger.Error(err)
		} else {
			(&channelEvent{_EVENT_M_NEW_SLAVER_CONN, conn}).sendTo(ch)
		}
	}
}

func (m *masterServer) startSlaverAgent(conn net.Conn) (err error) {
	conn.SetDeadline(time.Now().Add(_NETWORK_TIMEOUT))
	defer conn.SetDeadline(time.Time{})

	var cmd byte
	if cmd, err = parseCommandV1(conn); err != nil {
		return
	} else if cmd != CMD_V1_JOIN {
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

		s := newSlaverAgent(name, conn, m.tunnelAddr, m.ch)
		m.slavers[name] = s

		logger.Infof("slaver: %s join\n", name)
		go s.serve()
		logger.Infof("slaver: %s left\n", name)
	}
	return
}
