package fw

import "go.jonnrb.io/egress/fw/rules"

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
	Subnet string
}

// A connected network interface.
type Link interface {
	Name() string
}
