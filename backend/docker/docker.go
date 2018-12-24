package docker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/log"
)

type Params struct {
	LANNetwork      string
	FlatNetworks    []string
	UplinkNetwork   string
	UplinkInterface string
}

type Config struct {
	lan    netlink.Link
	uplink netlink.Link
	flat   []fw.StaticRoute
}

type link struct{ *netlink.LinkAttrs }

func (l link) Name() string {
	return l.LinkAttrs.Name
}

func (cfg *Config) LAN() fw.Link {
	return link{cfg.lan.Attrs()}
}

func (cfg *Config) Uplink() fw.Link {
	return link{cfg.uplink.Attrs()}
}

func (cfg *Config) FlatNetworks() []fw.StaticRoute {
	return cfg.flat
}

func (cfg *Config) ExtraRules() rules.RuleSet {
	return nil
}

func GetConfig(ctx context.Context, params Params) (*Config, error) {
	if params.LANNetwork == "" {
		return nil, errors.New("-docker.lan_network flag must be specified")
	}

	if params.UplinkNetwork == "" && params.UplinkInterface == "" {
		return nil, errors.New("-docker.uplink_network or -docker.uplink_interface must be specified")
	}

	cli, err := docker.NewEnvClient()
	if err != nil {
		return nil, fmt.Errorf("error connecting to docker: %v", err)
	}
	defer cli.Close()

	log.V(2).Info("connected to docker")

	containerID, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("error getting hostname: %v", err)
	}

	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect container using id %q: %v", containerID, err)
	}

	lanInterface, err := findInterfaceByDockerNetwork(params.LANNetwork, containerJSON)
	if err != nil {
		return nil, err
	}

	var sr []fw.StaticRoute
	for _, flatNetwork := range params.FlatNetworks {
		i, err := findInterfaceByDockerNetwork(flatNetwork, containerJSON)
		if err != nil {
			return nil, err
		}

		n, err := cli.NetworkInspect(ctx, flatNetwork, types.NetworkInspectOptions{})
		if err != nil {
			return nil, err
		}
		if len(n.IPAM.Config) != 1 {
			return nil, fmt.Errorf("expected 1 IPAM config; got: %v", n.IPAM.Config)
		}
		subnet := n.IPAM.Config[0].Subnet

		sr = append(sr, fw.StaticRoute{
			Link:   link{i.Attrs()},
			Subnet: subnet,
		})
	}

	var uplinkInterface netlink.Link
	if params.UplinkInterface != "" {
		uplinkInterface, err = netlink.LinkByName(params.UplinkInterface)
		if err != nil {
			return nil, fmt.Errorf("could not get interface %q: %v", params.UplinkInterface, err)
		}
	} else {
		uplinkInterface, err = findInterfaceByDockerNetwork(params.UplinkNetwork, containerJSON)
		if err != nil {
			return nil, fmt.Errorf("could not get interface for container network %q: %v", params.UplinkNetwork, err)
		}
	}

	log.V(2).Info("applying gateway hack")
	if err := dockerGatewayHacky(ctx, params, lanInterface, cli); err != nil {
		return nil, err
	}

	return &Config{
		lan:    lanInterface,
		uplink: uplinkInterface,
		flat:   sr,
	}, nil
}

// for macvlan networks: adds the gateway ip to the lan interface
// for bridge networks: adds the "DefaultGatewayIPv4" aux-address to the lan interface
// throws an error in any other case because there is no non-hacky way to run a container as a gateway as of now
func dockerGatewayHacky(ctx context.Context, params Params, lan netlink.Link, cli *docker.Client) error {
	networkJSON, err := cli.NetworkInspect(ctx, params.LANNetwork, types.NetworkInspectOptions{})

	if err != nil {
		return fmt.Errorf("error inspecting network %q: %v", params.LANNetwork, err)
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

func findInterfaceByDockerNetwork(dnet string, j types.ContainerJSON) (netlink.Link, error) {
	n, ok := j.NetworkSettings.Networks[dnet]
	if !ok {
		return nil, fmt.Errorf("network %q not found on container info", dnet)
	}

	ip := net.ParseIP(n.IPAddress)
	if ip == nil {
		return nil, fmt.Errorf("could not parse conatiner ip address %q", n.IPAddress)
	}

	return linkForIP(ip)
}

func linkForIP(ip net.IP) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("error listing network links: %v", err)
	}

	for _, link := range links {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return nil, fmt.Errorf("error listing addrs on %q: %v", link.Attrs().Name, err)
		}
		for _, addr := range addrs {
			if addr.IPNet.IP.Equal(ip) {
				return link, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find link for ip %v", ip)
}
