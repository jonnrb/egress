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
			vaddr.ActiveFunc(a.runWithHWAddr),
		},
	}.Run(ctx)
}

func (a VAddr) runWithHWAddr(ctx context.Context) error {
	for {
		log.V(2).Infof("Requesting a DHCP lease on %v", a.Link.Name())

		l, err := a.getLease(ctx)
		if err != nil {
			return err
		}

		log.V(2).Infof("Got DHCP lease for %v: %+v", a.Link.Name(), l)

		err = a.holdLease(ctx, l)
		if err != nil {
			return err
		}

		log.V(2).Infof("Lease for %v expired", a.Link.Name())
	}
}

// Races requesting a lease with getting an existing lease from the LeaseStore.
func (a VAddr) getLease(ctx context.Context) (l Lease, err error) {
	type LeaseGetter func(ctx context.Context) (Lease, error)
	gs := []LeaseGetter{a.requestLease}
	if a.LeaseStore != nil {
		gs = append(gs, a.LeaseStore.Get)
	}

	eg, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c := make(chan Lease, len(gs))
	for _, g := range gs {
		eg.Go(func() error {
			if l, err := g(ctx); err != nil {
				return err
			} else {
				c <- l
				return nil
			}
		})
	}

	for range gs {
		select {
		case l = <-c:
			// Discard if the lease would have entered the renew phase by now.
			// This is somewhat sensible to do at its face, but also consider
			// that getLease() runs immediately after the previous lease, which
			// may have been saved to the LeaseStore, had entered the renew
			// phase. This avoids the LeaseStore racing a DHCP request in a loop
			// until the lease actually expires.
			if l.StartTime.Add(l.RenewAfter).Before(time.Now()) {
				continue
			}
			cancel()
			eg.Wait()
			return
		case <-ctx.Done():
			err = eg.Wait()
			return
		}
	}
	err = fmt.Errorf(
		"dhcp: could not get unexpired lease; last lease seen: %+v", l)
	return
}

func (a VAddr) requestLease(ctx context.Context) (l Lease, err error) {
	c, err := nclient4.New(
		a.Link.Name(),
		nclient4.WithHWAddr(a.HWAddr),
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
func (a VAddr) holdLease(ctx context.Context, l Lease) error {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)

	if a.LeaseStore != nil {
		eg.Go(func() error {
			err := a.LeaseStore.Put(ctx, l)
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

	eg.Go(func() error {
		err := a.applyLease(l).Run(ctx)
		if err != nil {
			return fmt.Errorf("dhcp: error applying lease: %w", err)
		}
		return nil
	})

	return eg.Wait()
}

func (a VAddr) applyLease(l Lease) vaddr.Active {
	return vaddr.Suite{
		Wrappers: []vaddr.Wrapper{
			&vaddrutil.IP{
				Link: a.Link,
				Addr: leasedAddr(l),
			},
			&vaddrutil.DefaultRoute{
				Link: a.Link,
				GW:   gwAddr(l),
			},
			&vaddrutil.GratuitousARP{
				Link:   a.Link,
				HWAddr: a.HWAddr,
				IP:     l.LeasedIP,
			},
		},
		Actives: []vaddr.Active{vaddr.ActiveWaiter{}},
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
