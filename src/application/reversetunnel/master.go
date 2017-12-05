package reversetunnel

import (
	"logger"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
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

type slaverAgent struct {
	net.Conn
	ch chan *commandEvent
}

type rProxyTunnel struct {
	rEnd net.Conn
	lEnd net.Conn
	id   uint64
}

func (s *slaverAgent) serve() {
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
				return
			} else if e.typ == _COMMAND_OUT {
				s.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, err := s.Write(e.data); err != nil {
					logger.Error(err)
					return
				}
				s.SetWriteDeadline(time.Time{})
			}
		}
	}
}

type masterServer struct {
	client net.Listener
	ctrl   net.Listener
	tunnel net.Listener

	tunnelHost  []byte
	tunnelPort  [2]byte
	tunnelAType byte

	slavers     map[string]*slaverAgent
	slaversLock sync.Mutex

	name string
	ch   chan int
}

func newMasterServer(conf *config) *masterServer {
	m := new(masterServer)

	host, port, err := net.SplitHostPort(conf.TunnelAddr)
	if err != nil {
		panic(err)
	}
	ip := net.ParseIP(host)
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			m.tunnelAType = ATYP_IPV4
			m.tunnelHost = ip[net.IPv6len-net.IPv4len:]
			break
		} else if host[i] == ':' {
			m.tunnelAType = ATYP_IPV6
			m.tunnelHost = ip
			break
		}
	}
	p, _ := strconv.Atoi(port)
	m.tunnelPort[0] = byte(p >> 8 & 0xff)
	m.tunnelPort[1] = byte(p & 0xff)

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
	m.ch = make(chan int)

	return m
}

func (m *masterServer) run() {
	go http.Serve(m.client, m)

	m.listenSlaverJoin()
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

		s := &slaverAgent{
			Conn: conn,
			ch:   make(chan *commandEvent),
		}
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

			s.Close()
		}()
	}
	return
}
