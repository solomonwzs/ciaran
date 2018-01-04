package reversetunnel

import (
	"fmt"
	"logger"
	"net"
	"net/http"
	"time"
)

type tunnelConnAckReq struct {
	tid       uint64
	agentName string
	net.Conn
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
	go m.listenSlaverJoin()
	go m.listenTunnelConn()

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
			go m.startSlaverAgent(conn)
		case _EVENT_M_NEW_SLAVER_CONN_OK:
			sa := e.data.(*slaverAgent)
			m.slavers[sa.name] = sa
			logger.Infof("master: slaver [%s] join\n", sa.name)
			go sa.serve()
		case _EVENT_M_NEW_SLAVER_CONN_ERR:
			err := e.data.(error)
			logger.Infof("master: new slaver error: %s\n", err)
		case _EVENT_M_PTUNNEL_CONN_ACK:
			req := e.data.(*tunnelConnAckReq)
			if sa, exist := m.slavers[req.agentName]; exist {
				(&channelEvent{_EVENT_SA_PTUNNEL_CONN_ACK, req}).sendTo(
					sa.ch)
			}
		default:
		}
	}
}

func (m *masterServer) listenTunnelConn() {
	for {
		if conn, err := m.tunnel.Accept(); err != nil {
			logger.Error(err)
		} else {
			conn.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if cmd, err := parseCommandV1(conn); err != nil {
				logger.Error(err)
				conn.Close()
			} else if cmd != CMD_V1_BUILD_TUNNEL_ACK {
				logger.Error("error: command")
				conn.Close()
			} else if name, tid, err := parseBuildTunnelAckV1(
				conn); err != nil {
				logger.Error(err)
				conn.Close()
			} else {
				conn.SetReadDeadline(time.Time{})
				req := &tunnelConnAckReq{
					tid:       tid,
					agentName: name,
					Conn:      conn,
				}
				(&channelEvent{_EVENT_M_PTUNNEL_CONN_ACK, req}).sendTo(m.ch)
			}
		}
	}
}

func (m *masterServer) listenSlaverJoin() {
	for {
		if conn, err := m.ctrl.Accept(); err != nil {
			logger.Error(err)
		} else {
			(&channelEvent{_EVENT_M_NEW_SLAVER_CONN, conn}).sendTo(m.ch)
		}
	}
}

func (m *masterServer) startSlaverAgent(conn net.Conn) {
	var (
		cmd byte
		err error
		sa  *slaverAgent
	)

	conn.SetDeadline(time.Now().Add(_NETWORK_TIMEOUT))
	defer func() {
		if err == nil {
			conn.SetDeadline(time.Time{})
			(&channelEvent{_EVENT_M_NEW_SLAVER_CONN_OK, sa}).sendTo(m.ch)
		} else {
			conn.Close()
			(&channelEvent{_EVENT_M_NEW_SLAVER_CONN_ERR, err}).sendTo(m.ch)
		}
	}()

	if cmd, err = parseCommandV1(conn); err != nil {
		return
	} else if cmd != CMD_V1_JOIN {
		err = ErrCommand
		return
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
		sa = newSlaverAgent(name, conn, m.tunnelAddr, m.ch)
	}
}
