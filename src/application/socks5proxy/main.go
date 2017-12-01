package socks5proxy

import (
	"logger"
	"net"
	"relay/socks5"
)

func Main() {
	logger.AddLogger("default", nil)

	l, err := net.Listen("tcp", ":18081")
	if err != nil {
		logger.Error(err)
		return
	}

	for {
		client, err := l.Accept()
		if err != nil {
			logger.Error(err)
			return
		}

		go func() {
			var handler *socks5.TCPHandler = nil

			defer func() {
				if err := recover(); err != nil {
					logger.Error(err)
				}

				if handler != nil {
					handler.Close()
				}
			}()

			handler = socks5.NewTCPHandler(client)
			if handler == nil {
				return
			}

			handler.Run()
		}()
	}
}
