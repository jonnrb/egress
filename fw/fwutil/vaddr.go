package fwutil

import (
	"go.jonnrb.io/egress/vaddr"
	"go.jonnrb.io/egress/vaddr/dhcp"
	"go.jonnrb.io/egress/vaddr/vaddrutil"
)

func MakeVAddrLAN(c VAddrConfigLAN) vaddr.Active {
	return vaddr.Suite{
		Wrappers: []vaddr.Wrapper{
			&vaddrutil.Up{Link: c.LAN()},
			&vaddrutil.VirtualMAC{Link: c.LAN(), Addr: c.LANHWAddr()},
			&vaddrutil.IP{Link: c.LAN(), Addr: c.LANAddr()},
			&vaddrutil.GratuitousARP{
				IP:     c.LANAddr().IP,
				HWAddr: c.LANHWAddr(),
				Link:   c.LAN(),
			},
		},
	}
}

func MakeVAddrUplink(c VAddrConfigUplink) vaddr.Active {
	a, ok := c.UplinkAddr()
	if ok {
		return vaddr.Suite{
			Wrappers: []vaddr.Wrapper{
				&vaddrutil.Up{Link: c.Uplink()},
				&vaddrutil.VirtualMAC{Link: c.Uplink(), Addr: c.UplinkHWAddr()},
				&vaddrutil.IP{Link: c.Uplink(), Addr: a},
				&vaddrutil.GratuitousARP{
					IP:     a.IP,
					HWAddr: c.UplinkHWAddr(),
					Link:   c.Uplink(),
				},
			},
		}
	} else {
		return vaddr.Suite{
			Wrappers: []vaddr.Wrapper{&vaddrutil.Up{Link: c.Uplink()}},
			Actives: []vaddr.Active{
				dhcp.VAddr{
					HWAddr:     c.UplinkHWAddr(),
					Link:       c.Uplink(),
					LeaseStore: c.UplinkLeaseStore(),
				},
			},
		}
	}
}
