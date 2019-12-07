package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"

	cni "github.com/K8sNetworkPlumbingWG/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	"github.com/ericchiang/k8s"
)

// Wrapper around the CNI CRD k8s API.
type CNIClient struct {
	k *k8s.Client
}

func NewCNIClient(c *k8s.Client) *CNIClient {
	return &CNIClient{c}
}

type NetworkDefinition struct {
	Ranges []Range
}

type Range struct {
	Gateway net.IP
	Subnet  net.IPNet
}

var (
	currentNamespaceOnce sync.Once
	currentNamespace     string
	currentNamespaceErr  error
)

func getCurrentNamespace() (string, error) {
	currentNamespaceOnce.Do(func() {
		var ns []byte
		ns, currentNamespaceErr = ioutil.ReadFile(
			"/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		currentNamespace = string(ns)
	})
	return currentNamespace, currentNamespaceErr
}

// Gets a NetworkDefinition by its name, where netName is "namespace/net" or
// just "net" in the current namespace.
func (c *CNIClient) Get(ctx context.Context, netName string) (*NetworkDefinition, error) {
	namespace, name := splitFullName(netName)

	if namespace == "" {
		var err error
		namespace, err = getCurrentNamespace()
		if err != nil {
			return nil, fmt.Errorf("network namespace not provided and could not figure out current namespace from environment: %v", err)
		}
	}

	var net cni.NetworkAttachmentDefinition
	// Use the local, registered type to make a call.
	if err := c.k.Get(ctx, namespace, name, (*netAttachDef)(&net)); err != nil {
		return nil, err
	}

	conf := net.Spec.Config
	ranges, err := extractRanges([]byte(conf))
	if err != nil {
		return nil, err
	}

	return &NetworkDefinition{
		Ranges: ranges,
	}, nil
}

func extractRanges(raw []byte) ([]Range, error) {
	type confList struct {
		Plugins []interface{} `json:"plugins"`
	}
	var cl confList
	_ = json.Unmarshal(raw, &cl)
	if len(cl.Plugins) != 0 {
		var err error

		for _, p := range cl.Plugins {
			raw, err = json.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("could not remarshal sub-plugin: %w", err)
			}
			ranges, err := extractRanges(raw)
			if err == nil {
				return ranges, nil
			}
		}

		return nil, fmt.Errorf("could not find IPAM config in plugin chain: %w", err)
	}

	ipamCfg, _, err := allocator.LoadIPAMConfig(raw, "")
	if err != nil {
		return nil, err
	}

	var out []Range
	for _, set := range ipamCfg.Ranges {
		for _, r := range set {
			out = append(out, Range{
				Gateway: r.Gateway,
				Subnet:  net.IPNet(r.Subnet),
			})
		}
	}
	return out, nil
}

// If fullName has a "/", returns the first split, otherwise returns
// `"", fullName`
func splitFullName(fullName string) (namespace, name string) {
	s := strings.SplitN(fullName, "/", 2)
	switch len(s) {
	case 0:
		return "", ""
	case 1:
		return "", fullName
	case 2:
		return s[0], s[1]
	default:
		panic(fmt.Sprintf("impossible return value from strings.SplitN: %v", s))
	}
}
