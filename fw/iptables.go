package fw

import (
	"flag"
	"os/exec"

	"github.com/google/shlex"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/log"
)

var iptablesBin = flag.String("iptables.bin", "/sbin/iptables", "Path to iptables binary")

// Applies a set of iptables rules in order.
func ApplyRules(iptablesRules rules.RuleSet) error {
	for _, r := range iptablesRules {
		log.V(3).Infof("Applying rule %q", r)
		if err := runIptables(r); err != nil {
			return err
		}
	}
	return nil
}

func runIptables(rule rules.Rule) error {
	args, err := shlex.Split(string(rule))
	if err != nil {
		return err
	}
	return exec.Command(*iptablesBin, args...).Run()
}
