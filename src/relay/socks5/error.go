package socks5

import "errors"

var (
	ErrVersion             = errors.New("socks5: version error")
	ErrMethodNotAcceptable = errors.New("socks5: methods not acceptable")
	ErrUnknownAddrType     = errors.New("socks5: unknown address type")
	ErrUnknownCommand      = errors.New("socks5: unknown command")
)
