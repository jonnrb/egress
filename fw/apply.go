package fw

import (
	"go.jonnrb.io/egress/fw/rules"
)

func Apply(cfg Config) error {
	return applyRuleSet(
		rules.NewBuilder().
			Apply(rules.BaseRules).
			Apply(addFlatNetworkForwarding(cfg)).
			Add(50, []rules.Rule{
				Forward(cfg.LAN(), cfg.Uplink()),
				Masquerade(cfg.Uplink()),
			}).
			Add(60, cfg.ExtraRules()).
			Build())
}

func addFlatNetworkForwarding(cfg Config) func(rb rules.RuleSetBuilder) {
	var rs rules.RuleSet
	for _, s := range cfg.FlatNetworks() {
		rs = append(rs, ForwardToSubnet(cfg.LAN(), s.Link, s.Subnet))
	}

	return func(rb rules.RuleSetBuilder) {
		rb.Add(50, rs)
	}
}
