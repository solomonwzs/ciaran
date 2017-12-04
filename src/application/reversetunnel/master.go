package reversetunnel

import (
	"logger"
	"net"
	"net/http"
	"sync"
	"time"
	"web"
)

const (
	_COMMAND_IN    = 0x00
	_COMMAND_OUT   = 0x01
	_COMMAND_ERROR = 0x02
)

var (
	_ErrorCommandEvent = &commandEvent{
		typ:  _COMMAND_ERROR,
		cmd:  CMD_V1_UNKNOWN,
		data: nil,
	}
)

type commandEvent struct {
	typ  byte
	cmd  byte
	data []byte
}

type slaverServer struct {
	net.Conn
	ch chan *commandEvent
}

func (s *slaverServer) serve() {
	go func() {
		for {
			s.SetReadDeadline(time.Now().Add(10 * time.Second))

			if cmd, err := parseCommandV1(s); err != nil {
				logger.Error(err)
				s.ch <- _ErrorCommandEvent
				break
			} else if cmd == CMD_V1_HEARTBEAT {
				continue
			} else {
				s.ch <- _ErrorCommandEvent
				break
			}
		}
	}()

	for {
		select {
		case e := <-s.ch:
			if e.typ == _COMMAND_ERROR {
				break
			} else if e.typ == _COMMAND_OUT {
				s.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, err := s.Write(e.data); err != nil {
					logger.Error(err)
					break
				}
				s.SetWriteDeadline(time.Time{})
			}
		}
	}
	s.Close()
}

type masterServer struct {
	client      *web.BasicServer
	ctrl        net.Listener
	tunnel      net.Listener
	slavers     map[string]*slaverServer
	slaversLock sync.Mutex
	ch          chan int
}

func dispath(path, method string) (f web.ServeFunc, err error) {
	return func(w http.ResponseWriter, r *http.Request) int {
		w.Write([]byte("server running"))
		return http.StatusOK
	}, nil
}

func newMasterServer(conf *config) *masterServer {
	client := web.NewBasicServer(dispath, conf.ClientAddr)

	ctrlListener, err := net.Listen("tcp", conf.CtrlAddr)
	if err != nil {
		panic(err)
	}

	tunnelListener, err := net.Listen("tcp", conf.TunnelAddr)
	if err != nil {
		panic(err)
	}

	return &masterServer{
		client:  client,
		ctrl:    ctrlListener,
		tunnel:  tunnelListener,
		slavers: map[string]*slaverServer{},
	}
}

func (m *masterServer) run() {
	go m.client.Run()
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
	conn.SetDeadline(time.Now().Add(10 * time.Second))
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

		s := &slaverServer{
			Conn: conn,
			ch:   make(chan *commandEvent),
		}
		m.slaversLock.Lock()
		m.slavers[name] = s
		m.slaversLock.Unlock()

		go s.serve()
	}
	return
}
