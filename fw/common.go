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
func ForwardToSubnet(in, out Link, dst Addr) rules.Rule {
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

// Allows either tcp or udp input traffic to a specific port from a specific
// interface.
func OpenPortOnInterface(proto string, port int, iface Link) rules.Rule {
	switch proto {
	case "tcp":
	case "udp":
	default:
		panic(fmt.Sprintf("invalid proto: %q", proto))
	}

	return rules.Rule(fmt.Sprintf(
		"-I in-%s -j ACCEPT -p %s --dport %d -i %s",
		proto, proto, port, iface.Name()))
}

// Allows either tcp or udp input traffic to a specific port.
func OpenPort(proto string, port int) rules.Rule {
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

// Blocks input (local connections) from a specific network interface. This is
// specific to L4/transport-layer (TCP/UDP currently, other protos may be added
// in the future) assuming things like ICMP shouldn't be blocked.
func BlockInputFromInterface(proto string, iface Link) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-I in-%s -j RETURN -i %s -p %s",
		proto, iface.Name, proto))
}
