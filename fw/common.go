package fw

import (
	"fmt"

	"go.jonnrb.io/egress/fw/rules"
)

// Allows traffic to be forwarded from in to out. Note that this doesn't affect
// the routing rules at all.
func Forward(in, out Link) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t filter -A fw-interfaces -j ACCEPT -i %v -o %v",
		in.Name(), out.Name()))
}

// Allows traffic to be forwarded from in to out when directed to a specific
// subnet. Note that this doesn't affect the routing rules at all.
func ForwardToSubnet(in, out Link, dst string) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t filter -A fw-interfaces -j ACCEPT -d %v -i %v -o %v",
		dst, in.Name(), out.Name()))
}

// Masquerades traffic forwarded to out.
func Masquerade(out Link) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t nat -A POSTROUTING -j MASQUERADE -o %v",
		out.Name()))
}

// Allows either tcp or udp input traffic to a specific port.
func OpenPort(proto, port string) rules.Rule {
	switch proto {
	case "tcp":
	case "udp":
	default:
		panic(fmt.Sprintf("invalid proto: %q", proto))
	}

	return rules.Rule(fmt.Sprintf(
		"-I in-%s -j ACCEPT -p %s --dport %s",
		proto, proto, port))
}
