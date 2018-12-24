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
