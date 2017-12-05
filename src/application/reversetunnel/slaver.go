package reversetunnel

import (
	"bytes"
	"logger"
	"net"
	"time"
)

type slaverServer struct {
	ctrl       net.Conn
	masterAddr string
	name       string
}

type sProxyTunnel struct {
	rEnd       net.Conn
	lEnd       net.Conn
	id         uint64
	targetPort string
}

func newSlaverServer(conf *config) *slaverServer {
	s := new(slaverServer)
	s.masterAddr = conf.JoinAddr
	s.name = conf.Name
	return s
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
			s.ctrl.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err = s.ctrl.Write(buf.Bytes()); err != nil {
				continue
			}

			s.ctrl.SetReadDeadline(time.Now().Add(10 * time.Second))
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

func (s *slaverServer) run() {
	err := s.joinMaster()
	if err != nil {
		panic(err)
	}
	for {
	}
}
