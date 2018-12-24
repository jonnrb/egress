package fw

import (
	"flag"
	"os/exec"

	"github.com/google/shlex"
	"go.jonnrb.io/egress/fw/rules"
	"go.jonnrb.io/egress/log"
)

var iptablesBin = flag.String("iptables.bin", "/sbin/iptables", "Path to iptables binary")

func applyRuleSet(rs rules.RuleSet) error {
	for _, r := range rs {
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
