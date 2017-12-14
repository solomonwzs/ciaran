package socks5

import (
	"io"
	"logger"
	"net"
	"strconv"
)

var (
	_REPLY_NO_AUTH   = []byte{PROTO_VER, PROTO_METHOD_NOAUTH}
	_REPLY_NO_ACCEPT = []byte{PROTO_VER, PROTO_METHOD_NOT_ACCEPTABLE}

	_REPLY_GEN_SOCKS_SERVER_FAILURE = []byte{
		PROTO_VER,
		REP_GEN_SOCKS_SERVER_FAILURE,
		0x00,
		ATYP_IPV4,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
	}
	_REPLY_GEN_SOCKS_SERVER_SUCCESS = []byte{
		PROTO_VER,
		REP_SUCCESS,
		0x00,
		ATYP_IPV4,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
	}
)

type TCPHandler struct {
	conn   net.Conn
	server net.Conn
}

func NewTCPHandler(conn net.Conn) *TCPHandler {
	handler := TCPHandler{
		conn:   conn,
		server: nil,
	}
	return &handler
}

func (h *TCPHandler) Run() {
	if err := h.stageMethodNegotiation(); err != nil {
		logger.Error(err)
		return
	}

	if err := h.stageAddr(); err != nil {
		logger.Error(err)
		return
	}

	h.stageTransport()
}

func (h *TCPHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
	if h.server != nil {
		h.server.Close()
	}
}

func (h *TCPHandler) stageMethodNegotiation() (err error) {
	buf := make([]byte, 0xff, 0xff)
	if _, err = io.ReadFull(h.conn, buf[:2]); err != nil {
		return
	}

	ver, nMethods := buf[0], buf[1]
	if ver != PROTO_VER {
		return ErrVersion
	}

	if _, err = io.ReadFull(h.conn, buf[:nMethods]); err != nil {
		return
	}
	for i := byte(0); i < nMethods; i++ {
		if buf[i] == PROTO_METHOD_NOAUTH {
			_, err = h.conn.Write(_REPLY_NO_AUTH)
			return
		}
	}

	if _, err = h.conn.Write(_REPLY_NO_ACCEPT); err != nil {
		return
	} else {
		return ErrMethodNotAcceptable
	}
}

func (h *TCPHandler) stageAddr() (err error) {
	var (
		buf  = make([]byte, 0xff, 0xff)
		host string
		port string
	)

	if _, err = io.ReadFull(h.conn, buf[:4]); err != nil {
		return
	}

	ver, cmd, atyp := buf[0], buf[1], buf[3]
	if ver != PROTO_VER {
		return ErrVersion
	}

	if cmd == CMD_CONNECT {
		switch atyp {
		case ATYP_IPV4:
			if _, err = io.ReadFull(h.conn, buf[:4]); err != nil {
				return
			}
			host = net.IP(buf[:4]).String()
		case ATYP_DOMAINNAME:
			if _, err = io.ReadFull(h.conn, buf[:1]); err != nil {
				return
			}
			domainLen := buf[0]

			if _, err = io.ReadFull(h.conn, buf[:domainLen]); err != nil {
				return
			}
			host = string(buf[:domainLen])
		case ATYP_IPV6:
			if _, err = io.ReadFull(h.conn, buf[:16]); err != nil {
				return
			}
			host = net.IP(buf[:16]).String()
		default:
			return ErrUnknownAddrType
		}

		if _, err = io.ReadFull(h.conn, buf[:2]); err != nil {
			return
		}
		port = strconv.Itoa(int(buf[0])<<8 | int(buf[1]))

		if h.server, err = net.Dial(
			"tcp", net.JoinHostPort(host, port)); err != nil {
			h.conn.Write(_REPLY_GEN_SOCKS_SERVER_FAILURE)
			return
		}

		_, err = h.conn.Write(_REPLY_GEN_SOCKS_SERVER_SUCCESS)
		return
	} else {
		return ErrUnknownCommand
	}
}

func (handler *TCPHandler) stageTransport() {
	go io.Copy(handler.server, handler.conn)
	io.Copy(handler.conn, handler.server)
}
