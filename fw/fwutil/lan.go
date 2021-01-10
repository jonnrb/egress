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
	LANAddr() fw.Addr
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
	if i, ok := c.(ConfigLANHWAddr); ok {
		w = append(w,
			&vaddrutil.VirtualMAC{
				Link: c.LAN(),
				Addr: i.LANHWAddr(),
			})
	}
	return
}

func contributeLANIP(c fw.Config) (w []vaddr.Wrapper) {
	if i, ok := c.(ConfigLANAddr); ok {
		w = append(w,
			&vaddrutil.IP{
				Link: c.LAN(),
				Addr: i.LANAddr(),
			})
	}
	return
}

func contributeLANGratuitousARP(c fw.Config) (w []vaddr.Wrapper) {
	if i, ok := c.(ConfigLANHWAddr); ok {
		if j, ok := c.(ConfigLANAddr); ok {
			w = append(w,
				&vaddrutil.GratuitousARP{
					IP:     j.LANAddr().IP,
					HWAddr: i.LANHWAddr(),
					Link:   c.LAN(),
				})
		}
	}
	return
}
