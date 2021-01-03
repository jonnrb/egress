package metadata

import (
	"io/ioutil"
	"sync"
	"time"
)

var (
	attachments  = PodInfo{path: "/etc/podinfo/attachments"}
	podName      = PodInfo{path: "/etc/podinfo/pod-name"}
	podNamespace = PodInfo{path: "/var/run/secrets/kubernetes.io/serviceaccount/namespace"}
)

var (
	GetAttachments  = attachments.Get
	GetPodName      = podName.Get
	GetPodNamespace = podNamespace.Get
)

type PodInfo struct {
	path string

	once sync.Once
	val  string
	err  error
}

func (p *PodInfo) Get() (value string, err error) {
	p.once.Do(func() {
		p.val, p.err = getPodInfo(p.path)
	})
	return p.val, p.err
}

func getPodInfo(path string) (value string, err error) {
	// Do a backoff on the path.
	for backoff := 100 * time.Millisecond; backoff < time.Second; backoff = backoff * 2 {
		value, err = readFile(path)
		if err == nil {
			return
		}
		time.Sleep(backoff)
	}
	return
}

func readFile(f string) (v string, err error) {
	var b []byte
	b, err = ioutil.ReadFile(f)
	v = string(b)
	return
}
