package ha

import (
	"context"
	"time"
)

type LeaderFunc func(ctx context.Context, isLeaseAcceptable func(expiredToleration time.Duration) error) error

func (m LeaderFunc) Lead(ctx context.Context, isLeaseAcceptable func(expiredToleration time.Duration) error) error {
	return m(ctx, isLeaseAcceptable)
}

type TrivialLeader struct{}

func (TrivialLeader) Lead(ctx context.Context, _ func(_ time.Duration) error) error {
	<-ctx.Done()
	return ctx.Err()
}

type FollowerFunc func(ctx context.Context, leader string) error

func (m FollowerFunc) Follow(ctx context.Context, leader string) error {
	return m(ctx, leader)
}

type TrivialFollower struct{}

func (TrivialFollower) Follow(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

type LeaderFollower struct {
	Leader
	Follower
}
