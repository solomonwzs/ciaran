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

	_HEARTBEAT_DURATION = 2 * time.Second
	_NETWORK_TIMEOUT    = 4 * time.Second

	_TID      = uint32(time.Now().Unix())
	_TID_LOCK = sync.Mutex{}
)

func newTid() uint32 {
	_TID_LOCK.Lock()
	defer _TID_LOCK.Unlock()

	_TID += 1
	return _TID
}

type commandEvent struct {
	typ  byte
	cmd  byte
	data []byte
}

type mProxyTunnelConn struct {
	mConn net.Conn
	sConn net.Conn
	id    uint32
}

type mProxyTunnel struct {
	clientListener net.Listener
	ch             chan *mProxyTunnelConn
	sAddr          string
}

func (pt *mProxyTunnel) serv() {
	for {
		if conn, err := pt.clientListener.Accept(); err != nil {
			return
		} else {
			mc := &mProxyTunnelConn{
				mConn: conn,
			}
			pt.ch <- mc
		}
	}
}

func (pt *mProxyTunnel) close() {
	pt.clientListener.Close()
}

type slaverAgent struct {
	ctrl        net.Conn
	ch          chan *commandEvent
	pTunnelList []*mProxyTunnel

	pCh         chan *mProxyTunnelConn
	standByConn map[uint32]*mProxyTunnelConn
}

func (s *slaverAgent) recvHeartbeat() {
	for {
		s.ctrl.SetReadDeadline(time.Now().Add(_NETWORK_TIMEOUT))

		if cmd, err := parseCommandV1(s.ctrl); err != nil {
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
}

func (s *slaverAgent) serve() {
	go s.recvHeartbeat()
	for e := range s.ch {
		if e.typ == _COMMAND_ERROR {
			return
		} else if e.typ == _COMMAND_OUT {
			s.ctrl.SetWriteDeadline(time.Now().Add(_NETWORK_TIMEOUT))
			if _, err := s.ctrl.Write(e.data); err != nil {
				logger.Error(err)
				return
			}
			s.ctrl.SetWriteDeadline(time.Time{})
		}
	}
}

func (s *slaverAgent) close() {
	s.ctrl.Close()

	for _, pTunnel := range s.pTunnelList {
		pTunnel.close()
	}

	end := time.After(_NETWORK_TIMEOUT)
	for {
		select {
		case <-s.ch:
			break
		case <-end:
			return
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
	slaversLock sync.RWMutex

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
	m.ch = make(chan int, 10)

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

		s := &slaverAgent{
			ctrl:        conn,
			ch:          make(chan *commandEvent),
			pTunnelList: []*mProxyTunnel{},
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

	pTunnel := &mProxyTunnel{
		sAddr: req.SAddr,
	}
	if pTunnel.clientListener, err = net.Listen(
		"tcp", req.MAddr); err != nil {
		return
	}

	sa.pTunnelList = append(sa.pTunnelList, pTunnel)

	return
}
