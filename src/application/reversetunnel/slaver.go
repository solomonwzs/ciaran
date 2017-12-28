package reversetunnel

import (
	"bytes"
	"logger"
	"net"
	"time"
)

var (
	_BYTES_V1_HEARTBEAT = []byte{PROTO_VER, CMD_V1_HEARTBEAT}
)

type mProxyTunnelConnInfo struct {
	mAddr *address
	sAddr *address
	tid   uint64
}

type slaverServer struct {
	ctrl       net.Conn
	masterAddr string
	name       string
	ch         chan *channelEvent
	conns      map[uint64]*sProxyTunnelConn
}

func newSlaverServer(conf *config) *slaverServer {
	s := &slaverServer{
		masterAddr: conf.JoinAddr,
		name:       conf.Name,
		ch:         make(chan *channelEvent, 10),
		conns:      map[uint64]*sProxyTunnelConn{},
	}
	return s
}

func sendHeartbeat(conn net.Conn, ch chan *channelEvent) {
	for {
		conn.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
		if _, err := conn.Write(_BYTES_V1_HEARTBEAT); err != nil {
			(&channelEvent{_EVENT_S_HEARTBEAT_ERROR, err}).sendTo(ch)
			return
		}
		time.Sleep(_HEARTBEAT_DURATION)
	}
}

func (s *slaverServer) joinMaster() (err error) {
	buf := new(bytes.Buffer)
	buf.Write([]byte{PROTO_VER, CMD_V1_JOIN, byte(len(s.name))})
	buf.WriteString(s.name)
	retry := false
	for {
		if s.ctrl != nil {
			s.ctrl.Close()
		}
		if err != nil {
			logger.Error(err)
		}
		if retry {
			logger.Info("slaver: wait for retry")
			time.Sleep(2 * time.Second)
		}

		retry = true
		s.ctrl, err = net.Dial("tcp", s.masterAddr)
		if err != nil {
			continue
		} else {
			s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if _, err = s.ctrl.Write(buf.Bytes()); err != nil {
				continue
			}

			s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			var b byte
			b, err = parseCommandV1(s.ctrl)
			if err != nil {
				continue
			}
			if b != CMD_V1_JOIN_ACK {
				err = ErrCommand
				continue
			}

			b, err = parseJoinAckV1(s.ctrl)
			if err != nil {
				continue
			}
			if b != REP_SUCCEEDS {
				if b == REP_ERR_DUP_SLAVER_NAME {
					err = ErrDupicateSlaverName
				} else {
					err = ErrCommand
				}
				continue
			}

			logger.Info("slaver: join master")
			break
		}
	}

	return
}

func recvCommand(conn net.Conn, ch chan *channelEvent) {
	var (
		cmd byte
		err error
	)
	for {
		conn.SetReadDeadline(time.Time{})
		if cmd, err = parseCommandV1(conn); err != nil {
			if err == ErrIO {
				(&channelEvent{_EVENT_S_CONN_ERROR, err}).sendTo(ch)
				return
			} else {
				(&channelEvent{_EVENT_S_CMD_ERROR, err}).sendTo(ch)
				continue
			}
		}

		switch cmd {
		case CMD_V1_BUILD_TUNNEL:
			info := new(mProxyTunnelConnInfo)
			conn.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			info.mAddr, info.sAddr, info.tid, err = parseBuildTunnelV1(
				conn)
			if err != nil {
				(&channelEvent{_EVENT_S_CMD_ERROR, err}).sendTo(ch)
				break
			}
			(&channelEvent{_EVENT_S_PT_CONN_INFO, info}).sendTo(ch)
		}
	}
}

func (s *slaverServer) serve() {
	err := s.joinMaster()
	if err != nil {
		panic(err)
	}
	go sendHeartbeat(s.ctrl, s.ch)
	go recvCommand(s.ctrl, s.ch)

	for e := range s.ch {
		switch e.typ {
		case _EVENT_S_ERROR:
			logger.Error(e.data.(error))
			goto end
		case _EVENT_S_PT_CONN_INFO:
			info := e.data.(*mProxyTunnelConnInfo)
			c, err0 := newSProxyTunnelConn(info, s)
			if err0 != nil {
				logger.Error(err0)
				continue
			}
			go c.dataTransport()
		case _EVENT_S_HEARTBEAT_ERROR:
			logger.Error(e.data.(error))
			goto end
		case _EVENT_S_CMD_ERROR:
		case _EVENT_S_CONN_ERROR:
			logger.Error(e.data.(error))
			goto end
		default:
		}
	}
end:
	s.terminate()
}

func runSlaver(s *slaverServer) {
	for {
		s.serve()
	}
}

func (s *slaverServer) terminate() {
	s.ctrl.Close()
	waitForChanClean(s.ch)
}
