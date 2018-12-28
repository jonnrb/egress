package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/internal"
	"golang.org/x/sync/errgroup"
)

const (
	envVarHost = "KUBERNETES_SERVICE_HOST"
	envVarPort = "KUBERNETES_SERVICE_PORT"
)

// Returns if it seems the current environment is hosted by Kubernetes.
func InCluster() bool {
	return os.Getenv(envVarHost) != "" && os.Getenv(envVarPort) != ""
}

// Gets many CNI network on ctx.
func getNetMap(ctx context.Context, cli *internal.CNIClient, netNames []string) (map[string]*internal.NetworkDefinition, error) {
	type mapEntry struct {
		k string
		v *internal.NetworkDefinition
	}
	c := make(chan mapEntry, len(netNames))

	grp, ctx := errgroup.WithContext(ctx)
	for _, netName := range netNames {
		n := netName // make a lexical capture for the goroutine
		grp.Go(func() (err error) {
			var net *internal.NetworkDefinition
			net, err = cli.Get(ctx, n)
			if err == nil {
				c <- mapEntry{n, net}
			}
			return
		})
	}

	err := grp.Wait()
	if err != nil {
		return nil, err
	}

	m := make(map[string]*internal.NetworkDefinition)
	close(c) // all writers have returned, so this is safe (see `grp.Wait()`)
	for e := range c {
		m[e.k] = e.v
	}
	return m, nil
}

// XXX: A pod may be connected to a network by multiple interfaces. It is
// assumed that all of these connections are equivalent.
func makeAttachmentMap(attachs []internal.Attachment) map[string]internal.Attachment {
	m := make(map[string]internal.Attachment)
	for _, a := range attachs {
		m[a.Name] = a
	}
	return m
}

func getLinkForNet(attachments map[string]internal.Attachment, net string) (netlink.Link, error) {
	// net may be of the form namespace/name, but attachments only show up with
	// the "name" bit.
	split := strings.SplitN(net, "/", 2)
	if len(split) == 2 {
		net = split[1]
	}

	attach, ok := attachments[net]
	if !ok {
		return nil, fmt.Errorf("pod not attached to network %q", net)
	}
	return netlink.LinkByName(attach.Interface)
}
