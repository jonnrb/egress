package dhcp

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"go.jonnrb.io/egress/log"
)

type LeaseStore interface {
	Get(ctx context.Context) (Lease, error)
	Put(ctx context.Context, l Lease) error
}

type Lease struct {
	LeasedIP   net.IP
	SubnetMask int
	GatewayIP  net.IP
	ServerIP   net.IP

	StartTime   time.Time
	Duration    time.Duration
	RenewAfter  time.Duration
	RebindAfter time.Duration
}

type rawLease nclient4.Lease

func newRawLease(r *nclient4.Lease, err error) (*rawLease, error) {
	if err != nil {
		return nil, fmt.Errorf("dhcp: error getting lease: %w", err)
	}
	return (*rawLease)(r), nil
}

const defaultLeaseTime = 24 * time.Hour

func defaultRenewFactor(d time.Duration) time.Duration {
	return d / 2
}
func defaultRebindFactor(d time.Duration) time.Duration {
	return 9 * d / 10
}

func (r rawLease) ToLease() (l Lease, err error) {
	l.LeasedIP, err = r.LeasedIP()
	if err != nil {
		return
	}
	l.SubnetMask, err = r.SubnetMaskLen()
	if err != nil {
		return
	}
	l.ServerIP, err = r.ServerIP()
	if err != nil {
		return
	}
	l.GatewayIP, err = r.GatewayIP()
	if err != nil {
		l.GatewayIP = l.ServerIP
		err = nil
	}
	l.StartTime = r.CreationTime
	l.Duration, err = r.LeaseTime(defaultLeaseTime)
	if err != nil {
		return
	}
	l.RenewAfter, err = r.RenewalTime(defaultRenewFactor(l.Duration))
	if err != nil {
		return
	}
	l.RebindAfter, err = r.RebindingTime(defaultRebindFactor(l.Duration))
	if err != nil {
		return
	}
	return
}

func (r rawLease) LeasedIP() (net.IP, error) {
	o, a := r.Offer.YourIPAddr, r.ACK.YourIPAddr
	switch {
	case (o == nil || o.IsUnspecified()) && (a == nil || a.IsUnspecified()):
		return nil, fmt.Errorf("dhcp: no offered rawLease IP")
	case o.Equal(a):
		return a, nil
	case o == nil || o.IsUnspecified():
		log.Warningf("dhcp: offer had no rawLease IP; using rawLease IP from ack")
		return a, nil
	case a == nil || a.IsUnspecified():
		log.Warningf("dhcp: offer had no rawLease IP; using rawLease IP from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack lease IPs differed (offer=%v, ack=%v); using lease IP from ack", o, a)
		return a, nil
	}
}

func (r rawLease) SubnetMask() (net.IPMask, error) {
	o, a := r.Offer.SubnetMask(), r.ACK.SubnetMask()
	switch {
	case o == nil && a == nil:
		return nil, fmt.Errorf("dhcp: no offered subnet mask")
	case bytes.Equal(o, a): // Play fast and loose with bytes.Equal() :)
		return a, nil
	case o == nil:
		log.Warningf("dhcp: offer had no subnet mask; using subnet mask from ack")
		return a, nil
	case a == nil:
		log.Warningf("dhcp: offer had no subnet mask; using subnet mask from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack subnet masks differed (offer=%v, ack=%v); using subnet mask from ack", o, a)
		return a, nil
	}
}

func (r rawLease) SubnetMaskLen() (int, error) {
	m, err := r.SubnetMask()
	if err != nil {
		return 0, err
	}
	ones, bits := m.Size()
	if bits != 32 {
		return 0, fmt.Errorf("dhcp: got subnet mask with %v (!= 32) bits", bits)
	}
	return ones, nil
}

func (r rawLease) GatewayIP() (net.IP, error) {
	o, a := r.Offer.GatewayIPAddr, r.ACK.GatewayIPAddr
	switch {
	case (o == nil || o.IsUnspecified()) && (a == nil || a.IsUnspecified()):
		return nil, fmt.Errorf("dhcp: no offered gateway IP")
	case o.Equal(a):
		return a, nil
	case o == nil || o.IsUnspecified():
		log.Warningf("dhcp: offer had no gateway IP; using gateway IP from ack")
		return a, nil
	case a == nil || a.IsUnspecified():
		log.Warningf("dhcp: offer had no gateway IP; using gateway IP from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack gateway IPs differed (offer=%v, ack=%v); using gateway IP from ack", o, a)
		return a, nil
	}
}

func (r rawLease) ServerIP() (net.IP, error) {
	o, a := r.Offer.ServerIPAddr, r.ACK.ServerIPAddr
	switch {
	case (o == nil || o.IsUnspecified()) && (a == nil || a.IsUnspecified()):
		return nil, fmt.Errorf("dhcp: no offered server IP")
	case o.Equal(a):
		return a, nil
	case o == nil || o.IsUnspecified():
		log.Warningf("dhcp: offer had no server IP; using server IP from ack")
		return a, nil
	case a == nil || a.IsUnspecified():
		log.Warningf("dhcp: offer had no server IP; using server IP from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack server IPs differed (offer=%v, ack=%v); using server IP from ack", o, a)
		return a, nil
	}
}

func (r rawLease) LeaseTime(def time.Duration) (time.Duration, error) {
	o, a := r.Offer.IPAddressLeaseTime(def), r.ACK.IPAddressLeaseTime(def)
	switch {
	case o == 0 && a == 0:
		return 0, fmt.Errorf("dhcp: no offered lease time")
	case o == a:
		return a, nil
	case o == 0:
		log.Warningf("dhcp: offer had no lease time; using lease time from ack")
		return a, nil
	case a == 0:
		log.Warningf("dhcp: offer had no lease time; using lease time from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack lease times differed (offer=%v, ack=%v); using lease time from ack", o, a)
		return a, nil
	}
}

func (r rawLease) RenewalTime(def time.Duration) (time.Duration, error) {
	o, a := r.Offer.IPAddressRenewalTime(def), r.ACK.IPAddressRenewalTime(def)
	switch {
	case o == 0 && a == 0:
		return 0, fmt.Errorf("dhcp: no offered renewal time")
	case o == a:
		return a, nil
	case o == 0:
		log.Warningf("dhcp: offer had no renewal time; using renewal time from ack")
		return a, nil
	case a == 0:
		log.Warningf("dhcp: offer had no renewal time; using renewal time from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack renewal times differed (offer=%v, ack=%v); using renewal time from ack", o, a)
		return a, nil
	}
}

func (r rawLease) RebindingTime(def time.Duration) (time.Duration, error) {
	o, a := r.Offer.IPAddressRebindingTime(def), r.ACK.IPAddressRebindingTime(def)
	switch {
	case o == 0 && a == 0:
		return 0, fmt.Errorf("dhcp: no offered rebinding time")
	case o == a:
		return a, nil
	case o == 0:
		log.Warningf("dhcp: offer had no rebinding time; using rebinding time from ack")
		return a, nil
	case a == 0:
		log.Warningf("dhcp: offer had no rebinding time; using rebinding time from offer")
		return o, nil
	default:
		log.Warningf("dhcp: offer and ack rebinding times differed (offer=%v, ack=%v); using rebinding time from ack", o, a)
		return a, nil
	}
}
