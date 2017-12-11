package reversetunnel

// version 1

const PROTO_VER = 0x01

// Ctrl command

const (
	CMD_V1_JOIN             = 0x00 // slaver join master
	CMD_V1_JOIN_ACK         = 0x01 // master response of slaver join request
	CMD_V1_BUILD_TUNNEL     = 0x02 // master ask for build tunnel
	CMD_V1_BUILD_TUNNEL_ACK = 0x03 // slaver response of build tunnel request
	CMD_V1_HEARTBEAT        = 0x04 // check server alive
	CMD_V1_UNKNOWN          = 0xff
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
//   o X'01' duplicate slaver name error

const (
	REP_SUCCEEDS            = 0x00
	REP_ERR_DUP_SLAVER_NAME = 0x01
)

// After connected, slaver sends heartbeat to master,
// master ensure slaver alive by heartbeat

// +-----+-------+
// | VER |  CMD  |
// +-----+-------+
// |  1  | X'04' |
// +-----+-------+

// The master send command to slaver to build tunnel,
// The master listen the M.ADDR:M.PORT

// +-----+-------+--------+--------+--------+--------+--------+--------+------+
// | VER |  CMD  | M.ATYP | M.ADDR | M.PORT | S.ATYP | S.ADDR | S.PORT | T.ID |
// +-----+-------+--------+--------+--------+--------+--------+--------+------+
// |  1  | X'02' |    1   |   Var  |   2    |   1    |   Var  |   2    |  8   |
// +-----+-------+--------+--------+--------+--------+--------+--------+------+

// o ATYP address type of following address
//   o X'01' IP V4 address
//   o X'02' IP V6 address

const (
	ATYP_IPV4 = 0x01
	ATYP_IPV6 = 0x02
)

// The slaver send the reply to M.ADDR:M.PORT

// +-----+-------+----------+---------+------+-----+
// | VER |  CMD  | NAME.LEN |  NAME   | T.ID | REP |
// +-----+-------+----------+---------+------+-----+
// |  1  | X'03' |    2     | 1 to 64 |  8   |  1  |
// +-----+-------+----------+---------+------+-----+
