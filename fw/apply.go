package fw

import (
	"fmt"

	"go.jonnrb.io/egress/fw/rules"
)

func Apply(cfg Config) error {
	return applyRuleSet(
		rules.NewBuilder().
			Apply(rules.BaseRules).
			Apply(addFlatNetworkForwarding(cfg)).
			Add(50, []rules.Rule{
				forward(cfg.LAN(), cfg.Uplink()),
				masquerade(cfg.Uplink()),
			}).
			Add(60, cfg.ExtraRules()).
			Build())
}

func addFlatNetworkForwarding(cfg Config) func(rb rules.RuleSetBuilder) {
	var rs rules.RuleSet
	for _, s := range cfg.FlatNetworks() {
		rs = append(rs, forwardToSubnet(cfg.LAN(), s.Link, s.Subnet))
	}

	return func(rb rules.RuleSetBuilder) {
		rb.Add(50, rs)
	}
}

func forward(a, b Link) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t filter -A fw-interfaces -j ACCEPT -i %v -o %v",
		a.Name(), b.Name()))
}

func forwardToSubnet(a, b Link, dst string) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t filter -A fw-interfaces -j ACCEPT -d %v -i %v -o %v",
		dst, a.Name(), b.Name()))
}

func masquerade(l Link) rules.Rule {
	return rules.Rule(fmt.Sprintf(
		"-t nat -A POSTROUTING -j MASQUERADE -o %v",
		l.Name()))
}

func OpenPort(proto, port string) rules.Rule {
	switch proto {
	case "tcp":
	case "udp":
	default:
		panic(fmt.Sprintf("invalid proto: %q", proto))
	}

	return rules.Rule(fmt.Sprintf(
		"-I in-%s -j ACCEPT -p %s --dport %s",
		proto, proto, port))
}
