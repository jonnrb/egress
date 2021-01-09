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

func InstallError(g *metadata.Getter, err error) Stubbed {
	s := Stubbed{g, *g}
	*g = func() (string, error) {
		return "", err
	}
	return s
}

func InstallPanic(g *metadata.Getter, v interface{}) Stubbed {
	s := Stubbed{g, *g}
	*g = func() (string, error) {
		panic(v)
	}
	return s
}

func (s Stub) Uninstall() {
	for _, sd := range s {
		*sd.g = sd.p
	}
}
