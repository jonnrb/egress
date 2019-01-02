package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
)

type Params struct {
	LANNetwork      string   `json:"lanNetwork"`
	FlatNetworks    []string `json:"flatNetworks"`
	UplinkNetwork   string   `json:"uplinkNetwork"`
	UplinkInterface string   `json:"uplinkInterface"`
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
	if params.UplinkNetwork == "" && params.UplinkInterface == "" {
		return fmt.Errorf("uplinkNetwork or uplinkInterface must be specified")
	}
	if params.UplinkNetwork != "" && params.UplinkInterface != "" {
		return fmt.Errorf("cannot specify both uplinkNetwork and uplinkInterface")
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
