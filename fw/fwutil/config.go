package fwutil

import (
	"net"

	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/vaddr/dhcp"
)

type VAddrConfigLAN interface {
	// Link connected to the network with local clients.
	LAN() fw.Link

	// The MAC address to use for the LAN. This is virtually assigned to
	// facilitate HA failover.
	LANHWAddr() net.HardwareAddr

	// The IP+net of the LAN interface expected by local clients.
	LANAddr() fw.Addr
}

type VAddrConfigUplink interface {
	// Link connected to a broader network (possibly the internet) that will
	// be used to masquerade outbound connections from a LAN.
	Uplink() fw.Link

	// The MAC address to use for the uplink interface. This is virtually
	// assigned to facilitate HA failover with picky (*cough* *cough* FiOS
	// *cough*) network hardware. This is required.
	UplinkHWAddr() net.HardwareAddr

	// The IP+net of the uplink interface for connection. This is optional and
	// will be preferred over DHCP.
	UplinkAddr() (a fw.Addr, ok bool)

	// When UplinkAddr() is not specified, the dhcp.LeaseStore to use when using
	// DHCP. This can be nil and DHCP will still work; it just won't save
	// leases.
	UplinkLeaseStore() dhcp.LeaseStore
}
