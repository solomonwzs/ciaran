package socks5proxy

import (
	"fmt"
	"net"
	"relay/socks5"

	"github.com/solomonwzs/goxutil/logger"
)

func Main() {
	logger.NewLogger(func(r *logger.Record) {
		fmt.Printf("%s", r)
	})

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
