package internal

import (
	cni "github.com/K8sNetworkPlumbingWG/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/ericchiang/k8s"
	metav1 "github.com/ericchiang/k8s/apis/meta/v1"
)

func init() {
	k8s.Register("k8s.cni.cncf.io", "v1", "network-attachment-definitions", true, &netAttachDef{})
}

// Creates a (context.Context-enabled) kubernetes client configured from the
// environment.
func GetK8sClient() (*k8s.Client, error) {
	return k8s.NewInClusterClient()
}

type netAttachDef cni.NetworkAttachmentDefinition

func (net *netAttachDef) GetMetadata() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{}
}
