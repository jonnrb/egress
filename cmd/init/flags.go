package main

import "flag"

var (
	healthCheck   = flag.Bool("health_check", false, "If set, connects to the internal healthcheck endpoint and exits.")
	tunCreateName = flag.String("create_tun", "", "If set, creates a tun interface with the specified name (to be used with -docker.uplink_interface and probably a VPN client")
	cmd           = flag.String("c", "", "Command to run after initialization")
	httpAddr      = flag.String("http.addr", "0.0.0.0:8080", "Port to serve metrics and health status on")

	lanNetwork          = flag.String("docker.lan_network", "", "Container network that this container will act as the gateway for")
	flatNetworks        = flag.String("docker.flat_networks", "", "CSV of container networks that this container will forward to (not masqueraded)")
	uplinkNetwork       = flag.String("docker.uplink_network", "", "Container network used for uplink (connections will be masqueraded)")
	uplinkInterfaceName = flag.String("docker.uplink_interface", "", "Interface used for uplink (connections will be masqueraded)")
)
