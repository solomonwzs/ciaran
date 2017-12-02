package reversetunnel

// version 1

const PROTO_VER = 0x01

// Ctrl command

const (
	CMD_V1_JOIN         = 0x00 // slaver join master
	CMD_V1_JOIN_ACK     = 0x01 // master response of slaver join request
	CMD_V1_BUILD_TUNNEL = 0x02 // master ask for build tunnel
)

// The slaver connects to master, and sends a join message:

// +-----+-------+----------+---------+
// | VER |  CMD  | NAME.LEN |  NAME   |
// +-----+-------+----------+---------+
// |  1  | X'00' |    2     | 1 to 64 |
// +-----+-------+----------+---------+

// The master response:

// +-----+-------+-----+
// | VER |  CMD  | REP |
// +-----+-------+-----+
// |  1  | X'01' |  1  |
// +-----+-------+-----+

// o REP
//   o X'00' succeeds
//   o X'01' failure

const (
	REP_SUCCEEDS = 0x00
	REP_FAILURE  = 0x01
)
