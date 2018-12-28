package kubernetes

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/internal"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
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
	cli, err := internal.GetK8sClient()
	if err != nil {
		err = fmt.Errorf("could not get kubernetes client: %v", err)
		return
	}
	env.cli = internal.NewCNIClient(cli)

	attachments, err := internal.GetAttachments()
	if err != nil {
		err = fmt.Errorf("could not get attached networks: %v", err)
		return
	}
	env.attachments = makeAttachmentMap(attachments)
	log.V(2).Infof("attached to networks: %+v", attachments)

	return
}

func getConfigInternal(ctx context.Context, env environment, params Params) (*Config, error) {
	var (
		uplink, lan netlink.Link
		flat        []fw.StaticRoute
	)
	grp, ctx := errgroup.WithContext(ctx)

	grp.Go(func() (err error) {
		uplink, err = getUplink(env, params)
		return
	})
	grp.Go(func() (err error) {
		lan, err = getLAN(ctx, env, params)
		return
	})
	grp.Go(func() (err error) {
		flat, err = getFlatNetworks(ctx, env, params)
		return
	})

	if err := grp.Wait(); err != nil {
		return nil, err
	}

	return &Config{
		uplink: uplink,
		lan:    lan,
		flat:   flat,
	}, nil
}

func getUplink(env environment, params Params) (netlink.Link, error) {
	wrappedErr := func(l netlink.Link, err error) (netlink.Link, error) {
		if err != nil {
			err = fmt.Errorf("could not get uplink: %v", err)
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

// Gets the LAN link with a context.Context (i.e. this can block). Getting the
// actual link is "immediate", but assigning the gateway IP is not (we have to
// fetch the CNI definition to get the gateway IP).
func getLAN(ctx context.Context, env environment, params Params) (netlink.Link, error) {
	lan, err := getLinkForNet(env.attachments, params.LANNetwork)
	if err != nil {
		return nil, fmt.Errorf("could not get link for LAN network %q: %v", params.LANNetwork, err)
	}

	net, err := env.cli.Get(ctx, params.LANNetwork)
	if err != nil {
		return nil, fmt.Errorf("error getting CNI config for network %q: %v", params.LANNetwork, err)
	}

	if err := applyGWIP(lan, net); err != nil {
		return nil, fmt.Errorf("error applying GW IP to LAN interface: %v", err)
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
			ip := r.Subnet.IP.String()
			cidr, _ := r.Subnet.Mask.Size()
			routes = append(routes, fw.StaticRoute{
				Link:   links[name],
				Subnet: fmt.Sprintf("%s/%d", ip, cidr),
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