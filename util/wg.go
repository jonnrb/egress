package util

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func CreateWg(name string) error {
	if ok, err := wgExist(name); ok || err != nil {
		return err
	}

	la := netlink.NewLinkAttrs()
	la.Name = name

	link := &netlink.Wireguard{LinkAttrs: la}

	err := netlink.LinkAdd(link)
	if err != nil {
		return fmt.Errorf("error creating wireguard dev %q: %v", name, err)
	}

	return nil
}

func wgExist(name string) (bool, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false, nil
	}

	_, isWg := link.(*netlink.Wireguard)
	if isWg {
		return true, nil
	} else {
		return false, fmt.Errorf("device %q exists, but isn't a wireguard dev: %T %+v", name, link, link)
	}
}
