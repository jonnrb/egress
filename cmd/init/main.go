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
	"go.jonnrb.io/egress/backend/docker"
	"go.jonnrb.io/egress/backend/kubernetes"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/health"
	"go.jonnrb.io/egress/log"
	"go.jonnrb.io/egress/metrics"
	"go.jonnrb.io/egress/util"
	"golang.org/x/sync/errgroup"
)

func main() {
	flag.Parse()
	args, extraRules := processArgs()

	if *healthCheck {
		healthCheckMain()
		return
	}

	// Create things that aren't bound by the main context.Context.
	maybeCreateNetworks()
	cfg := getFWConfig()
	if *noCmd && !*justMetrics {
		// Skip some stuff if noCmd.
		applyFWRules(cfg, extraRules)
		return
	}

	httpCfg := listenHTTP()
	if *justMetrics {
		httpServeContext(context.Background(), httpCfg)
		return
	}

	applyFWRules(cfg, extraRules)

	// Create the root context.Context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupHTTPHandlers(ctx, cfg, httpCfg)

	// Create the steady-state.
	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		return httpServeContext(ctx, httpCfg)
	})
	grp.Go(func() error {
		return runSubprocess(ctx, args)
	})

	switch err := grp.Wait(); err {
	case errSubprocessExited, http.ErrServerClosed, nil:
	default:
		log.Fatal(err)
	}
}

func healthCheckMain() {
	client := &http.Client{}
	_, port, err := net.SplitHostPort(*httpAddr)
	if err != nil {
		fmt.Printf("bad address %q: %v\n", *httpAddr, err)
		os.Exit(1)
	}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%v/healthz", port))
	if err != nil {
		fmt.Printf("error connecting to healthcheck: %v\n", err)
		os.Exit(1)
	}
	io.Copy(os.Stdout, resp.Body)
	if resp.StatusCode != http.StatusOK {
		os.Exit(resp.StatusCode)
	}
}

func processArgs() (args []string, openPortRules rules.RuleSet) {
	openPortRules = append(getOpenPortRules(), openHTTPPort())
	args = flag.Args()

	if *cmd == "" {
		return
	}

	if len(args) > 0 {
		log.Fatal("Delegate process can be specifed by -c and a string or a list of args, but not both")
	}

	var err error
	args, err = shlex.Split(*cmd)
	if err != nil {
		log.Fatalf("Error parsing shell command %q: %v", *cmd, err)
	}
	return
}

func maybeCreateNetworks() {
	tun := *tunCreateName
	if tun == "" {
		return
	}

	log.V(2).Infof("Attempting to create tunnel %q", tun)
	err := util.CreateTun(tun)
	if err != nil {
		log.Fatalf("Could not create tunnel specified by -create_tun: %v", err)
	}
}

func getFWConfig() fw.Config {
	log.V(2).Info("Getting fw.Config")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if kubernetes.InCluster() {
		params, err := kubernetes.ParamsFromFile()
		if err != nil {
			log.Fatalf("Error getting Kubernetes router parameters: %v", err)
		}
		cfg, err := kubernetes.GetConfig(ctx, params)
		if err != nil {
			log.Fatalf("Error configuring router from Kubernetes environment: %v", err)
		}
		return cfg
	} else {
		cfg, err := docker.GetConfig(ctx, docker.Params{
			LANNetwork:      *lanNetwork,
			FlatNetworks:    strings.Split(*flatNetworks, ","),
			UplinkNetwork:   *uplinkNetwork,
			UplinkInterface: *uplinkInterfaceName,
		})
		if err != nil {
			log.Fatalf("Error configuring router from Docker environment: %v", err)
		}
		return cfg
	}
}

func maybeActivateFWConfig(cfg fw.Config) {
	type dormantConfig interface {
		Activate(ctx context.Context) error
	}
	dormantCfg, ok := cfg.(dormantConfig)
	if !ok {
		log.V(2).Info("FW config requires no activation")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dormantCfg.Activate(ctx); err != nil {
		log.Fatalf("Error activating config: %v", err)
	}
}

type httpConfig struct {
	listener net.Listener
	mux      *http.ServeMux
}

func listenHTTP() httpConfig {
	l, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatalf("Could not listen on given -http.addr %q: %v", *httpAddr, err)
	}
	log.Infof("listening on %q", *httpAddr)

	return httpConfig{
		listener: l,
		mux:      http.NewServeMux(),
	}
}

func openHTTPPort() rules.Rule {
	_, port, err := net.SplitHostPort(*httpAddr)
	if err != nil {
		log.Fatalf("Bad \"-http.addr\" %q: %v", *httpAddr, err)
	}
	return fw.OpenPort("tcp", port)
}

func applyFWRules(cfg fw.Config, extraRules rules.RuleSet) {
	maybeActivateFWConfig(cfg)

	log.V(2).Info("Applying fw rules from environment")
	if err := fw.Apply(fw.WithExtraRules(cfg, extraRules)); err != nil {
		log.Fatalf("Error applying fw rules: %v", err)
	}
}

func getOpenPortRules() (r rules.RuleSet) {
	for _, s := range strings.Split(*openPortsCSV, ",") {
		s2 := strings.SplitN(s, "/", 2)
		if len(s2) != 2 {
			log.Fatalf("Flag \"-open_ports\" should be a CSV of (tcp|udp)/port pairs; got %q", *openPortsCSV)
		}
		r = append(r, fw.OpenPort(s2[0], s2[1]))
	}
	return
}

func setupHTTPHandlers(ctx context.Context, cfg fw.Config, httpCfg httpConfig) {
	metricsHandler, err := metrics.New(ctx, metrics.Config{
		UplinkName: cfg.Uplink().Name(),
	})
	if err != nil {
		log.Fatalf("Error setting up metrics: %v", err)
	}

	httpCfg.mux.Handle("/metrics", metricsHandler)
	httpCfg.mux.Handle("/healthz", health.New(ctx))
}

func httpServeContext(ctx context.Context, cfg httpConfig) error {
	s := http.Server{
		Handler: cfg.mux,

		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		defer s.Close()
		<-ctx.Done()
	}()

	err := s.Serve(cfg.listener)
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}

var errSubprocessExited = fmt.Errorf("subprocess exited")

func runSubprocess(ctx context.Context, args []string) error {
	if len(args) > 0 {
		log.Infof("running %q", strings.Join(args, " "))
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("error starting subprocess: %v", err)
		}
		if err := util.ReapChildren(cmd.Process); err != nil {
			return fmt.Errorf("error waiting for subprocess: %v", err)
		}
		return errSubprocessExited
	} else {
		log.Info("sleeping forever")
		<-ctx.Done()
		return nil
	}
}
