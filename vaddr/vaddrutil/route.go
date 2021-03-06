package vaddrutil

import (
	"fmt"
	"net"
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
	route, err := r.route()
	if err != nil {
		return err
	}
	err = netlink.RouteAdd(route)
	// EEXIST is ok.
	if errno, ok := err.(syscall.Errno); ok && errno == unix.EEXIST {
		err = netlink.RouteReplace(route)
	}
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: could not add/replace route %+v: %w", route, err)
	}
	return nil
}

func (r *DefaultRoute) Stop() error {
	route, err := r.route()
	if err != nil {
		return err
	}
	err = netlink.RouteDel(route)
	if errno, ok := err.(syscall.Errno); ok && errno == unix.ESRCH {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: failed to delete route %+v: %w", route, err)
	}
	return nil
}

func (r *DefaultRoute) route() (*netlink.Route, error) {
	l, err := netlink.LinkByName(r.Link.Name())
	if err != nil {
		return nil, fmt.Errorf(
			"vaddrutil: failed to get link %q: %w", r.Link.Name(), err)
	}
	gw, err := netlink.ParseAddr(r.GW.String())
	if err != nil {
		panic(fmt.Sprintf(
			"vaddrutil: bad conversion of fw.Addr to netlink.Addr: %v", err))
	}
	route := &netlink.Route{
		LinkIndex: l.Attrs().Index,
		Gw:        gw.IP,
		Dst: &net.IPNet{
			IP:   net.ParseIP("0.0.0.0"),
			Mask: net.CIDRMask(32, 0),
		},
	}
	return route, nil
}
