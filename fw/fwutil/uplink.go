package fwutil

import (
	"net"

	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/vaddr"
	"go.jonnrb.io/egress/vaddr/dhcp"
	"go.jonnrb.io/egress/vaddr/vaddrutil"
)

// Implementing this interface implies a virtual IP will be used. To specify a
// static virtual IP, implement ConfigUplinkAddr. If this interface is
// implemented and ConfigUplinkAddr is not implemented, a virtual IP will be
// requested using DHCP on the uplink.
type ConfigUplinkHWAddr interface {
	// The MAC address to use for the uplink interface. This is virtually
	// assigned to facilitate HA failover with picky (*cough* *cough* FiOS
	// *cough*) network hardware.
	UplinkHWAddr() net.HardwareAddr
}

// If this interface isn't implemented by a fw.Config and ConfigUplinkHWAddr is,
// it's assumed DHCP will be used.
type ConfigUplinkAddr interface {
	// The IP+net of the uplink interface for connection.
	UplinkAddr() fw.Addr
}

// Allows specifying the dhcp.LeaseStore when DHCP is used. DHCP is used when
// ConfigUplinkHWAddr is implemented and ConfigUplinkAddr is not.
type ConfigUplinkLeaseStore interface {
	// When ConfigUplinkAddr isn't implemented, the dhcp.LeaseStore to use when
	// using DHCP. This can be nil and DHCP will still work; it just won't save
	// leases.
	UplinkLeaseStore() dhcp.LeaseStore
}

func MakeVAddrUplink(c fw.Config) vaddr.Suite {
	var w []vaddr.Wrapper
	var a []vaddr.Active
	w = append(w, contributeUplinkUp(c)...)
	w = append(w, contributeUplinkVirtualMAC(c)...)
	w = append(w, contributeUplinkIP(c)...)
	w = append(w, contributeUplinkGratuitousARP(c)...)
	a = append(a, contributeUplinkDHCP(c)...)
	return vaddr.Suite{Wrappers: w}
}

func contributeUplinkUp(c fw.Config) []vaddr.Wrapper {
	return []vaddr.Wrapper{&vaddrutil.Up{Link: c.Uplink()}}
}

func contributeUplinkVirtualMAC(c fw.Config) (w []vaddr.Wrapper) {
	if i, ok := c.(ConfigUplinkHWAddr); ok {
		w = append(w,
			&vaddrutil.VirtualMAC{
				Link: c.Uplink(),
				Addr: i.UplinkHWAddr(),
			})
	}
	return
}

func contributeUplinkIP(c fw.Config) (w []vaddr.Wrapper) {
	if i, ok := c.(ConfigUplinkAddr); ok {
		w = append(w,
			&vaddrutil.IP{
				Link: c.Uplink(),
				Addr: i.UplinkAddr(),
			})
	}
	return
}

func contributeUplinkGratuitousARP(c fw.Config) (w []vaddr.Wrapper) {
	if i, ok := c.(ConfigUplinkHWAddr); ok {
		if j, ok := c.(ConfigUplinkAddr); ok {
			w = append(w,
				&vaddrutil.GratuitousARP{
					IP:     j.UplinkAddr().IP,
					HWAddr: i.UplinkHWAddr(),
					Link:   c.Uplink(),
				})
		}
	}
	return
}

func contributeUplinkDHCP(c fw.Config) (a []vaddr.Active) {
	if i, ok := c.(ConfigUplinkHWAddr); ok {
		if _, ok := c.(ConfigUplinkAddr); ok {
			return
		}
		var ls dhcp.LeaseStore
		if j, ok := c.(ConfigUplinkLeaseStore); ok {
			ls = j.UplinkLeaseStore()
		}
		a = append(a,
			&dhcp.VAddr{
				HWAddr:     i.UplinkHWAddr(),
				Link:       c.Uplink(),
				LeaseStore: ls,
			})
	}
	return
}
