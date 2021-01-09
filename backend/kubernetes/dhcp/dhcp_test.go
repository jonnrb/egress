package kubernetesdhcp_test

import (
	"context"
	"testing"

	kubernetesdhcp "go.jonnrb.io/egress/backend/kubernetes/dhcp"
	"go.jonnrb.io/egress/vaddr/dhcp"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTodo(t *testing.T) {
	s := kubernetesdhcp.Lease{
		LeaseName: "test-lease",
		Client:    fake.NewSimpleClientset(),
	}

	l, err := s.Get(context.Background())

	if err != nil {
		t.Errorf("no lease should not error; got: %v", err)
	}
	if l != (dhcp.Lease{}) {
		t.Errorf("no lease should be an empty lease; got: %+v", l)
	}
}
