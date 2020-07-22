package util

import (
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func CreateTun(name string) error {
	if err := maybeCreateDevNetTun(); err != nil {
		return fmt.Errorf("error creating /dev/net/tun: %v", err)
	}

	if ok, err := tunExists(name); ok || err != nil {
		return err
	}

	la := netlink.NewLinkAttrs()
	la.Name = name

	link := &netlink.Tuntap{
		LinkAttrs: la,
		Mode:      netlink.TUNTAP_MODE_TUN,
		Flags:     netlink.TUNTAP_DEFAULTS,
	}

	err := netlink.LinkAdd(link)
	if err != nil {
		return fmt.Errorf("error creating tun %q: %v", name, err)
	}

	return nil
}

func tunExists(name string) (bool, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false, nil
	}

	tuntap, isTuntap := link.(*netlink.Tuntap)
	if isTuntap && tuntap.Mode&netlink.TUNTAP_MODE_TUN != 0 {
		return true, nil
	} else {
		return false, fmt.Errorf("device %q exists, but isn't a tun: %+v", name, link)
	}
}

func maybeCreateDevNetTun() error {
	if err := os.Mkdir("/dev/net", os.FileMode(0755)); !os.IsExist(err) && err != nil {
		return err
	}
	tunMode := uint32(020666)
	if err := unix.Mknod("/dev/net/tun", tunMode, int(unix.Mkdev(10, 200))); !os.IsExist(err) && err != nil {
		return err
	}
	return nil
}
