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
	"strconv"
	"strings"
	"time"

	"github.com/google/shlex"
	"go.jonnrb.io/egress/backend/kubernetes"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/fw/fwutil"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/ha"
	"go.jonnrb.io/egress/health"
	"go.jonnrb.io/egress/log"
	"go.jonnrb.io/egress/metrics"
	"go.jonnrb.io/egress/util"
	"go.jonnrb.io/egress/vaddr"
	"go.jonnrb.io/egress/vaddr/vaddrha"
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

	// Get the ha.Coordinator (if configured).
	hac := fwutil.GetHACoordinator(cfg)
	var m ha.MemberGroup

	va := vaddr.Join(
		fwutil.MakeVAddrLAN(cfg),
		fwutil.MakeVAddrUplink(cfg))

	if *noCmd && !*justMetrics && !vaddr.HasActive(va) {
		if hac != nil {
			log.Warning("HA is configured but -noCmd was specified.")
		}
		// Skip some stuff if noCmd.
		applyFWRules(cfg, extraRules)
		onlyStartVAddr(va)
		return
	}

	httpCfg := listenHTTP()
	if *justMetrics {
		if vaddr.HasActive(va) {
			log.Fatalf(
				"Trying to run with -justMetrics when active virtual addresses configured")
		}
		if hac != nil {
			log.Warning("Running with -justMetrics but HA is configured.")
		}
		ctx := context.Background()
		setupHTTPHandlers(ctx, cfg, httpCfg, nil)
		httpServeContext(ctx, httpCfg)
		return
	}

	applyFWRules(cfg, extraRules)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupHTTPHandlers(ctx, cfg, httpCfg, &m)

	// Create the steady-state.
	va.Actives = append(va.Actives,
		vaddr.ActiveFunc(func(ctx context.Context) error {
			return httpServeContext(ctx, httpCfg)
		}))
	va.Actives = append(va.Actives,
		vaddr.ActiveFunc(func(ctx context.Context) error {
			return runSubprocess(ctx, args)
		}))

	if hac != nil {
		m.Add(vaddrha.New(va))
		switch err := hac.Run(ctx, &m); err {
		case errSubprocessExited, http.ErrServerClosed, nil:
		default:
			log.Fatal(err)
		}
	} else {
		switch err := va.Run(ctx); err {
		case errSubprocessExited, http.ErrServerClosed, nil:
		default:
			log.Fatal(err)
		}
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

func processArgs() (args []string, extraRules rules.RuleSet) {
	extraRules = getOpenPortRules()
	extraRules = append(extraRules, openHTTPPort())
	extraRules = append(extraRules, getBlockInterfaceInputRules()...)
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
	if tun != "" {
		log.V(2).Infof("Attempting to create tunnel %q", tun)
		err := util.CreateTun(tun)
		if err != nil {
			log.Fatalf("Could not create tunnel specified by -create_tun: %v", err)
		}
	}

	wg := *wgCreateName
	if wg != "" {
		log.V(2).Infof("Attempting to create wireguard dev %q", wg)
		err := util.CreateWg(wg)
		if err != nil {
			log.Fatalf("Could not create wireguard dev specified by -create_wg: %v", err)
		}
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
		log.Fatalf("Error configuring router: no available configuration backend")
		return nil
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
	_, portString, err := net.SplitHostPort(*httpAddr)
	if err != nil {
		log.Fatalf("Bad \"-http.addr\" %q: %v", *httpAddr, err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Fatalf("Bad \"-http.addr\" %q: %v", *httpAddr, err)
	}
	if *httpIface != "" {
		return fw.OpenPortOnInterface("tcp", port, fw.LinkString(*httpIface))
	} else {
		return fw.OpenPort("tcp", port)
	}
}

func applyFWRules(cfg fw.Config, extraRules rules.RuleSet) {
	log.V(2).Info("Applying fw rules from environment")
	if err := fw.Apply(fw.WithExtraRules(cfg, extraRules)); err != nil {
		log.Fatalf("Error applying fw rules: %v", err)
	}
}

func onlyStartVAddr(s vaddr.Suite) {
	log.V(2).Info("Bringing up virtual addresses")
	w, _ := vaddr.Split(s)
	err := w.Start()
	if err != nil {
		log.V(2).Infof(
			"Could not bring up virtual addresses from vaddr.Suite: %+v", s)
		log.Fatalf("Error bringing up virtual addresses: %v", err)
	}
}

func getBlockInterfaceInputRules() (r rules.RuleSet) {
	if *blockInterfaceInputCSV == "" {
		return
	}
	for _, iface := range strings.Split(*blockInterfaceInputCSV, ",") {
		r = append(r,
			fw.BlockInputFromInterface("tcp", fw.LinkString(iface)))
	}
	return
}

func getOpenPortRules() (r rules.RuleSet) {
	if *openPortsCSV == "" {
		return
	}
	for _, s := range strings.Split(*openPortsCSV, ",") {
		var port int
		var err error
		// Grab potentially one more token than expected to allow early failure.
		switch s2 := strings.SplitN(s, "/", 4); true {
		case len(s2) >= 2:
			port, err = strconv.Atoi(s2[1])
			if err != nil {
				log.Fatalf("Flag \"-open_ports\" should be a CSV of (tcp|udp)/port pairs or (tcp|udp)/port/iface triples; got %q: %v", *openPortsCSV, err)
			}
			fallthrough
		case len(s2) == 2:
			r = append(r, fw.OpenPort(s2[0], port))
		case len(s2) == 3:
			r = append(r, fw.OpenPortOnInterface(s2[0], port, fw.LinkString(s2[2])))
		case len(s2) < 2 || len(s2) > 3:
			log.Fatalf("Flag \"-open_ports\" should be a CSV of (tcp|udp)/port pairs or (tcp|udp)/port/iface triples; got %q", *openPortsCSV)
		}
	}
	return
}

func setupHTTPHandlers(ctx context.Context, cfg fw.Config, httpCfg httpConfig, m *ha.MemberGroup) {
	metricsHandler, err := metrics.New(ctx, metrics.Config{
		UplinkName: cfg.Uplink().Name(),
	})
	if err != nil {
		log.Fatalf("Error setting up metrics: %v", err)
	}
	var mr func(m ha.Member)
	if m != nil {
		mr = m.Add
	}

	httpCfg.mux.Handle("/metrics", metricsHandler)
	httpCfg.mux.Handle("/healthz", health.New(ctx, mr))
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
