package fw

import (
	"fmt"
	"net"
	"strings"

	"go.jonnrb.io/egress/fw/rules"
)

type Config interface {
	// Link connected to the network with local clients.
	LAN() Link

	// Link connected to a broader network (possibly the internet) that will
	// be used to masquerade outbound connections from LAN().
	Uplink() Link

	// Other networks that can be routed to from LAN without masquerading. The
	// static route will not be established in the reverse direction.
	FlatNetworks() []StaticRoute

	ExtraRules() rules.RuleSet
}

// Union of a subnet specified in CIDR and the Link it can be reached on.
type StaticRoute struct {
	Link   Link
	Subnet Addr
}

// A connected network interface.
type Link interface {
	Name() string
}

type LinkString string

func (l LinkString) Name() string {
	return string(l)
}

// An IP address and CIDR mask.
type Addr struct {
	IP   net.IP
	Mask net.IPMask
}

func ParseAddr(s string) (a Addr, err error) {
	// Just an IP implies a /32.
	if !strings.Contains(s, "/") {
		s += "/32"
	}
	ip, net, err := net.ParseCIDR(s)
	if err != nil {
		return
	}
	a.IP = ip
	a.Mask = net.Mask
	return
}

func (a Addr) String() string {
	ones, _ := a.Mask.Size()
	return fmt.Sprintf("%s/%d", a.IP, ones)
}
