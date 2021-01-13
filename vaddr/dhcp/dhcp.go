package dhcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"go.jonnrb.io/egress/fw"
	"go.jonnrb.io/egress/log"
	"go.jonnrb.io/egress/vaddr"
	"go.jonnrb.io/egress/vaddr/vaddrutil"
	"golang.org/x/sync/errgroup"
)

type VAddr struct {
	HWAddr     net.HardwareAddr
	Link       fw.Link
	LeaseStore LeaseStore
}

func (a VAddr) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	return vaddr.Suite{
		Wrappers: []vaddr.Wrapper{
			&vaddrutil.VirtualMAC{
				Link: a.Link,
				Addr: a.HWAddr,
			},
		},
		Actives: []vaddr.Active{
			&vaddrState{addr: &a},
		},
	}.Run(ctx)
}

type vaddrState struct {
	addr        *VAddr
	curLease    *Lease
	activeVAddr vaddr.Wrapper
}

func (s *vaddrState) Run(ctx context.Context) error {
	defer func() {
		if s.activeVAddr != nil {
			s.activeVAddr.Stop()
		}
	}()

	for {
		log.V(2).Infof("Requesting a DHCP lease on %v", s.addr.Link.Name())

		l, err := s.getLease(ctx)
		if err != nil {
			return err
		}

		log.V(2).Infof("Got DHCP lease for %v: %+v", s.addr.Link.Name(), l)

		if l == nil {
			panic("dhcp: impossible nil lease")
		}
		err = s.maybeBind(*l)
		if err != nil {
			return fmt.Errorf("dhcp: error rebinding: %w", err)
		}

		err = s.holdLease(ctx)
		if err != nil {
			return err
		}

		log.V(2).Infof("Lease for %v expired", s.addr.Link.Name())
	}
}

// Races requesting a lease with getting an existing lease from the LeaseStore.
func (s *vaddrState) getLease(ctx context.Context) (*Lease, error) {
	var (
		requestLeaseChan = make(chan Lease, 1)
		requestLeaseDone = false
		leaseStoreChan   = make(chan Lease, 1)
		leaseStoreDone   = false
	)

	eg, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		l, err := s.requestLease(ctx)
		requestLeaseChan <- l
		return err
	})

	if s.addr.LeaseStore != nil {
		eg.Go(func() error {
			l, err := s.addr.LeaseStore.Get(ctx)
			leaseStoreChan <- l
			return err
		})
	} else {
		leaseStoreDone = true
	}

	for !requestLeaseDone && !leaseStoreDone {
		select {
		case l := <-requestLeaseChan:
			// Discard if the lease would have entered the renew phase by now.
			// This is somewhat sensible to do at its face, but also consider
			// that getLease() runs immediately after the previous lease, which
			// may have been saved to the LeaseStore, had entered the renew
			// phase. This avoids the LeaseStore racing a DHCP request in a loop
			// until the lease actually expires.
			if l.StartTime.Add(l.RenewAfter).Before(time.Now()) {
				requestLeaseDone = true
				continue
			}

			cancel()
			eg.Wait()

			select {
			case newLease := <-requestLeaseChan:
				renewTime := newLease.StartTime.Add(newLease.RenewAfter)
				if !renewTime.Before(time.Now()) {
					// Favor a lease just requested. The server would probably
					// expect us to use it...
					l = newLease
				}
			default:
			}

			return &l, nil
		case l := <-requestLeaseChan:
			if l.StartTime.Add(l.RenewAfter).Before(time.Now()) {
				requestLeaseDone = true
				continue
			}
			cancel()
			eg.Wait()
			return &l, nil
		case <-s.timeToRebind():
			s.unbind()
			continue
		case <-ctx.Done():
			return nil, eg.Wait()
		}
	}
	return nil, fmt.Errorf(
		"dhcp: could not get unexpired lease; last lease seen: %+v", s.curLease)
}

func (s *vaddrState) timeToRebind() <-chan time.Time {
	if s.curLease == nil {
		return nil
	}
	rebindTime := s.curLease.StartTime.Add(s.curLease.RebindAfter)
	return time.After(time.Until(rebindTime))
}

func (s *vaddrState) maybeBind(l Lease) error {
	netwiseSame := s.curLease != nil &&
		s.curLease.LeasedIP.Equal(l.LeasedIP) &&
		s.curLease.SubnetMask == l.SubnetMask &&
		s.curLease.GatewayIP.Equal(l.GatewayIP)

	if netwiseSame &&
		s.activeVAddr != nil &&
		s.curLease.StartTime.After(l.StartTime) {
		return nil
	}

	return s.bind(l)
}

func (s *vaddrState) bind(l Lease) error {
	err := s.unbind()
	if err != nil {
		s.curLease = nil
		s.activeVAddr = nil
		return err
	}
	s.curLease = &l
	s.activeVAddr = s.vaddrForLease(l)
	return s.activeVAddr.Start()
}

func (s *vaddrState) unbind() error {
	s.curLease = nil
	if s.activeVAddr == nil {
		return nil
	}
	err := s.activeVAddr.Stop()
	s.activeVAddr = nil
	return err
}

func (s *vaddrState) requestLease(ctx context.Context) (l Lease, err error) {
	c, err := nclient4.New(
		s.addr.Link.Name(),
		nclient4.WithHWAddr(s.addr.HWAddr),
		nclient4.WithRetry(45),
		nclient4.WithSummaryLogger())
	if err != nil {
		err = fmt.Errorf("dhcp: could not create dhcp client: %w", err)
		return
	}
	defer c.Close()

	// There would be a multitude of good places to issue a DHCP release if this
	// fails, but releases are no more than an optimization and floating a
	// virtual MAC address effectively makes releasing unnecessary. If we ever
	// did rely on releasing on failure, we'd get screwed at some point since
	// we can't atomically acquire a lease while logging that we've acquired the
	// lease.
	rl, err := newRawLease(c.Request(ctx))
	if err != nil {
		err = fmt.Errorf("dhcp: could not get lease from network: %w", err)
		return
	}

	return rl.ToLease()
}

// Holds the lease and potentially writes the lease to a LeaseStore. Returns nil
// when the lease should be renewed.
func (s *vaddrState) holdLease(ctx context.Context) error {
	if s.curLease == nil {
		panic("dhcp: can't have nil curLease here")
	}
	l := *s.curLease

	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)

	if s.addr.LeaseStore != nil {
		eg.Go(func() error {
			err := s.addr.LeaseStore.Put(ctx, l)
			if err != nil {
				return fmt.Errorf(
					"dhcp: error putting lease into lease store: %w", err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		select {
		case <-time.After(l.RenewAfter - time.Now().Sub(l.StartTime)):
			// No point in waiting on LeaseStore.Put() if the lease needs to be
			// renewed now.
			cancel()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	return eg.Wait()
}

func (s *vaddrState) vaddrForLease(l Lease) vaddr.Wrapper {
	return &vaddr.CombinedWrappers{
		Wrappers: []vaddr.Wrapper{
			&vaddrutil.IP{
				Link: s.addr.Link,
				Addr: leasedAddr(l),
			},
			&vaddrutil.DefaultRoute{
				Link: s.addr.Link,
				GW:   gwAddr(l),
			},
			&vaddrutil.GratuitousARP{
				Link:   s.addr.Link,
				HWAddr: s.addr.HWAddr,
				IP:     l.LeasedIP,
			},
		},
	}
}

func leasedAddr(l Lease) fw.Addr {
	a, err := fw.ParseAddr(fmt.Sprintf("%s/%d", l.LeasedIP, l.SubnetMask))
	if err != nil {
		panic(fmt.Sprintf("dhcp: invalid lease %v", l))
	}
	return a
}

func gwAddr(l Lease) fw.Addr {
	a, err := fw.ParseAddr(l.GatewayIP.String())
	if err != nil {
		panic(fmt.Sprintf("dhcp: invalid lease %v", l))
	}
	return a
}
