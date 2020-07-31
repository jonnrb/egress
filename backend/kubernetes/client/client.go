package client

import (
	"sync"

	"k8s.io/client-go/rest"
)

var (
	once sync.Once
	c    *rest.Config
	e    error
)

// Creates a kubernetes client configured from the environment.
func Get() (*rest.Config, error) {
	once.Do(func() {
		c, e = rest.InClusterConfig()
	})
	return c, e
}
