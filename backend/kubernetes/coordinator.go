package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.jonnrb.io/egress/backend/kubernetes/client"
	"go.jonnrb.io/egress/backend/kubernetes/metadata"
	"go.jonnrb.io/egress/ha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type Coordinator struct {
	// The name of the lock and the namespace to store it in.
	LockName      string
	LockNamespace string

	// These are as defined in
	// k8s.io/client-go/tools/leaderelection.LeaderElectionConfig.
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

func (c *Coordinator) Run(ctx context.Context, m ha.Member) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s, err := c.setupLeaderElector(ctx, m)

	for ctx.Err() == nil && err == nil {
		err = s.runOnce(ctx, m)
	}

	switch {
	case err != nil:
		return err
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		return nil
	}
}

type leState struct {
	runOnceInternal func(ctx context.Context)

	mu   sync.Mutex
	loop *ha.ControlLoop
}

func (c *Coordinator) setupLeaderElector(ctx context.Context, m ha.Member) (*leState, error) {
	name, err := metadata.GetPodName()
	if err != nil {
		return nil, fmt.Errorf("failed to get pod name: %w", err)
	}

	var s leState

	var le *leaderelection.LeaderElector
	le, err = c.createLeaderElector(
		name,
		func(ctx context.Context) {
			s.getLoop().BecomeLeader(le.Check)
		},
		func(leader string) {
			if leader != name {
				s.getLoop().BecomeFollower(leader)
			}
		},
	)
	if err != nil {
		return nil, err
	}

	s.runOnceInternal = le.Run
	return &s, nil
}

func (s *leState) runOnce(ctx context.Context, m ha.Member) error {
	ctx = s.newLoop(ctx, m)
	s.runOnceInternal(ctx)
	return s.loop.StopAndWait()
}

func (s *leState) newLoop(ctx context.Context, m ha.Member) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loop, ctx = ha.StartControlLoop(ctx, m)
	return ctx
}

func (s *leState) getLoop() *ha.ControlLoop {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loop
}

func (c *Coordinator) createLeaderElector(
	name string,
	onStartedLeading func(ctx context.Context),
	onNewLeader func(leader string),
) (*leaderelection.LeaderElector, error) {
	restCfg, err := client.Get()
	if err != nil {
		return nil, err
	}

	cli, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      c.LockName,
			Namespace: c.LockNamespace,
		},
		Client: cli.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: name,
		},
	}

	cfg := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,

		LeaseDuration: c.LeaseDuration,
		RenewDeadline: c.RenewDeadline,
		RetryPeriod:   c.RetryPeriod,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStartedLeading,
			OnStoppedLeading: func() {},
			OnNewLeader:      onNewLeader,
		},
	}

	return leaderelection.NewLeaderElector(cfg)
}
