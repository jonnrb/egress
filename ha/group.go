package ha

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type MemberGroup struct {
	mu sync.Mutex

	ms []Member

	eg *errgroup.Group

	ctx               context.Context
	isLeader          bool
	isLeaseAcceptable func(expiredToleration time.Duration) error
	otherLeader       *string
}

func (g *MemberGroup) Lead(
	ctx context.Context,
	isLeaseAcceptable func(expiredToleration time.Duration) error,
) error {
	waitUntilCanceled := g.becomeLeader(ctx, isLeaseAcceptable)

	waitUntilCanceled()
	return g.wait()
}

func (g *MemberGroup) Follow(ctx context.Context, leader string) error {
	waitUntilCanceled := g.startFollowing(ctx, leader)

	waitUntilCanceled()
	return g.wait()
}

func (g *MemberGroup) Add(m Member) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.ms = append(g.ms, m)

	switch {
	case g.isLeader:
		ctx, isLeaseAcceptable := g.ctx, g.isLeaseAcceptable
		g.eg.Go(func() error {
			return m.Lead(ctx, isLeaseAcceptable)
		})
	case g.otherLeader != nil:
		ctx, leader := g.ctx, *g.otherLeader
		g.eg.Go(func() error {
			return m.Follow(ctx, leader)
		})
	}
}

func (g *MemberGroup) becomeLeader(
	ctx context.Context,
	isLeaseAcceptable func(expiredToleration time.Duration) error,
) func() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.eg, g.ctx = errgroup.WithContext(ctx)
	ctx = g.ctx
	g.isLeader = true
	g.isLeaseAcceptable = isLeaseAcceptable

	for i := range g.ms {
		m := g.ms[i]
		g.eg.Go(func() error {
			return m.Lead(ctx, isLeaseAcceptable)
		})
	}

	return func() {
		<-ctx.Done()
	}
}

func (g *MemberGroup) startFollowing(ctx context.Context, leader string) func() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.eg, g.ctx = errgroup.WithContext(ctx)
	ctx = g.ctx
	g.otherLeader = &leader

	for i := range g.ms {
		m := g.ms[i]
		g.eg.Go(func() error {
			return m.Follow(ctx, leader)
		})
	}

	return func() {
		<-ctx.Done()
	}
}

func (g *MemberGroup) wait() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	err := g.eg.Wait()
	g.eg, g.ctx, g.isLeader, g.isLeaseAcceptable, g.otherLeader = nil, nil, false, nil, nil
	return err
}
