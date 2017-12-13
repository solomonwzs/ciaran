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
}

type sProxyTunnel struct {
	rEnd       net.Conn
	lEnd       net.Conn
	id         uint32
	targetPort string
}

func newSlaverServer(conf *config) *slaverServer {
	s := new(slaverServer)
	s.masterAddr = conf.JoinAddr
	s.name = conf.Name
	s.ch = make(chan *channelEvent, 10)
	return s
}

func (s *slaverServer) sendHeartbeat() {
	var err error
	for {
		s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
		if _, err = s.ctrl.Write(_BYTES_V1_HEARTBEAT); err != nil {
			(&channelEvent{_EVENT_S_ERROR, err}).sendTo(s.ch)
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

			break
		}
	}

	return
}

func (s *slaverServer) recvCommand() {
	for {
		s.ctrl.SetReadDeadline(time.Time{})
		if _, err := parseCommandV1(s.ctrl); err != nil {
			(&channelEvent{_EVENT_S_ERROR, err}).sendTo(s.ch)
		} else {
		}
	}
}

func (s *slaverServer) serve() {
	err := s.joinMaster()
	if err != nil {
		panic(err)
	}
	go s.sendHeartbeat()
	go s.recvCommand()
	for e := range s.ch {
		if e.typ == _EVENT_S_ERROR {
			logger.Error(e.data.(error))
			return
		}
	}
}

func (s *slaverServer) reset() {
	s.ctrl.Close()

	end := time.After(_NETWORK_TIMEOUT + 1*time.Second)
	for {
		select {
		case <-s.ch:
			break
		case <-end:
			return
		}
	}
}

func (s *slaverServer) run() {
	for {
		s.serve()
		s.reset()
	}
}
