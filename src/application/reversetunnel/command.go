package reversetunnel

import (
	"encoding/binary"
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

func parseJoinAckV1(r io.Reader) (rep byte, err error) {
	buf := make([]byte, 1)

	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}
	return buf[0], nil
}

func parseBuildTunnelV1(r io.Reader) (
	mAddr, sAddr *address, tid uint64, err error) {
	buf := make([]byte, 8, 8)

	addrs := []*address{new(address), new(address)}
	for i := 0; i < 2; i++ {
		if _, err = io.ReadFull(r, buf[:1]); err != nil {
			return
		} else {
			if buf[0] == ATYP_IPV4 {
				addrs[i].ip = make([]byte, 4)
			} else if buf[0] == ATYP_IPV6 {
				addrs[i].ip = make([]byte, 16)
			} else {
				err = ErrCommand
				return
			}
			if _, err = io.ReadFull(r, addrs[i].ip); err != nil {
				return
			}
			if _, err = io.ReadFull(r, addrs[i].port[:]); err != nil {
				return
			}
			addrs[i].atype = buf[0]
		}
	}
	if err = binary.Read(r, binary.BigEndian, &tid); err != nil {
		return
	}

	return addrs[0], addrs[1], tid, nil
}

func parseBuildTunnelAckV1(r io.Reader) (
	name string, tid uint64, err error) {
	buf := make([]byte, 64, 64)
	if _, err = io.ReadFull(r, buf[:1]); err != nil {
		return
	} else if buf[0] == 0 || buf[0] > 64 {
		err = ErrCommand
		return
	}

	nameLen := buf[0]
	if _, err = io.ReadFull(r, buf[:nameLen]); err != nil {
		return
	}

	if err = binary.Read(r, binary.BigEndian, &tid); err != nil {
		return
	}

	if io.ReadFull(r, buf[:1]); err != nil {
		return
	} else if buf[0] != REP_SUCCEEDS {
		err = ErrReply
		return
	}

	return
}
