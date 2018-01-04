package reversetunnel

import (
	"bytes"
	"encoding/binary"
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
	conns      map[uint64]*sProxyTunnelConn
}

func newSlaverServer(conf *config) *slaverServer {
	s := &slaverServer{
		masterAddr: conf.JoinAddr,
		name:       conf.Name,
		ch:         make(chan *channelEvent, _CHANNEL_SIZE),
		bytesCh:    make(chan []byte, _CHANNEL_SIZE),
		conns:      map[uint64]*sProxyTunnelConn{},
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
			info := new(mProxyTunnelConnInfo)
			s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			info.mAddr, info.sAddr, info.tid, err = parseBuildTunnelV1(
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
			info := e.data.(*mProxyTunnelConnInfo)
			go s.startSProxyTunnelConn(info)
		case _EVENT_S_PT_CONN_SUCC:
			c := e.data.(*sProxyTunnelConn)
			logger.Infof("slaver: new conn, tid: %d\n", c.tid)
			s.conns[c.tid] = c
			go c.dataTransport()
		case _EVENT_S_PT_CONN_FAIL:
			err := e.data.(error)
			logger.Errorf("slaver: new conn error: %s\n", err)
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

func (s *slaverServer) startSProxyTunnelConn(info *mProxyTunnelConnInfo) {
	var (
		err error = nil
		c   *sProxyTunnelConn
	)
	defer func() {
		if err != nil {
			c.Close()
			(&channelEvent{_EVENT_S_PT_CONN_FAIL, err}).sendTo(s.ch)
		} else {
			(&channelEvent{_EVENT_S_PT_CONN_SUCC, c}).sendTo(s.ch)
		}
	}()

	c = &sProxyTunnelConn{
		tid:   info.tid,
		sChan: s.ch,
	}

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
	} else {
		buf.WriteByte(REP_SUCCEEDS)
	}

	c.mConn.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
	defer c.mConn.SetWriteDeadline(time.Time{})

	_, err = c.mConn.Write(buf.Bytes())
}

func (s *slaverServer) terminate() {
	close(s.bytesCh)
	s.ctrl.Close()
	waitForChanClean(s.ch)
}
