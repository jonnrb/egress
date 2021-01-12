package leasestore

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"go.jonnrb.io/egress/backend/kubernetes/client"
	"go.jonnrb.io/egress/backend/kubernetes/metadata"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/vaddr/dhcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type LeaseStore struct {
	// The name of the ConfigMap to write a lease to.
	Name string

	// The namespace to write the ConfigMap into. By default, the ConfigMap will
	// be written to the namespace the active pod resides in.
	Namespace string

	Client kubernetes.Interface
}

func New() (*LeaseStore, error) {
	cli, err := client.Get()
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cli)
	if err != nil {
		return nil, err
	}
	return &LeaseStore{Client: cs}, nil
}

func (c *LeaseStore) Get(ctx context.Context) (l dhcp.Lease, err error) {
	cmi, err := c.configMapInterface()
	if err != nil {
		err = fmt.Errorf("leasestore: could not load client: %w", err)
		return
	}
	cm, err := cmi.Get(ctx, c.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Return an empty lease. This will never be valid since it will
			// appear to be waaaay in the past.
			err = nil
		}
		err = fmt.Errorf("leasestore: could not get lease: %w", err)
		return
	}
	d, ok := cm.BinaryData[configMapKey]
	if !ok {
		d = []byte(cm.Data[configMapKey])
	}
	return deserializeLease(d)
}

func (c *LeaseStore) Put(ctx context.Context, l dhcp.Lease) error {
	cmi, err := c.configMapInterface()
	if err != nil {
		return fmt.Errorf("leasestore: could not load client: %w", err)
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: c.Name},
		BinaryData: map[string][]byte{
			configMapKey: serializeLease(l),
		},
	}
	_, err = cmi.Create(ctx, &cm, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(
			"leasestore: could not create lease configmap: %w", err)
	}
	b, err := json.Marshal(corev1.ConfigMap{BinaryData: cm.BinaryData})
	if err != nil {
		panic(fmt.Sprintf(
			"leasestore: could not marshal configmap for lease: %+v", l))
	}
	_, err = cmi.Patch(
		ctx, c.Name, types.StrategicMergePatchType, b,
		metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf(
			"leasestore: could not patch existing lease configmap: %w", err)
	}
	return nil
}

func (c *LeaseStore) configMapInterface() (clientcorev1.ConfigMapInterface, error) {
	ns := c.Namespace
	if ns == "" {
		var err error
		ns, err = metadata.GetPodNamespace()
		if err != nil {
			return nil, err
		}
	}
	return c.Client.CoreV1().ConfigMaps(ns), nil
}

const configMapKey = "lease.json"

type serializableLease struct {
	LeasedIP    string    `json:"leasedIP"`
	GatewayIP   string    `json:"gatewayIP"`
	ServerIP    string    `json:"serverIP"`
	StartTime   time.Time `json:"startTime"`
	Duration    int       `json:"duration"`
	RenewAfter  int       `json:"renewAfter"`
	RebindAfter int       `json:"rebindAfter"`
}

func serializeLease(l dhcp.Lease) []byte {
	b, err := json.Marshal(
		serializableLease{
			LeasedIP:    fmt.Sprintf("%s/%d", l.LeasedIP, l.SubnetMask),
			GatewayIP:   l.GatewayIP.String(),
			ServerIP:    l.ServerIP.String(),
			StartTime:   l.StartTime,
			Duration:    int(l.Duration / time.Millisecond),
			RenewAfter:  int(l.RenewAfter / time.Millisecond),
			RebindAfter: int(l.RebindAfter / time.Millisecond),
		})
	if err != nil {
		panic(fmt.Sprintf("leasestore: could not marshal lease: %+v", l))
	}
	return b
}

func deserializeLease(d []byte) (l dhcp.Lease, err error) {
	var s serializableLease
	err = json.Unmarshal(d, &s)
	if err != nil {
		err = fmt.Errorf(
			"leasestore: could not unmarshal lease %q: %w", d, err)
		return
	}
	return parseLease(s)
}

func parseLease(s serializableLease) (l dhcp.Lease, err error) {
	leasedAddr, err := fw.ParseAddr(s.LeasedIP)
	if err != nil {
		err = fmt.Errorf(
			"leasestore: %q is not a valid IP: %w", s.LeasedIP, err)
		return
	}
	l.LeasedIP = leasedAddr.IP
	l.SubnetMask, _ = leasedAddr.Mask.Size()
	l.GatewayIP = net.ParseIP(s.GatewayIP)
	if l.GatewayIP == nil {
		err = fmt.Errorf("leasestore: %q is not a valid IP", s.GatewayIP)
		return
	}
	l.ServerIP = net.ParseIP(s.ServerIP)
	if l.ServerIP == nil {
		err = fmt.Errorf("leasestore: %q is not a valid IP", s.ServerIP)
		return
	}
	l.StartTime = s.StartTime
	l.Duration = time.Duration(s.Duration) * time.Millisecond
	l.RenewAfter = time.Duration(s.RenewAfter) * time.Millisecond
	l.RebindAfter = time.Duration(s.RebindAfter) * time.Millisecond
	return
}
