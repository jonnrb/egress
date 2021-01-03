package metadata

import (
	"io/ioutil"
	"sync"
	"time"
)

var (
	attachments = PodInfo{name: "attachments"}
	podName     = PodInfo{name: "pod-name"}
)

var (
	GetAttachments = attachments.Get
	GetPodName     = podName.Get
)

type PodInfo struct {
	name string

	once sync.Once
	val  string
	err  error
}

func (p *PodInfo) Get() (value string, err error) {
	p.once.Do(func() {
		p.val, p.err = getPodInfo(p.name)
	})
	return p.val, p.err
}

func getPodInfo(name string) (value string, err error) {
	// The downward API standard-ish mount point.
	path := "/etc/podinfo/" + name

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
