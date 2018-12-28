package kubernetes

import (
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/internal"
	"golang.org/x/sys/unix"
)

func applyGWIP(lan netlink.Link, net *internal.NetworkDefinition) (err error) {
	for _, gw := range extractGWCIDRs(net) {
		var addr *netlink.Addr
		addr, err = netlink.ParseAddr(gw)
		if err != nil {
			return
		}

		err = netlink.AddrAdd(lan, addr)
		// EEXIST is ok.
		if errno, ok := err.(syscall.Errno); ok && errno == unix.EEXIST {
			err = nil
		}
		if err != nil {
			return
		}
	}
	return
}

func extractGWCIDRs(net *internal.NetworkDefinition) (gws []string) {
	set := make(map[string]struct{})
	for _, r := range net.Ranges {
		ip := r.Gateway.String()
		bits, _ := r.Subnet.Mask.Size()
		cidr := fmt.Sprintf("%s/%d", ip, bits)
		set[cidr] = struct{}{}
	}

	for gw := range set {
		gws = append(gws, gw)
	}
	return
}
