package rules // import "go.jonnrb.io/egress/fw/rules"

import "sort"

// A rule, probably of the iptables variety, although nothing about the value is
// assumed in this package.
type Rule string

// A set of rules to be applied in order.
type RuleSet []Rule

// Maps a set of priorities to rules at those priorities. Some rules exported
// here will have "special" priorities that can be depended upon. Rules should
// be applied from the lowest numbered slice (highest priority) to the highest
// numbered slice (highest priority).
type RuleSetBuilder map[int][]Rule

func NewBuilder() RuleSetBuilder {
	return RuleSetBuilder(make(map[int][]Rule))
}

func (b RuleSetBuilder) Add(priority int, rules RuleSet) RuleSetBuilder {
	b[priority] = append(b[priority], rules...)
	return b
}

func (b RuleSetBuilder) Apply(mutate func(b RuleSetBuilder)) RuleSetBuilder {
	mutate(b)
	return b
}

func (b RuleSetBuilder) Build() RuleSet {
	var ks []int
	for k := range b {
		ks = append(ks, k)
	}
	sort.Ints(ks)

	var rs []Rule
	for _, k := range ks {
		rs = append(rs, b[k]...)
	}
	return rs
}
