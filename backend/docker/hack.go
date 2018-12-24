package docker

import (
	"errors"
	"fmt"
	"strings"

	"docker.io/go-docker/api/types"
	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/log"
)

// for macvlan networks: adds the gateway ip to the lan interface
// for bridge networks: adds the "DefaultGatewayIPv4" aux-address to the lan interface
// throws an error in any other case because there is no non-hacky way to run a container as a gateway as of now
func dockerGatewayHacky(env environment, lan netlink.Link) error {
	networkJSON, err := env.cli.NetworkInspect(env.ctx, env.params.LANNetwork, types.NetworkInspectOptions{})
	if err != nil {
		return fmt.Errorf("error inspecting network %q: %v", env.params.LANNetwork, err)
	}

	if networkJSON.IPAM.Driver != "default" {
		return fmt.Errorf("found unsupported ipam driver %q", networkJSON.IPAM.Driver)
	}

	switch networkJSON.Driver {
	case "bridge":
		found := false

		for _, ipam := range networkJSON.IPAM.Config {
			if gw, ok := ipam.AuxAddress["DefaultGatewayIPv4"]; ok {
				found = true
				var mask int
				if a := strings.Split(ipam.Subnet, "/"); len(a) != 2 {
					return fmt.Errorf("error parsing subnet %q: wrong format %v", ipam.Subnet, a)
				} else if n, err := fmt.Sscanf(a[1], "%d", &mask); n != 1 {
					return fmt.Errorf("error parsing subnet %q: wrong format %q", ipam.Subnet, a[1])
				} else if err != nil {
					return fmt.Errorf("error parsing subnet %q: %v", ipam.Subnet, err)
				}
				s := fmt.Sprintf("%s/%d", gw, mask)
				if addr, err := netlink.ParseAddr(s); err != nil {
					return fmt.Errorf("error parsing address %q: %v", s, err)
				} else if err = netlink.AddrAdd(lan, addr); err != nil {
					return fmt.Errorf("error adding address %q to lan: %v", s, err)
				}
				log.V(2).Infof("added address %q to lan interface", s)
			}
		}

		if !found {
			return errors.New("did not find a suitable ipam on the bridge; DefaultGatewayIPv4 must be set as an aux-address")
		}
	case "macvlan":
		for _, ipam := range networkJSON.IPAM.Config {
			var mask int
			if a := strings.Split(ipam.Subnet, "/"); len(a) != 2 {
				return fmt.Errorf("error parsing subnet %q: wrong format %v", ipam.Subnet, a)
			} else if n, err := fmt.Sscanf(a[1], "%d", &mask); n != 1 {
				return fmt.Errorf("error parsing subnet %q: wrong format %q", ipam.Subnet, a[1])
			} else if err != nil {
				return fmt.Errorf("error parsing subnet %q: %v", ipam.Subnet, err)
			}
			s := fmt.Sprintf("%s/%d", ipam.Gateway, mask)
			if addr, err := netlink.ParseAddr(s); err != nil {
				return fmt.Errorf("error parsing address %q: %v", s, err)
			} else if err = netlink.AddrAdd(lan, addr); err != nil {
				return fmt.Errorf("error adding address %q to lan: %v", s, err)
			}
			log.V(2).Infof("added address %q to lan interface", s)
		}
	default:
		return fmt.Errorf("found unsupported lan network driver for gateway hack: %q", networkJSON.Driver)
	}

	return nil
}
