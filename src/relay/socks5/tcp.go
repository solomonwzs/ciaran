package socks5

import (
	"errors"
	"io"
	"logger"
	"net"
	"strconv"
)

const (
	CMD_CONNECT       = 0x01
	CMD_BIND          = 0x02
	CMD_UDP_ASSOCIATE = 0x03

	ATYPE_IPV4       = 0x01
	ATYPE_DOMAINNAME = 0x03
	ATYPE_IPV6       = 0x04
)

type TCPHandler struct {
	buffer []byte
	conn   net.Conn
	server net.Conn
}

func NewTCPHandler(conn net.Conn) *TCPHandler {
	handler := TCPHandler{
		buffer: make([]byte, 2048),
		conn:   conn,
		server: nil,
	}
	return &handler
}

func (handler *TCPHandler) Run() {
	err := handler.stageInit()
	if err != nil {
		logger.Error(err)
		return
	}

	err = handler.stageAddr()
	if err != nil {
		logger.Error(err)
		return
	}

	handler.stageTransport()
}

func (handler *TCPHandler) Close() {
	if handler.conn != nil {
		handler.conn.Close()
	}
	if handler.server != nil {
		handler.server.Close()
	}
}

func (handler *TCPHandler) stageInit() (err error) {
	buf := handler.buffer
	_, err = handler.conn.Read(buf[:])
	if err != nil {
		return
	}

	ver := buf[0]
	if ver != 0x05 {
		return errors.New("version error")
	}

	nMethods := int(buf[1])
	for i := 0; i < nMethods; i++ {
		if buf[i+2] == 0x00 {
			_, err = handler.conn.Write([]byte{0x05, 0x00})
			return
		}
	}

	handler.conn.Write([]byte{0x05, 0xff})
	return errors.New("identifier method error")
}

func (handler *TCPHandler) stageAddr() (err error) {
	var n int
	b := handler.buffer
	n, err = handler.conn.Read(b[:])
	if err != nil {
		return
	}

	ver := b[0]
	if ver != 0x05 {
		return errors.New("version error")
	}

	cmd := b[1]
	if cmd == CMD_CONNECT {
		var host, port string

		switch b[3] {
		case ATYPE_IPV4:
			host = net.IPv4(b[4], b[5], b[6], b[7]).String()
		case ATYPE_DOMAINNAME:
			host = string(b[5 : n-2])
		case ATYPE_IPV6:
			host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11],
				b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
		default:
			return errors.New("unknown atype")
		}
		port = strconv.Itoa(int(b[n-2])<<8 | int(b[n-1]))

		handler.server, err = net.Dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			return
		}

		_, err = handler.conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00})
		return
	} else {
		return errors.New("unknown command")
	}
}

func (handler *TCPHandler) stageTransport() {
	go io.Copy(handler.server, handler.conn)
	io.Copy(handler.conn, handler.server)
}
