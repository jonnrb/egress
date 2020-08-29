package ha

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemberGroup_AddBefore(t *testing.T) {
	var g MemberGroup
	var n int32

	for i := 0; i < 5; i++ {
		g.Add(LeaderFollower{
			Leader: LeaderFunc(func(_ context.Context, _ func(_ time.Duration) error) error {
				atomic.AddInt32(&n, 1)
				return nil
			}),
			Follower: TrivialFollower{},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := g.Lead(ctx, nil)

	if err != nil {
		t.Errorf("expected err == nil; got err == %v", err)
	}
	if n != 5 {
		t.Errorf("expected n == 5; got n == %v", n)
	}
}

func TestMemberGroup_ErrorCancelsOthers(t *testing.T) {
	var g MemberGroup
	var n int32

	g.Add(LeaderFollower{
		Leader: LeaderFunc(func(ctx context.Context, _ func(_ time.Duration) error) error {
			return errors.New("something bad")
		}),
		Follower: TrivialFollower{},
	})
	for i := 0; i < 5; i++ {
		g.Add(LeaderFollower{
			Leader: LeaderFunc(func(ctx context.Context, _ func(_ time.Duration) error) error {
				<-ctx.Done()
				atomic.AddInt32(&n, 1)
				return nil
			}),
			Follower: TrivialFollower{},
		})
	}

	err := g.Lead(context.Background(), nil)

	if err == nil || err.Error() != "something bad" {
		t.Errorf("expected err == errors.New(\"something bad\"); got err == %v", err)
	}
	if n != 5 {
		t.Errorf("expected n == 5; got n == %v", n)
	}
}

func TestMemberGroup_AddDuring(t *testing.T) {
	var g MemberGroup
	var n int32

	during := make(chan struct{})
	g.Add(LeaderFollower{
		Leader: LeaderFunc(func(_ context.Context, _ func(_ time.Duration) error) error {
			atomic.AddInt32(&n, 1)
			close(during)
			return nil
		}),
		Follower: TrivialFollower{},
	})
	go func() {
		<-during
		g.Add(LeaderFollower{
			Leader: LeaderFunc(func(_ context.Context, _ func(_ time.Duration) error) error {
				atomic.AddInt32(&n, 1)
				return context.Canceled
			}),
			Follower: TrivialFollower{},
		})
	}()

	for i := 0; i < 5; i++ {
		g.Add(LeaderFollower{
			Leader: LeaderFunc(func(_ context.Context, _ func(_ time.Duration) error) error {
				atomic.AddInt32(&n, 1)
				return nil
			}),
			Follower: TrivialFollower{},
		})
	}

	err := g.Lead(context.Background(), nil)

	if err != context.Canceled {
		t.Errorf("expected err == context.Canceled; got err == %v", err)
	}
	if n != 7 {
		t.Errorf("expected n == 7; got n == %v", n)
	}
}
