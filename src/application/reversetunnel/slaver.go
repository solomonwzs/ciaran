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

type slaverServer struct {
	ctrl       net.Conn
	masterAddr string
	name       string
	ch         chan *channelEvent
	bytesCh    chan []byte
	conns      map[connectionid]*proxyTunnelConn
}

func newSlaverServer(conf *config) *slaverServer {
	s := &slaverServer{
		masterAddr: conf.JoinAddr,
		name:       conf.Name,
		ch:         make(chan *channelEvent, _CHANNEL_SIZE),
		bytesCh:    make(chan []byte, _CHANNEL_SIZE),
		conns:      map[connectionid]*proxyTunnelConn{},
	}
	return s
}

func (s *slaverServer) heartbeat() {
	for {
		(&channelEvent{_EVENT_S_SEND_DATA, _BYTES_V1_HEARTBEAT}).sendTo(s.ch)
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

func (s *slaverServer) recvCommand() {
	var (
		cmd byte
		err error
	)
	for {
		s.ctrl.SetReadDeadline(time.Time{})
		if cmd, err = parseCommandV1(s.ctrl); err != nil {
			if err == ErrIO {
				(&channelEvent{_EVENT_S_CONN_ERROR, err}).sendTo(s.ch)
				return
			} else {
				(&channelEvent{_EVENT_S_CMD_ERROR, err}).sendTo(s.ch)
				continue
			}
		}

		switch cmd {
		case CMD_V1_BUILD_TUNNEL:
			info := new(proxyTunnelConnInfo)
			s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			info.mAddr, info.sAddr, info.cid, err = parseBuildTunnelV1(
				s.ctrl)
			if err != nil {
				(&channelEvent{_EVENT_S_CMD_ERROR, err}).sendTo(s.ch)
				break
			}
			(&channelEvent{_EVENT_S_PT_CONN_INFO, info}).sendTo(s.ch)
		}
	}
}

func (s *slaverServer) serve() {
	err := s.joinMaster()
	if err != nil {
		panic(err)
	}

	go sendData(s.ctrl, s.bytesCh, s.ch)
	go s.heartbeat()
	go s.recvCommand()

	for e := range s.ch {
		switch e.typ {
		case _EVENT_S_ERROR:
			logger.Error(e.data.(error))
			goto end
		case _EVENT_S_PT_CONN_INFO:
			info := e.data.(*proxyTunnelConnInfo)
			go asyncNewReadyProxyTunnelConn(info, s.name, s.ch)
		case _EVENT_PTC_READY:
			c := e.data.(*proxyTunnelConn)
			logger.Infof("slaver: new conn, cid: %d\n", c.cid)
			s.conns[c.cid] = c
			go c.serve()
			(&channelEvent{_EVENT_PTC_TRANS_START, nil}).sendTo(c.ch)
		case _EVENT_PTC_READY_FAIL:
			err := e.data.(error)
			logger.Errorf("slaver: new conn error: %s\n", err)
		case _EVENT_PTC_TERMINATE:
			cid := e.data.(connectionid)
			if _, exist := s.conns[cid]; exist {
				logger.Infof("slaver: end conn, cid: %d\n", cid)
				delete(s.conns, cid)
			}
		case _EVENT_S_SEND_DATA:
			data := e.data.([]byte)
			go func() { s.bytesCh <- data }()
		case _EVENT_X_SEND_DATA_ERR:
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

func (s *slaverServer) terminate() {
	close(s.bytesCh)
	s.ctrl.Close()

	e := (&channelEvent{_EVENT_PTC_CLOSE, nil})
	for _, c := range s.conns {
		e.sendTo(c.ch)
	}

	waitForChanClean(s.ch)
}
