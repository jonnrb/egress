package docker

import (
	"context"
	"fmt"
	"net"
	"os"

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"github.com/vishvananda/netlink"
)

func getContainerJSON(ctx context.Context, cli *docker.Client) (*types.ContainerJSON, error) {
	// So... don't set a custom hostname I guess?
	containerID, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("error getting hostname: %v", err)
	}

	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect container using id %q: %v", containerID, err)
	}

	return &containerJSON, nil
}

func findInterfaceByDockerNetwork(dnet string, j *types.ContainerJSON) (netlink.Link, error) {
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
