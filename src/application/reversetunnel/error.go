package reversetunnel

import "errors"

var (
	ErrDupicateSlaverName = errors.New("duplicate slaver name")
	ErrCommand            = errors.New("command error")
)
