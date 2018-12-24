package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/vishvananda/netlink"
	"go.jonnrb.io/egress/backend/docker"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/log"
	"go.jonnrb.io/egress/metrics"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

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

func main() {
	flag.Parse()

	args := flag.Args()

	if *healthCheck {
		client := &http.Client{}
		_, port, err := net.SplitHostPort(*httpAddr)
		if err != nil {
			fmt.Printf("bad address %q: %v\n", *httpAddr, err)
			os.Exit(1)
		}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%v/health", port))
		if err != nil {
			fmt.Printf("error connecting to healthcheck: %v\n", err)
			os.Exit(1)
		}
		io.Copy(os.Stdout, resp.Body)
		if resp.StatusCode != http.StatusOK {
			os.Exit(resp.StatusCode)
		}
		return
	}

	var err error
	if *cmd != "" {
		if len(args) > 0 {
			log.Fatal("-c or an exec line; pick one")
		}
		args, err = shlex.Split(*cmd)
		if err != nil {
			log.Fatalf("error parsing shell command %q: %v", *cmd, err)
		}
	}

	log.V(2).Infof("MaybeCreateNetworks()")
	if err := MaybeCreateNetworks(); err != nil {
		log.Fatalf("error creating networks: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	log.V(2).Infof("docker.GetConfig()")
	cfg, err := docker.GetConfig(ctx, docker.Params{
		LANNetwork:      *lanNetwork,
		FlatNetworks:    strings.Split(*flatNetworks, ","),
		UplinkNetwork:   *uplinkNetwork,
		UplinkInterface: *uplinkInterfaceName,
	})
	if err != nil {
		log.Fatalf("error initializing network configuration: %v", err)
	}
	cancel()

	l, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal(err)
	}

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		log.Fatal(err)
	}
	r := fw.OpenPort("tcp", port)

	log.Infof("listening on %q", *httpAddr)

	log.V(2).Infof("PatchIPTables()")
	if err := fw.Apply(fw.WithExtraRules(cfg, rules.RuleSet{r})); err != nil {
		log.Fatalf("error patching iptables: %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	metricsHandler, err := metrics.New(ctx, metrics.Config{
		UplinkName: cfg.Uplink().Name(),
	})
	if err != nil {
		log.Fatalf("error setting up metrics: %v", err)
	}
	http.Handle("/metrics", metricsHandler)

	hc := SetupHealthCheck()
	defer hc.Close()

	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		defer cancel()
		go func() {
			<-ctx.Done()
			l.Close()
		}()
		return http.Serve(l, nil)
	})
	grp.Go(func() error {
		defer cancel()

		if len(args) > 0 {
			log.Infof("running %q", strings.Join(args, " "))
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("error starting subprocess: %v", err)
			}
			if err := ReapChildren(cmd.Process); err != nil {
				return fmt.Errorf("error waiting for subprocess: %v", err)
			}
		} else {
			log.Info("sleeping forever")
			for {
				time.Sleep(time.Duration(9223372036854775807))
			}
		}
		return nil
	})

	if err := grp.Wait(); err != nil {
		log.Fatal(err)
	}
}

func MaybeCreateNetworks() error {
	if *tunCreateName == "" {
		return nil
	}

	if err := maybeCreateDevNetTun(); err != nil {
		return fmt.Errorf("error creating /dev/net/tun: %v", err)
	}

	la := netlink.NewLinkAttrs()
	la.Name = *tunCreateName

	link := &netlink.Tuntap{
		LinkAttrs: la,
		Mode:      netlink.TUNTAP_MODE_TUN,
		Flags:     netlink.TUNTAP_DEFAULTS,
	}

	err := netlink.LinkAdd(link)
	if err != nil {
		return fmt.Errorf("error creating tun %q: %v", *tunCreateName, err)
	}

	return nil
}

func maybeCreateDevNetTun() error {
	if err := os.Mkdir("/dev/net", os.FileMode(0755)); !os.IsExist(err) && err != nil {
		return err
	}
	tunMode := uint32(020666)
	if err := unix.Mknod("/dev/net/tun", tunMode, int(unix.Mkdev(10, 200))); !os.IsExist(err) && err != nil {
		return err
	}
	return nil
}
