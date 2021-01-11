package vaddrutil

import (
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"golang.org/x/sys/unix"
)

type DefaultRoute struct {
	Link fw.Link
	GW   fw.Addr
}

func (r *DefaultRoute) Start() error {
	l, err := netlink.LinkByName(r.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", r.Link.Name(), err)
	}
	gw, err := netlink.ParseAddr(r.GW.String())
	if err != nil {
		panic(fmt.Sprintf(
			"vaddrutil: bad conversion of fw.Addr to netlink.Addr: %v", err))
	}
	route := &netlink.Route{
		LinkIndex: l.Attrs().Index,
		Dst:       gw.IPNet,
		Scope:     netlink.SCOPE_LINK,
	}
	err = netlink.RouteAdd(route)
	// EEXIST is ok.
	if errno, ok := err.(syscall.Errno); ok && errno == unix.EEXIST {
		err = netlink.RouteReplace(route)
	}
	return err
}

func (r *DefaultRoute) Stop() error {
	l, err := netlink.LinkByName(r.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", r.Link.Name(), err)
	}
	gw, err := netlink.ParseAddr(r.GW.String())
	if err != nil {
		panic(fmt.Sprintf(
			"vaddrutil: bad conversion of fw.Addr to netlink.Addr: %v", err))
	}
	return netlink.RouteDel(
		&netlink.Route{
			LinkIndex: l.Attrs().Index,
			Dst:       gw.IPNet,
			Scope:     netlink.SCOPE_LINK,
		})
}
