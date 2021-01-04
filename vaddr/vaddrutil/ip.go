package vaddrutil

import (
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"golang.org/x/sys/unix"
)

type IP struct {
	Link fw.Link
	Addr fw.Addr
}

func (ip *IP) Start() error {
	l, err := netlink.LinkByName(ip.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", ip.Link.Name(), err)
	}
	a, err := netlink.ParseAddr(ip.Addr.String())
	if err != nil {
		panic(fmt.Sprintf(
			"vaddrutil: bad conversion of fw.Addr to netlink.Addr: %v", err))
	}
	err = netlink.AddrAdd(l, a)
	// EEXIST is ok.
	if errno, ok := err.(syscall.Errno); ok && errno == unix.EEXIST {
		return nil
	}
	return err
}

func (ip *IP) Stop() error {
	l, err := netlink.LinkByName(ip.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", ip.Link.Name(), err)
	}
	a, err := netlink.ParseAddr(ip.Addr.String())
	if err != nil {
		panic(fmt.Sprintf(
			"vaddrutil: bad conversion of fw.Addr to netlink.Addr: %v", err))
	}
	return netlink.AddrDel(l, a)
}
