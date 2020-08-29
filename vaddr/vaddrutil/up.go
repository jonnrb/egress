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
		return fmt.Errorf("failed to get link %q: %w", a.Link.Name(), err)
	}
	return netlink.LinkSetUp(l)
}

func (a *Up) Stop() error {
	l, err := netlink.LinkByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", a.Link.Name(), err)
	}
	return netlink.LinkSetDown(l)
}
