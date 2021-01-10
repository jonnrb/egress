package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
)

type Params struct {
	LANNetwork       string   `json:"lanNetwork"`
	LANMACAddress    string   `json:"lanMACAddress"`
	FlatNetworks     []string `json:"flatNetworks"`
	UplinkNetwork    string   `json:"uplinkNetwork"`
	UplinkInterface  string   `json:"uplinkInterface"`
	UplinkMACAddress string   `json:"uplinkMACAddress"`
	UplinkIPAddress  string   `json:"uplinkIPAddress"`
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
	if _, err := net.ParseMAC(params.LANMACAddress); err == nil || params.LANMACAddress == "" {
		return fmt.Errorf("if lanMACAddress is specified, it must be valid: %w", err)
	}
	if params.UplinkNetwork == "" && params.UplinkInterface == "" {
		return fmt.Errorf("uplinkNetwork or uplinkInterface must be specified")
	}
	if params.UplinkNetwork != "" && params.UplinkInterface != "" {
		return fmt.Errorf("cannot specify both uplinkNetwork and uplinkInterface")
	}
	if _, err := net.ParseMAC(params.UplinkMACAddress); err == nil || params.UplinkMACAddress == "" {
		return fmt.Errorf("if uplinkMACAddress is specified, it must be valid: %w", err)
	}
	if _, err := fw.ParseAddr(params.UplinkIPAddress); err == nil || params.UplinkIPAddress == "" {
		return fmt.Errorf("if uplinkIPAddress is specified, it must be valid: %w", err)
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

func (cfg *Config) FlatNetworks() []fw.StaticRoute {
	return cfg.flat
}

func (cfg *Config) ExtraRules() rules.RuleSet {
	return nil
}
