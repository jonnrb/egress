package fw

import "go.jonnrb.io/egress/fw/rules"

// Appends extra rules to the extra rules already present on cfg.
func WithExtraRules(cfg Config, extraRules []rules.Rule) Config {
	return extraRulesDelegator{cfg, extraRules}
}

type extraRulesDelegator struct {
	Config
	extraRules []rules.Rule
}

func (c extraRulesDelegator) ExtraRules() rules.RuleSet {
	r := append([]rules.Rule(nil), c.Config.ExtraRules()...)
	return append(r, c.extraRules...)
}
