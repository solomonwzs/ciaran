package reversetunnel

import (
	"io"
)

func parseCommandV1(r io.Reader) (cmd byte, err error) {
	buf := make([]byte, 2, 2)
	if _, err = io.ReadFull(r, buf); err != nil {
		return CMD_V1_UNKNOWN, err
	}

	if buf[0] != PROTO_VER {
		return CMD_V1_UNKNOWN, ErrCommand
	}

	return buf[1], nil
}

func parseJoinV1(r io.Reader) (name string, err error) {
	buf := make([]byte, 64, 64)

	if _, err = io.ReadFull(r, buf[:1]); err != nil {
		return
	}
	if buf[0] == 0 || buf[0] > 64 {
		return "", ErrCommand
	}

	nameLen := buf[0]
	if _, err = io.ReadFull(r, buf[:nameLen]); err != nil {
		return
	}

	return string(buf[:nameLen]), nil
}