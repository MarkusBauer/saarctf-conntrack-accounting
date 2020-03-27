package main

import "strconv"

const PROTO_ICMP = 1
const PROTO_TCP = 6
const PROTO_DCCP = 33
const PROTO_SCTP = 132

const (
	TCP_CONNTRACK_NONE        = 0
	TCP_CONNTRACK_SYN_SENT    = 1
	TCP_CONNTRACK_SYN_RECV    = 2
	TCP_CONNTRACK_ESTABLISHED = 3
	TCP_CONNTRACK_FIN_WAIT    = 4
	TCP_CONNTRACK_CLOSE_WAIT  = 5
	TCP_CONNTRACK_LAST_ACK    = 6
	TCP_CONNTRACK_TIME_WAIT   = 7
	TCP_CONNTRACK_CLOSE       = 8
	TCP_CONNTRACK_LISTEN      = 9
	TCP_CONNTRACK_MAX         = 10
	TCP_CONNTRACK_IGNORE      = 11
	TCP_CONNTRACK_RETRANS     = 12
	TCP_CONNTRACK_UNACK       = 13
	TCP_CONNTRACK_TIMEOUT_MAX = 14
)

// ProtoLookup translates a protocol integer into its string representation.
func ProtoLookup(p uint8) string {
	protos := map[uint8]string{
		1:   "icmp",
		2:   "igmp",
		6:   "tcp",
		17:  "udp",
		33:  "dccp",
		47:  "gre",
		58:  "ipv6-icmp",
		94:  "ipip",
		115: "l2tp",
		132: "sctp",
		136: "udplite",
	}

	if val, ok := protos[p]; ok {
		return val
	}

	return strconv.FormatUint(uint64(p), 10)
}
