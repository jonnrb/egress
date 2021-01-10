package vaddrutil

import (
	"bytes"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
)

type VirtualMAC struct {
	Link fw.Link
	Addr net.HardwareAddr

	originalAddr net.HardwareAddr
}

// Applies the consistent MAC address to the device and saves the original MAC
// address.
func (a *VirtualMAC) Start() error {
	nl, err := netlink.LinkByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", a.Link.Name(), err)
	}

	a.originalAddr = nl.Attrs().HardwareAddr
	if bytes.Equal(a.originalAddr, a.Addr) {
		log.Warningf(
			"vaddr: link %q already had MAC address %s (setting it anyway...)",
			a.Link.Name(), a.Addr)
	}

	if err = netlink.LinkSetHardwareAddr(nl, a.Addr); err != nil {
		return fmt.Errorf(
			"failed to set link %q MAC address %s: %w",
			a.Link.Name(), a.Addr, err)
	}

	return nil
}

func (a *VirtualMAC) Stop() error {
	l, err := netlink.LinkByName(a.Link.Name())
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", a.Link.Name(), err)
	}

	if a.originalAddr != nil {
		err = netlink.LinkSetHardwareAddr(l, a.originalAddr)
	}
	if err != nil {
		return fmt.Errorf(
			"failed to set link %q MAC address %s: %w",
			a.Link.Name(), a.originalAddr, err)
	}

	return nil
}
