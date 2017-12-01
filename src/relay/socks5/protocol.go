package socks5

// The client connects to the server, and sends a version
// identifier/method selection message:

//                 +----+----------+----------+
//                 |VER | NMETHODS | METHODS  |
//                 +----+----------+----------+
//                 | 1  |    1     | 1 to 255 |
//                 +----+----------+----------+

// The VER field is set to X'05' for this version of the protocol.  The
// NMETHODS field contains the number of method identifier octets that
// appear in the METHODS field.

// The server selects from one of the methods given in METHODS, and
// sends a METHOD selection message:

//                       +----+--------+
//                       |VER | METHOD |
//                       +----+--------+
//                       | 1  |   1    |
//                       +----+--------+

// If the selected METHOD is X'FF', none of the methods listed by the
// client are acceptable, and the client MUST close the connection.

// The values currently defined for METHOD are:

//        o  X'00' NO AUTHENTICATION REQUIRED
//        o  X'01' GSSAPI
//        o  X'02' USERNAME/PASSWORD
//        o  X'03' to X'7F' IANA ASSIGNED
//        o  X'80' to X'FE' RESERVED FOR PRIVATE METHODS
//        o  X'FF' NO ACCEPTABLE METHODS

const (
	PROTO_VER = 0x05

	PROTO_METHOD_NOAUTH         = 0x00
	PROTO_METHOD_GSSAPI         = 0x01
	PROTO_METHOD_PASSWORD       = 0x02
	PROTO_METHOD_NOT_ACCEPTABLE = 0xff
)

// Once the method-dependent subnegotiation has completed, the client
// sends the request details.  If the negotiated method includes
// encapsulation for purposes of integrity checking and/or
// confidentiality, these requests MUST be encapsulated in the method-
// dependent encapsulation.

// The SOCKS request is formed as follows:

//      +----+-----+-------+------+----------+----------+
//      |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
//      +----+-----+-------+------+----------+----------+
//      | 1  |  1  | X'00' |  1   | Variable |    2     |
//      +----+-----+-------+------+----------+----------+

//   Where:

//        o  VER    protocol version: X'05'
//        o  CMD
//           o  CONNECT X'01'
//           o  BIND X'02'
//           o  UDP ASSOCIATE X'03'
//        o  RSV    RESERVED
//        o  ATYP   address type of following address
//           o  IP V4 address: X'01'
//           o  DOMAINNAME: X'03'
//           o  IP V6 address: X'04'
//        o  DST.ADDR       desired destination address
//        o  DST.PORT desired destination port in network octet
//           order

const (
	CMD_CONNECT       = 0x01
	CMD_BIND          = 0x02
	CMD_UDP_ASSOCIATE = 0x03

	ATYP_IPV4       = 0x01
	ATYP_DOMAINNAME = 0x03
	ATYP_IPV6       = 0x04
)

// The SOCKS request information is sent by the client as soon as it has
// established a connection to the SOCKS server, and completed the
// authentication negotiations.  The server evaluates the request, and
// returns a reply formed as follows:

//      +----+-----+-------+------+----------+----------+
//      |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
//      +----+-----+-------+------+----------+----------+
//      | 1  |  1  | X'00' |  1   | Variable |    2     |
//      +----+-----+-------+------+----------+----------+

//   Where:

//        o  VER    protocol version: X'05'
//        o  REP    Reply field:
//           o  X'00' succeeded
//           o  X'01' general SOCKS server failure
//           o  X'02' connection not allowed by ruleset
//           o  X'03' Network unreachable
//           o  X'04' Host unreachable
//           o  X'05' Connection refused
//           o  X'06' TTL expired
//           o  X'07' Command not supported
//           o  X'08' Address type not supported
//           o  X'09' to X'FF' unassigned
//        o  RSV    RESERVED
//        o  ATYP   address type of following address

const (
	REP_SUCCESS                  = 0x00
	REP_GEN_SOCKS_SERVER_FAILURE = 0x01
	REP_CONN_NOT_ALLOWED         = 0x02
	REP_NETWORK_UNREACHABLE      = 0x03
	REP_HOST_UNREACHABLE         = 0x04
	REP_CONN_REFUSED             = 0x05
	REP_TTL_EXPIRED              = 0X06
	REP_CMD_NOT_SUPPORTED        = 0x07
	REP_ADDR_TYPE_NOT_SUPPORTED  = 0x08
)

// A UDP-based client MUST send its datagrams to the UDP relay server at
// the UDP port indicated by BND.PORT in the reply to the UDP ASSOCIATE
// request.  If the selected authentication method provides
// encapsulation for the purposes of authenticity, integrity, and/or
// confidentiality, the datagram MUST be encapsulated using the
// appropriate encapsulation.  Each UDP datagram carries a UDP request
// header with it:

//    +----+------+------+----------+----------+----------+
//    |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
//    +----+------+------+----------+----------+----------+
//    | 2  |  1   |  1   | Variable |    2     | Variable |
//    +----+------+------+----------+----------+----------+

//   The fields in the UDP request header are:

//        o  RSV  Reserved X'0000'
//        o  FRAG    Current fragment number
//        o  ATYP    address type of following addresses:
//           o  IP V4 address: X'01'
//           o  DOMAINNAME: X'03'
//           o  IP V6 address: X'04'
//        o  DST.ADDR       desired destination address
//        o  DST.PORT       desired destination port
//        o  DATA     user data
