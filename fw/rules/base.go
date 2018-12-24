package rules

// Adds a set of base rules to the builder. When this is used, priorities
// [0, 10) and [990, 1000) should be assumed reserved.
//
// TODO: Document these rules a bit.
//
func BaseRules(b RuleSetBuilder) {
	b.Add(0, policyRules).
		Add(1, baseChains).
		Add(999, rejections)
}

var policyRules = []Rule{
	Rule("-t filter -P INPUT DROP"),
	Rule("-t filter -P FORWARD DROP"),
	Rule("-t filter -N in-tcp"),
	Rule("-t filter -N in-udp"),
	Rule("-t filter -N fw-interfaces"),
	Rule("-t filter -N fw-open"),
}

var baseChains = []Rule{
	Rule("-t filter -A INPUT -j DROP -m state --state INVALID"),
	Rule("-t filter -A INPUT -j ACCEPT -m conntrack --ctstate RELATED,ESTABLISHED"),
	Rule("-t filter -A INPUT -j ACCEPT -i lo"),
	Rule("-t filter -A INPUT -j ACCEPT -p icmp --icmp-type 8 -m conntrack --ctstate NEW"),
	Rule("-t filter -A INPUT -j in-tcp -p tcp --tcp-flags FIN,SYN,RST,ACK SYN -m conntrack --ctstate NEW"),
	Rule("-t filter -A INPUT -j in-udp -p udp -m conntrack --ctstate NEW"),
	Rule("-t filter -A INPUT -j REJECT --reject-with icmp-proto-unreachable"),
	Rule("-t filter -A FORWARD -j ACCEPT -m conntrack --ctstate ESTABLISHED,RELATED"),
	Rule("-t filter -A FORWARD -j fw-interfaces"),
	Rule("-t filter -A FORWARD -j fw-open"),
	Rule("-t filter -A FORWARD -j REJECT --reject-with icmp-host-unreach"),
	Rule("-t nat -A PREROUTING -j ACCEPT -m conntrack --ctstate RELATED,ESTABLISHED"),
}

var rejections = []Rule{
	Rule("-t filter -A in-tcp -j REJECT -p tcp --reject-with tcp-reset"),
	Rule("-t filter -A in-udp -j REJECT -p udp --reject-with icmp-port-unreachable"),
}
