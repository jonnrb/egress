package ha

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestControlLoop_DoNothing_Joins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := LeaderFollower{TrivialLeader{}, TrivialFollower{}}
	l, ctx := StartControlLoop(ctx, m)

	if err := l.StopAndWait(); err != context.Canceled {
		t.Errorf("expected err == context.Canceled; got err == %v", err)
	}
}

func TestControlLoop_SwitchesOnNotify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan string, 5)
	m := LeaderFollower{
		Leader: LeaderFunc(func(ctx context.Context, _ func(time.Duration) error) error {
			c <- "me"
			<-ctx.Done()
			return ctx.Err()
		}),
		Follower: FollowerFunc(func(ctx context.Context, leader string) error {
			c <- leader
			<-ctx.Done()
			return ctx.Err()
		}),
	}
	l, ctx := StartControlLoop(ctx, m)

	l.BecomeLeader(nil)
	l.BecomeFollower("A")
	l.BecomeFollower("B")
	l.BecomeLeader(nil)
	l.BecomeFollower("C")

	if err := l.StopAndWait(); err != context.Canceled {
		t.Errorf("expected err == context.Canceled; got err == %v", err)
	}

	s := strings.Join([]string{<-c, <-c, <-c, <-c, <-c}, " ")
	if s != "me A B me C" {
		t.Errorf("expected \"me A B me C\"; got: %q", s)
	}
}

func TestControlLoop_ReturnsErrorFromLeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	didNilError := false
	m := LeaderFollower{
		Leader: LeaderFunc(func(ctx context.Context, _ func(time.Duration) error) error {
			if didNilError {
				return errors.New("something bad")
			}
			didNilError = true
			return nil
		}),
		Follower: TrivialFollower{},
	}

	l, _ := StartControlLoop(ctx, m)
	l.BecomeLeader(nil)
	if err := l.StopAndWait(); err != nil {
		t.Errorf("expected err == nil; got err == %v", err)
	}

	l, _ = StartControlLoop(ctx, m)
	l.BecomeLeader(nil)
	if err := l.StopAndWait(); err == nil || err.Error() != "something bad" {
		t.Errorf("expected err == errors.New(\"something bad\"); got err == %v", err)
	}
}

func TestControlLoop_ReturnsErrorFromFollower(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	didNilError := false
	m := LeaderFollower{
		Leader: TrivialLeader{},
		Follower: FollowerFunc(func(ctx context.Context, leader string) error {
			if didNilError {
				return errors.New("something bad")
			}
			didNilError = true
			return nil
		}),
	}

	l, _ := StartControlLoop(ctx, m)
	l.BecomeFollower("")
	if err := l.StopAndWait(); err != nil {
		t.Errorf("expected err == nil; got err == %v", err)
	}

	l, _ = StartControlLoop(ctx, m)
	l.BecomeFollower("")
	if err := l.StopAndWait(); err == nil || err.Error() != "something bad" {
		t.Errorf("expected err == errors.New(\"something bad\"); got err == %v", err)
	}
}

func TestControlLoop_ReturnsCanceledFromSubContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var n int32

	m := LeaderFollower{
		Leader: LeaderFunc(func(ctx context.Context, _ func(time.Duration) error) error {
			atomic.AddInt32(&n, 1)
			ctx, cancel := context.WithCancel(ctx)
			cancel()
			return ctx.Err()
		}),
		Follower: TrivialFollower{},
	}

	l, _ := StartControlLoop(ctx, m)
	l.BecomeLeader(nil)
	l.BecomeLeader(nil)
	l.BecomeLeader(nil)

	if err := l.StopAndWait(); err != context.Canceled {
		t.Errorf("expected err == context.Canceled; got err == %v", err)
	}
	if n != 3 {
		t.Errorf("expected n == 3; got n == %v", n)
	}
}
