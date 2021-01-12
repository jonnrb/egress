package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/client"
	"go.jonnrb.io/egress/backend/kubernetes/internal"
	"go.jonnrb.io/egress/backend/kubernetes/leasestore"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
	"go.jonnrb.io/egress/vaddr/dhcp"
	"golang.org/x/sync/errgroup"
)

func GetConfig(ctx context.Context, params Params) (*Config, error) {
	if err := params.check(); err != nil {
		return nil, err
	}
	log.V(2).Infof("k8s backend params: %+v", params)

	env, err := loadEnvironment()
	if err != nil {
		return nil, err
	}

	// Create a sub-context so we can cancel any futures on the first failure.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return getConfigInternal(ctx, env, params)
}

type environment struct {
	cli         *internal.CNIClient
	attachments map[string]internal.Attachment
}

func loadEnvironment() (env environment, err error) {
	cli, err := client.Get()
	if err != nil {
		err = fmt.Errorf("could not get kubernetes client: %w", err)
		return
	}
	env.cli, err = internal.NewCNIClient(cli)
	if err != nil {
		err = fmt.Errorf("could not get kubernetes client: %w", err)
		return
	}

	attachments, err := internal.GetAttachments()
	if err != nil {
		err = fmt.Errorf("could not get attached networks: %w", err)
		return
	}
	env.attachments = makeAttachmentMap(attachments)
	log.V(2).Infof("attached to networks: %+v", attachments)

	return
}

func getConfigInternal(ctx context.Context, env environment, params Params) (*Config, error) {
	var (
		uplink, lan      netlink.Link
		lanAddr          fw.Addr
		flat             []fw.StaticRoute
		uplinkLeaseStore dhcp.LeaseStore
	)
	grp, ctx := errgroup.WithContext(ctx)

	grp.Go(func() (err error) {
		uplink, err = getUplink(env, params)
		return
	})
	grp.Go(func() (err error) {
		lan, err = getLAN(env, params)
		return
	})
	grp.Go(func() (err error) {
		lanAddr, err = getLANGWAddr(ctx, env, params)
		return
	})
	grp.Go(func() (err error) {
		flat, err = getFlatNetworks(ctx, env, params)
		return
	})
	grp.Go(func() (err error) {
		uplinkLeaseStore, err = getUplinkLeaseStore(params)
		return
	})

	if err := grp.Wait(); err != nil {
		return nil, err
	}

	return &Config{
		params:           params,
		uplink:           uplink,
		uplinkLeaseStore: uplinkLeaseStore,
		lan:              lan,
		lanAddr:          lanAddr,
		flat:             flat,
	}, nil
}

func getUplink(env environment, params Params) (netlink.Link, error) {
	wrappedErr := func(l netlink.Link, err error) (netlink.Link, error) {
		if err != nil {
			err = fmt.Errorf("could not get uplink: %w", err)
		}
		return l, err
	}

	if params.UplinkInterface != "" {
		return wrappedErr(netlink.LinkByName(params.UplinkInterface))
	} else if params.UplinkNetwork != "" {
		return wrappedErr(getLinkForNet(env.attachments, params.UplinkNetwork))
	} else {
		panic("params.check() should make this condition impossible")
	}
}

func getLAN(env environment, params Params) (netlink.Link, error) {
	lan, err := getLinkForNet(env.attachments, params.LANNetwork)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get link for LAN network %q: %w", params.LANNetwork, err)
	}
	return lan, nil
}

func getFlatNetworks(ctx context.Context, env environment, params Params) ([]fw.StaticRoute, error) {
	links, err := getFlatNetLinks(env, params)
	if err != nil {
		return nil, err
	}

	netMap, err := getNetMap(ctx, env.cli, params.FlatNetworks)
	if err != nil {
		return nil, err
	}

	var routes []fw.StaticRoute
	for name, net := range netMap {
		for _, r := range net.Ranges {
			routes = append(routes, fw.StaticRoute{
				Link: links[name],
				Subnet: fw.Addr{
					IP:   r.Subnet.IP,
					Mask: r.Subnet.Mask,
				},
			})
		}
	}
	return routes, nil
}

func getFlatNetLinks(env environment, params Params) (map[string]fw.Link, error) {
	m := make(map[string]fw.Link)
	for _, net := range params.FlatNetworks {
		l, err := getLinkForNet(env.attachments, net)
		if err != nil {
			return nil, fmt.Errorf("could not link link for flat network %q: %v", net, err)
		}
		m[net] = link{l.Attrs()}
	}
	return m, nil
}

func getLANGWAddr(ctx context.Context, env environment, params Params) (a fw.Addr, err error) {
	net, err := env.cli.Get(ctx, params.LANNetwork)
	if err != nil {
		err = fmt.Errorf(
			"kubernetes: error getting CNI config for network %q: %w",
			params.LANNetwork, err)
		return
	}

	for _, r := range net.Ranges {
		if r.Gateway != nil {
			ip := r.Gateway.String()
			bits, _ := r.Subnet.Mask.Size()
			if a, err = fw.ParseAddr(fmt.Sprintf("%s/%d", ip, bits)); err != nil {
				panic(fmt.Sprintf(
					"kubernetes: could not parse cidr address: %v", err))
			}
			return
		}
	}
	err = fmt.Errorf("kubernetes: no gateway found in network definition")
	return
}

func getUplinkLeaseStore(params Params) (dhcp.LeaseStore, error) {
	if params.UplinkLeaseConfigMap == "" {
		return nil, nil
	}
	ns, name, err := splitNamespaceName(params.UplinkLeaseConfigMap)
	if err != nil {
		panic(fmt.Sprintf(
			"kubernetes: should have been checked on the way in: %v", err))
	}
	ls, err := leasestore.New()
	if err != nil {
		return nil, fmt.Errorf(
			"kubernetes: could not create LeaseStore: %w", err)
	}
	ls.Name = name
	ls.Namespace = ns
	return ls, nil
}

func splitNamespaceName(s string) (ns, name string, err error) {
	v := strings.SplitN(s, "/", 3)
	switch len(v) {
	case 1:
		name = v[0]
	case 2:
		ns, name = v[0], v[1]
	default:
		err = fmt.Errorf("kubernetes: invalid [namespace/]name string: %q", s)
	}
	return
}
