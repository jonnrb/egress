package docker

import (
	"context"
	"errors"
	"fmt"

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

func (params Params) check() error {
	if params.LANNetwork == "" {
		return errors.New("-docker.lan_network flag must be specified")
	}

	if params.UplinkNetwork == "" && params.UplinkInterface == "" {
		return errors.New("-docker.uplink_network or -docker.uplink_interface must be specified")
	}

	return nil
}

type Config struct {
	params Params
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
	if err := params.check(); err != nil {
		return nil, err
	}

	env, err := loadEnvironment(ctx, params)
	if err != nil {
		return nil, err
	}
	defer env.cli.Close()

	return getConfigInternal(env)
}

// Environment bundle throughout ctx.
type environment struct {
	ctx           context.Context
	params        Params
	containerJSON *types.ContainerJSON
	cli           *docker.Client
}

func loadEnvironment(ctx context.Context, params Params) (env environment, err error) {
	env.ctx = ctx
	env.params = params

	env.cli, err = docker.NewEnvClient()
	if err != nil {
		err = fmt.Errorf("error connecting to docker: %v", err)
		return
	}
	log.V(2).Info("Connected to docker")

	env.containerJSON, err = getContainerJSON(ctx, env.cli)
	if err != nil {
		env.cli.Close()
		return
	}
	return
}

func getConfigInternal(env environment) (*Config, error) {
	sr, err := extractStaticRoutes(env)
	if err != nil {
		return nil, err
	}

	uplinkInterface, err := getUplinkInterface(env)
	if err != nil {
		return nil, err
	}

	lanInterface, err := findInterfaceByDockerNetwork(env.params.LANNetwork, env.containerJSON)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		params: env.params,
		lan:    lanInterface,
		uplink: uplinkInterface,
		flat:   sr,
	}

	return cfg, nil
}

func (c *Config) Activate(ctx context.Context) error {
	env, err := loadEnvironment(ctx, c.params)
	if err != nil {
		return err
	}

	log.V(2).Info("Applying gateway hack")
	return dockerGatewayHacky(env, c.lan)
}

func extractStaticRoutes(env environment) ([]fw.StaticRoute, error) {
	var sr []fw.StaticRoute
	for _, flatNetwork := range env.params.FlatNetworks {
		i, err := findInterfaceByDockerNetwork(flatNetwork, env.containerJSON)
		if err != nil {
			return nil, err
		}

		n, err := env.cli.NetworkInspect(env.ctx, flatNetwork, types.NetworkInspectOptions{})
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
	return sr, nil
}

func getUplinkInterface(env environment) (l netlink.Link, err error) {
	i, n := env.params.UplinkInterface, env.params.UplinkNetwork
	if i != "" {
		l, err = netlink.LinkByName(i)
		if err != nil {
			err = fmt.Errorf("could not get interface %q: %v", i, err)
		}
	} else {
		l, err = findInterfaceByDockerNetwork(n, env.containerJSON)
		if err != nil {
			err = fmt.Errorf("could not get interface for container network %q: %v", n, err)
		}
	}
	return
}
