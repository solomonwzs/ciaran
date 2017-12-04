package reversetunnel

import "errors"

var (
	ErrDupicateSlaverName = errors.New("master: duplicate slaver name")
	ErrCommand            = errors.New("master: command error")
)
