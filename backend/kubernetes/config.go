package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/leasestore"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/vaddr/dhcp"
)

type Params struct {
	LANNetwork           string   `json:"lanNetwork"`
	LANMACAddress        string   `json:"lanMACAddress"`
	FlatNetworks         []string `json:"flatNetworks"`
	UplinkNetwork        string   `json:"uplinkNetwork"`
	UplinkInterface      string   `json:"uplinkInterface"`
	UplinkMACAddress     string   `json:"uplinkMACAddress"`
	UplinkIPAddress      string   `json:"uplinkIPAddress"`
	UplinkLeaseConfigMap string   `json:"uplinkLeaseConfigMap"`
}

// Reads params from the file `/etc/config/egress.json`.
func ParamsFromFile() (params Params, err error) {
	var f io.ReadCloser
	f, err = os.Open("/etc/config/egress.json")
	if err != nil {
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&params)
	return
}

func (params Params) check() error {
	if params.LANNetwork == "" {
		return fmt.Errorf("lanNetwork must be specified")
	}
	if _, err := net.ParseMAC(params.LANMACAddress); err != nil && params.LANMACAddress != "" {
		return fmt.Errorf("if lanMACAddress is specified, it must be valid: %w", err)
	}
	if params.UplinkNetwork == "" && params.UplinkInterface == "" {
		return fmt.Errorf("uplinkNetwork or uplinkInterface must be specified")
	}
	if params.UplinkNetwork != "" && params.UplinkInterface != "" {
		return fmt.Errorf("cannot specify both uplinkNetwork and uplinkInterface")
	}
	if _, err := net.ParseMAC(params.UplinkMACAddress); err != nil && params.UplinkMACAddress != "" {
		return fmt.Errorf("if uplinkMACAddress is specified, it must be valid: %w", err)
	}
	if _, err := fw.ParseAddr(params.UplinkIPAddress); err != nil && params.UplinkIPAddress != "" {
		return fmt.Errorf("if uplinkIPAddress is specified, it must be valid: %w", err)
	}
	if _, _, err := params.uplinkLeaseStoreName(); params.UplinkLeaseConfigMap != "" && err != nil {
		return fmt.Errorf("if uplinkLeaseStoreName is specified, it must be valid: %w", err)
	}
	return nil
}

type Config struct {
	params  Params
	lan     netlink.Link
	lanAddr fw.Addr
	uplink  netlink.Link
	flat    []fw.StaticRoute
}

type link struct{ *netlink.LinkAttrs }

func (l link) Name() string {
	return l.LinkAttrs.Name
}

func (cfg *Config) LAN() fw.Link {
	return link{cfg.lan.Attrs()}
}

func (cfg *Config) LANHWAddr() net.HardwareAddr {
	a, err := net.ParseMAC(cfg.params.LANMACAddress)
	if err != nil {
		return nil
	}
	return a
}

func (cfg *Config) LANAddr() (a fw.Addr, ok bool) {
	return cfg.lanAddr, true
}

func (cfg *Config) Uplink() fw.Link {
	return link{cfg.uplink.Attrs()}
}

func (cfg *Config) UplinkHWAddr() net.HardwareAddr {
	a, err := net.ParseMAC(cfg.params.UplinkMACAddress)
	if err != nil {
		return nil
	}
	return a
}

func (cfg *Config) UplinkLeaseStore() dhcp.LeaseStore {
	if cfg.params.UplinkLeaseConfigMap == "" {
		return nil
	}
	ns, name, err := cfg.params.uplinkLeaseStoreName()
	if err != nil {
		panic(fmt.Sprintf(
			"kubernetes: should have been checked on the way in: %v", err))
	}
	return &leasestore.LeaseStore{
		Name:      name,
		Namespace: ns,
	}
}

func (params Params) uplinkLeaseStoreName() (ns, name string, err error) {
	cm := params.UplinkLeaseConfigMap
	v := strings.SplitN(cm, "/", 3)
	switch len(v) {
	case 1:
		name = v[0]
	case 2:
		ns, name = v[0], v[1]
	default:
		err = fmt.Errorf("kubernetes: invalid namespace/name string: %q", cm)
	}
	return
}

func (cfg *Config) FlatNetworks() []fw.StaticRoute {
	return cfg.flat
}

func (cfg *Config) ExtraRules() rules.RuleSet {
	return nil
}
