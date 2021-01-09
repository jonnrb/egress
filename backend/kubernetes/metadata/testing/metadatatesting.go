package metadatatesting

import "go.jonnrb.io/egress/backend/kubernetes/metadata"

type Stubbed struct {
	g *metadata.Getter
	p metadata.Getter
}

type Stub []Stubbed

func Install(g *metadata.Getter, v string) Stubbed {
	s := Stubbed{g, *g}
	*g = func() (string, error) {
		return v, nil
	}
	return s
}

func (s Stub) Uninstall() {
	for _, sd := range s {
		*sd.g = sd.p
	}
}
