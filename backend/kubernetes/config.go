package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/kubernetes/coordinator"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/ha"
	"go.jonnrb.io/egress/vaddr/dhcp"
)

type Params struct {
	LANNetwork           string    `json:"lanNetwork"`
	LANMACAddress        string    `json:"lanMACAddress"`
	FlatNetworks         []string  `json:"flatNetworks"`
	UplinkNetwork        string    `json:"uplinkNetwork"`
	UplinkInterface      string    `json:"uplinkInterface"`
	UplinkMACAddress     string    `json:"uplinkMACAddress"`
	UplinkIPAddress      string    `json:"uplinkIPAddress"`
	UplinkLeaseConfigMap string    `json:"uplinkLeaseConfigMap"`
	HA                   *HAParams `json:"ha"`
}

type HAParams struct {
	LockName      string `json:"lockName"`
	LeaseDuration string `json:"leaseDuration"`
	RenewDeadline string `json:"renewDeadline"`
	RetryPeriod   string `json:"retryPeriod"`
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
	if _, _, err := splitNamespaceName(params.UplinkLeaseConfigMap); params.UplinkLeaseConfigMap != "" && err != nil {
		return fmt.Errorf("if uplinkLeaseStoreName is specified, it must be valid: %w", err)
	}
	if err := params.HA.check(); err != nil {
		return fmt.Errorf("if ha is specified, it must be valid: %w", err)
	}
	return nil
}

func (haParams *HAParams) check() error {
	if haParams == nil {
		return nil
	}
	if _, _, err := splitNamespaceName(haParams.LockName); err != nil {
		return fmt.Errorf("lockName must be valid: %w", err)
	}
	if _, err := time.ParseDuration(haParams.LeaseDuration); haParams.LeaseDuration != "" && err != nil {
		return fmt.Errorf("if leaseDuration is specified, it must be valid: %w", err)
	}
	if _, err := time.ParseDuration(haParams.RenewDeadline); haParams.RenewDeadline != "" && err != nil {
		return fmt.Errorf("if renewDeadline is specified, it must be valid: %w", err)
	}
	if _, err := time.ParseDuration(haParams.RetryPeriod); haParams.RetryPeriod != "" && err != nil {
		return fmt.Errorf("if retryPeriod is specified, it must be valid: %w", err)
	}
	return nil
}

type Config struct {
	params           Params
	uplink           netlink.Link
	uplinkLeaseStore dhcp.LeaseStore
	lan              netlink.Link
	lanAddr          fw.Addr
	flat             []fw.StaticRoute
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
	return cfg.uplinkLeaseStore
}

func (cfg *Config) HACoordinator() ha.Coordinator {
	if cfg.params.HA == nil {
		return nil
	}
	var (
		lockNamespace, lockName, err = splitNamespaceName(cfg.params.HA.LockName)
		leaseDuration, _             = time.ParseDuration(cfg.params.HA.LeaseDuration)
		renewDeadline, _             = time.ParseDuration(cfg.params.HA.RenewDeadline)
		retryPeriod, _               = time.ParseDuration(cfg.params.HA.RetryPeriod)
	)
	if err != nil {
		panic(fmt.Sprintf(
			"params.check() should make this condition impossible: %v", err))
	}
	if leaseDuration == 0 {
		leaseDuration = 10 * time.Second
	}
	if renewDeadline == 0 {
		renewDeadline = 5 * time.Second
	}
	if retryPeriod == 0 {
		retryPeriod = 1 * time.Second
	}
	return &coordinator.Coordinator{
		LockName:      lockName,
		LockNamespace: lockNamespace,

		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
	}
}

func (cfg *Config) FlatNetworks() []fw.StaticRoute {
	return cfg.flat
}

func (cfg *Config) ExtraRules() rules.RuleSet {
	return nil
}
