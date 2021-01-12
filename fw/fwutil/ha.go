package fwutil

import (
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/ha"
)

type ConfigHACoordinator interface {
	// If non-nil, the ha.Coordinator to use for running the fw in HA mode.
	HACoordinator() ha.Coordinator
}

func GetHACoordinator(c fw.Config) ha.Coordinator {
	hac, ok := c.(ConfigHACoordinator)
	if !ok {
		return nil
	}
	return hac.HACoordinator()
}
