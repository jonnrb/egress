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
	args := processArgs()

	if *healthCheck {
		healthCheckMain()
		return
	}

	// Create things that aren't bound by the main context.Context.
	maybeCreateNetworks()
	cfg := getFWConfig()
	httpCfg := listenHTTP()
	applyFWRules(cfg, httpCfg)

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

func processArgs() []string {
	args := flag.Args()
	if *cmd == "" {
		return args
	}

	if len(args) > 0 {
		log.Fatal("Delegate process can be specifed by -c and a string or a list of args, but not both")
	}

	args, err := shlex.Split(*cmd)
	if err != nil {
		log.Fatalf("Error parsing shell command %q: %v", *cmd, err)
	}

	return args
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

type httpConfig struct {
	listener     net.Listener
	openPortRule rules.Rule
	mux          *http.ServeMux
}

func listenHTTP() httpConfig {
	l, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatalf("Could not listen on given -http.addr %q: %v", *httpAddr, err)
	}
	log.Infof("listening on %q", *httpAddr)

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		panic(err)
	}

	return httpConfig{
		listener:     l,
		openPortRule: fw.OpenPort("tcp", port),
		mux:          http.NewServeMux(),
	}
}

func applyFWRules(cfg fw.Config, httpCfg httpConfig) {
	log.V(2).Info("Applying fw rules from environment")

	extraRules := rules.RuleSet{httpCfg.openPortRule}
	if err := fw.Apply(fw.WithExtraRules(cfg, extraRules)); err != nil {
		log.Fatalf("Error applying fw rules: %v", err)
	}
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
