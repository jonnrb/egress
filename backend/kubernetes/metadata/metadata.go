package metadata

import (
	"io/ioutil"
	"sync"
	"time"
)

var (
	attachments  = podInfo{path: "/etc/podinfo/attachments"}
	podName      = podInfo{path: "/etc/podinfo/pod-name"}
	podNamespace = podInfo{path: "/var/run/secrets/kubernetes.io/serviceaccount/namespace"}
)

type Getter func() (val string, err error)

var (
	GetAttachments  = Getter(attachments.Get)
	GetPodName      = Getter(podName.Get)
	GetPodNamespace = Getter(podNamespace.Get)
)

type podInfo struct {
	path string

	once sync.Once
	val  string
	err  error
}

func (p *podInfo) Get() (value string, err error) {
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
