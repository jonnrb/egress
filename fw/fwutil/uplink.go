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
	UplinkAddr() (a fw.Addr, ok bool)
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
	i, ok := c.(ConfigUplinkHWAddr)
	if !ok {
		return
	}
	hwAddr := i.UplinkHWAddr()
	if hwAddr == nil {
		return
	}
	w = append(w,
		&vaddrutil.VirtualMAC{
			Link: c.Uplink(),
			Addr: hwAddr,
		})
	return
}

func contributeUplinkIP(c fw.Config) (w []vaddr.Wrapper) {
	i, ok := c.(ConfigUplinkAddr)
	if !ok {
		return
	}
	var a fw.Addr
	if a, ok = i.UplinkAddr(); !ok {
		return
	}
	w = append(w,
		&vaddrutil.IP{
			Link: c.Uplink(),
			Addr: a,
		})
	return
}

func contributeUplinkGratuitousARP(c fw.Config) (w []vaddr.Wrapper) {
	i, ok := c.(ConfigUplinkHWAddr)
	if !ok {
		return
	}
	hwAddr := i.UplinkHWAddr()
	if hwAddr == nil {
		return
	}
	j, ok := c.(ConfigUplinkAddr)
	if !ok {
		return
	}
	var a fw.Addr
	if a, ok = j.UplinkAddr(); !ok {
		return
	}
	w = append(w,
		&vaddrutil.GratuitousARP{
			IP:     a.IP,
			HWAddr: hwAddr,
			Link:   c.Uplink(),
		})
	return
}

func contributeUplinkDHCP(c fw.Config) (a []vaddr.Active) {
	i, ok := c.(ConfigUplinkHWAddr)
	if !ok {
		return
	}
	hwAddr := i.UplinkHWAddr()
	if hwAddr == nil {
		return
	}
	if j, ok := c.(ConfigUplinkAddr); ok {
		if _, ok := j.UplinkAddr(); ok {
			return
		}
	}
	var ls dhcp.LeaseStore
	if j, ok := c.(ConfigUplinkLeaseStore); ok {
		ls = j.UplinkLeaseStore()
	}
	a = append(a,
		&dhcp.VAddr{
			HWAddr:     hwAddr,
			Link:       c.Uplink(),
			LeaseStore: ls,
		})
	return
}
