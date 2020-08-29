package ha

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

// Helps adapt systems that provide asynchronous notifications of leadership
// changes (*cough* *cough* Kubernetes *cough*) into mutually exclusive
// leader/follower states.
type ControlLoop struct {
	cancel func()
	nc     chan<- notification
	done   <-chan struct{}
	ec     <-chan error
}

func StartControlLoop(ctx context.Context, m Member) (*ControlLoop, context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	var (
		nc   = make(chan notification)
		done = make(chan struct{})
		ec   = make(chan error, 1)
	)

	go runControlLoop(ctx, cancel, m, nc, done, ec)

	l := &ControlLoop{
		cancel: cancel,
		nc:     nc,
		done:   done,
		ec:     ec,
	}

	return l, ctx
}

func (l *ControlLoop) BecomeLeader(isLeaseAcceptable func(expiredToleration time.Duration) error) {
	select {
	case <-l.done:
	case l.nc <- notification{isLeaseAcceptable: isLeaseAcceptable}:
	}
}

func (l *ControlLoop) BecomeFollower(leader string) {
	select {
	case <-l.done:
	case l.nc <- notification{leader: &leader}:
	}
}

func (l *ControlLoop) StopAndWait() error {
	l.cancel()
	return <-l.ec
}

type notification struct {
	leader            *string
	isLeaseAcceptable func(expiredToleration time.Duration) error
}

func runControlLoop(
	ctx context.Context,
	cancel func(),
	m Member,
	nc <-chan notification,
	done chan<- struct{},
	ec chan<- error,
) {
	cancelAndJoinTask := func() error {
		// Return this when no leadership changes have been sent.
		return ctx.Err()
	}

	defer close(done)
	defer close(ec)

	for {
		select {
		case <-ctx.Done():
			ec <- cancelAndJoinTask()
			return
		case n, ok := <-nc:
			if !ok {
				// We only ever include this since the default value of
				// notification is a valid leader state.
				panic("ha: nc shouldn't ever get closed (this is bad!)")
			}
			switch err := cancelAndJoinTask(); err {
			case nil, context.Canceled:
			default:
				ec <- err
				return
			}
			cancelAndJoinTask = runTask(ctx, cancel, func(ctx context.Context) error {
				if n.leader == nil {
					return m.Lead(ctx, n.isLeaseAcceptable)
				} else {
					return m.Follow(ctx, *n.leader)
				}
			})
		}
	}
}

func runTask(ctx context.Context, outerCancel func(), f func(ctx context.Context) error) func() error {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		switch err := f(ctx); err {
		case nil:
			return nil
		case context.Canceled:
			return err
		default:
			if outerCancel != nil {
				outerCancel()
			}
			return err
		}
	})

	return func() error {
		cancel()
		return eg.Wait()
	}
}
