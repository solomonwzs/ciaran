package main

import (
	"logger"
	"net"
	"relay"
)

func main() {
	logger.Init()
	logger.AddLogger("default", nil)

	l, err := net.Listen("tcp", ":18081")
	if err != nil {
		logger.Log(logger.ERROR, err)
		return
	}

	for {
		client, err := l.Accept()
		if err != nil {
			logger.Log(logger.ERROR, err)
			return
		}

		go func() {
			var handler *relay.TCPHandler = nil

			defer func() {
				if err := recover(); err != nil {
					logger.Log(logger.ERROR, err)
				}

				if handler != nil {
					handler.Close()
				}
			}()

			handler = relay.NewTCPHandler(client)
			if handler == nil {
				return
			}

			handler.Run()
		}()
	}
}