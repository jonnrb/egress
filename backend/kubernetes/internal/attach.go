package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type Attachment struct {
	// The name of the CNI network attached to.
	Name string `json:"name"`

	// The name of the network interface in the pod.
	Interface string `json:"interface"`

	// IPs and MAC assigned by CNI to the interface.
	IPs []string `json:"ips"`
	MAC string   `json:"mac"`

	Default bool `json:"default"`

	DNS cnitypes.DNS `json:"dns"`
}

// Gets the networks this pod is attached to. Expects that the
// `metadata.annotations['k8s.v1.cni.cncf.io/networks-status']` fieldPath is
// exposed to this container via the downward API [1] at
// `/etc/podinfo/attachments`.
//
// [1] https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/
//
func GetAttachments() ([]Attachment, error) {
	const path = "/etc/podinfo/attachments"
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening %q: %v", path, err)
	}
	defer f.Close()
	return decodeNetworkStatus(f)
}

func decodeNetworkStatus(r io.Reader) ([]Attachment, error) {
	var nets []Attachment
	if err := json.NewDecoder(r).Decode(&nets); err != nil {
		return nil, err
	}
	return nets, nil
}
