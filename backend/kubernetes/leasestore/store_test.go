package leasestore_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.jonnrb.io/egress/backend/kubernetes/leasestore"
	"go.jonnrb.io/egress/backend/kubernetes/metadata"
	"go.jonnrb.io/egress/backend/kubernetes/metadata/metadatatesting"
	"go.jonnrb.io/egress/vaddr/dhcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func installMetadata() metadatatesting.Stub {
	return metadatatesting.Stub{
		metadatatesting.Install(&metadata.GetPodNamespace, "some-namespace"),
	}
}

func installErrorMetadata() metadatatesting.Stub {
	return metadatatesting.Stub{
		metadatatesting.InstallPanic(&metadata.GetPodNamespace, "no! bad!"),
	}
}

func checkEmptyLease(t *testing.T) func(dhcp.Lease, error) {
	return func(l dhcp.Lease, err error) {
		if err != nil {
			t.Errorf("no lease should not error; got: %v", err)
		} else if diff := cmp.Diff(l, dhcp.Lease{}); diff != "" {
			t.Errorf("no lease should be an empty lease; diff: %v", diff)
		}
	}
}

func checkUpdated(t *testing.T, l dhcp.Lease) func(dhcp.Lease, error) {
	return func(updated dhcp.Lease, err error) {
		if err != nil {
			t.Fatalf("should not error; got: %v", err)
		} else if diff := cmp.Diff(l, updated); diff != "" {
			t.Errorf("lease wasn't updated; diff: %v", diff)
		}
	}
}

func TestGetEmpty(t *testing.T) {
	t.Run("NamespaceFromMetadata", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name:   "my-lease",
			Client: fake.NewSimpleClientset(),
		}
		defer installMetadata().Uninstall()

		checkEmptyLease(t)(s.Get(context.Background()))
	})

	t.Run("NamespaceFromStruct", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name:      "my-lease",
			Namespace: "overriden-namespace",
			Client:    fake.NewSimpleClientset(),
		}
		defer installErrorMetadata().Uninstall()

		checkEmptyLease(t)(s.Get(context.Background()))
	})
}

func TestGetPresent(t *testing.T) {
	t.Run("NoJsonValue", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name: "my-lease",
			Client: fake.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-lease",
						Namespace: "some-namespace",
					},
					Data: map[string]string{},
				},
			),
		}
		defer installMetadata().Uninstall()

		_, err := s.Get(context.Background())

		if err == nil {
			t.Errorf("expected err != nil; got: %v", err)
		}
	})

	t.Run("EmptyJsonValue", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name: "my-lease",
			Client: fake.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-lease",
						Namespace: "some-namespace",
					},
					Data: map[string]string{"lease.json": ""},
				},
			),
		}
		defer installMetadata().Uninstall()

		_, err := s.Get(context.Background())

		if err == nil {
			t.Errorf("expected err != nil; got: %v", err)
		}
	})

	t.Run("BadJsonValue", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name: "my-lease",
			Client: fake.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-lease",
						Namespace: "some-namespace",
					},
					Data: map[string]string{
						"lease.json": `{
						  "leasedIP":    "",
						  "gatewayIP":   "",
						  "serverIP":    "",
						  "startTime":   "",
						  "duration":    "",
						  "renewAfter":  "",
						  "rebindAfter": ""
						}`,
					},
				},
			),
		}
		defer installMetadata().Uninstall()

		_, err := s.Get(context.Background())

		if err == nil {
			t.Errorf("expected err != nil; got: %v", err)
		}
	})

	t.Run("ParsesLease", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name: "my-lease",
			Client: fake.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-lease",
						Namespace: "some-namespace",
					},
					Data: map[string]string{
						"lease.json": `{
						  "leasedIP":    "10.11.11.17/24",
						  "gatewayIP":   "10.11.11.1",
						  "serverIP":    "10.11.11.32",
						  "startTime":   "2020-10-10T11:11:11Z",
						  "duration":    604800,
						  "renewAfter":  86400,
						  "rebindAfter": 86400
						}`,
					},
				},
			),
		}
		defer installMetadata().Uninstall()

		_, err := s.Get(context.Background())

		if err != nil {
			t.Errorf("expected err == nil; got: %v", err)
		}
	})
}

func TestPut(t *testing.T) {
	defer installErrorMetadata().Uninstall()

	t.Run("PreviouslyAbsent", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name:      "my-lease",
			Namespace: "some-namespace",
			Client:    fake.NewSimpleClientset(),
		}

		l := dhcp.Lease{
			LeasedIP:    net.IPv4(10, 11, 11, 17),
			SubnetMask:  24,
			GatewayIP:   net.IPv4(10, 11, 11, 1),
			ServerIP:    net.IPv4(10, 11, 11, 32),
			StartTime:   time.Date(2020, 10, 10, 11, 11, 11, 0, time.UTC),
			Duration:    7 * 24 * time.Hour,
			RenewAfter:  24 * time.Hour,
			RebindAfter: 24 * time.Hour,
		}
		err := s.Put(context.Background(), l)

		if err != nil {
			t.Errorf("expected err == nil; got: %v", err)
		}
		checkUpdated(t, l)(s.Get(context.Background()))
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		s := leasestore.LeaseStore{
			Name:      "my-lease",
			Namespace: "some-namespace",
			Client: fake.NewSimpleClientset(
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-lease",
						Namespace: "some-namespace",
					},
					Data: map[string]string{
						"lease.json": `{
						  "leasedIP":    "10.11.11.17/24",
						  "gatewayIP":   "10.11.11.1",
						  "serverIP":    "10.11.11.32",
						  "startTime":   "2020-10-09:11:11Z",
						  "duration":    604800,
						  "renewAfter":  86400,
						  "rebindAfter": 86400
						}`,
					},
				},
			),
		}

		l := dhcp.Lease{
			LeasedIP:    net.IPv4(10, 11, 11, 17),
			SubnetMask:  24,
			GatewayIP:   net.IPv4(10, 11, 11, 1),
			ServerIP:    net.IPv4(10, 11, 11, 32),
			StartTime:   time.Date(2020, 10, 10, 11, 11, 11, 0, time.UTC),
			Duration:    7 * 24 * time.Hour,
			RenewAfter:  24 * time.Hour,
			RebindAfter: 24 * time.Hour,
		}
		err := s.Put(context.Background(), l)

		if err != nil {
			t.Errorf("expected err == nil; got: %v", err)
		}
		checkUpdated(t, l)(s.Get(context.Background()))
	})
}
