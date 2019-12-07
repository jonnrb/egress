package main

import "flag"

var (
	healthCheck            = flag.Bool("health_check", false, "If set, connects to the internal healthcheck endpoint and exits.")
	tunCreateName          = flag.String("create_tun", "", "If set, creates a tun interface with the specified name (to be used with -docker.uplink_interface and probably a VPN client")
	cmd                    = flag.String("c", "", "Command to run after initialization")
	httpAddr               = flag.String("http.addr", "0.0.0.0:8080", "Port to serve metrics and health status on")
	openPortsCSV           = flag.String("open_ports", "", "Additional ports to open (tcp/1234,udp/2345,tcp/654/lo)")
	blockInterfaceInputCSV = flag.String("block_interface_input", "", "Interfaces that cannot connect to ports on this router (e.g. eth0,eth1)")
	noCmd                  = flag.Bool("no_cmd", false, "Exit on success (the default when no cmd is specified is to sleep)")
	justMetrics            = flag.Bool("just_metrics", false, "Just serves metrics without doing any setup (meant to be used in a pod)")

	lanNetwork          = flag.String("docker.lan_network", "", "Container network that this container will act as the gateway for")
	flatNetworks        = flag.String("docker.flat_networks", "", "CSV of container networks that this container will forward to (not masqueraded)")
	uplinkNetwork       = flag.String("docker.uplink_network", "", "Container network used for uplink (connections will be masqueraded)")
	uplinkInterfaceName = flag.String("docker.uplink_interface", "", "Interface used for uplink (connections will be masqueraded)")
)
