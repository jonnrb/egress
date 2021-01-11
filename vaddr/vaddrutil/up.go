package vaddrutil

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
)

type Up struct {
	Link fw.Link
}

func (a *Up) Start() error {
	l, err := netlink.LinkByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: failed to get link %q: %w", a.Link.Name(), err)
	}
	err = netlink.LinkSetUp(l)
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: failed to up link %q: %w", a.Link.Name(), err)
	}
	return nil
}

func (a *Up) Stop() error {
	l, err := netlink.LinkByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: failed to get link %q: %w", a.Link.Name(), err)
	}
	err = netlink.LinkSetDown(l)
	if err != nil {
		return fmt.Errorf(
			"vaddrutil: failed to down link %q: %w", a.Link.Name(), err)
	}
	return nil
}
