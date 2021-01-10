package fwutil

import (
	"net"

	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/vaddr"
	"go.jonnrb.io/egress/vaddr/vaddrutil"
)

type ConfigLANHWAddr interface {
	// The MAC address to use for the LAN. This is virtually assigned to
	// facilitate HA failover.
	LANHWAddr() net.HardwareAddr
}

type ConfigLANAddr interface {
	// The IP+net of the LAN interface expected by local clients.
	LANAddr() (a fw.Addr, ok bool)
}

func MakeVAddrLAN(c fw.Config) vaddr.Suite {
	var w []vaddr.Wrapper
	w = append(w, contributeLANUp(c)...)
	w = append(w, contributeLANVirtualMAC(c)...)
	w = append(w, contributeLANIP(c)...)
	w = append(w, contributeLANGratuitousARP(c)...)
	return vaddr.Suite{Wrappers: w}
}

func contributeLANUp(c fw.Config) []vaddr.Wrapper {
	return []vaddr.Wrapper{&vaddrutil.Up{Link: c.LAN()}}
}

func contributeLANVirtualMAC(c fw.Config) (w []vaddr.Wrapper) {
	i, ok := c.(ConfigLANHWAddr)
	if !ok {
		return
	}
	hwAddr := i.LANHWAddr()
	if hwAddr == nil {
		return
	}
	w = append(w,
		&vaddrutil.VirtualMAC{
			Link: c.LAN(),
			Addr: hwAddr,
		})
	return
}

func contributeLANIP(c fw.Config) (w []vaddr.Wrapper) {
	i, ok := c.(ConfigLANAddr)
	if !ok {
		return
	}
	var a fw.Addr
	if a, ok = i.LANAddr(); !ok {
		return
	}
	w = append(w,
		&vaddrutil.IP{
			Link: c.LAN(),
			Addr: a,
		})
	return
}

func contributeLANGratuitousARP(c fw.Config) (w []vaddr.Wrapper) {
	i, ok := c.(ConfigLANHWAddr)
	if !ok {
		return
	}
	hwAddr := i.LANHWAddr()
	if hwAddr == nil {
		return
	}
	j, ok := c.(ConfigLANAddr)
	if !ok {
		return
	}
	var a fw.Addr
	if a, ok = j.LANAddr(); !ok {
		return
	}
	w = append(w,
		&vaddrutil.GratuitousARP{
			IP:     a.IP,
			HWAddr: hwAddr,
			Link:   c.LAN(),
		})
	return
}
