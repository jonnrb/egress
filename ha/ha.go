package ha

import (
	"context"
	"time"
)

type Leader interface {
	// Runs when the member becomes the leader. The context is cancelled if
	// this node is notified a member on another node has become the leader.
	// Returning nil or context.Canceled should signal to a Coordinator that the
	// leadership was successful and the leader is stepping down cleanly.
	// Returning another value of error indicates some sort of failure and the
	// leader must step down.
	Lead(ctx context.Context, isLeaseAcceptable func(expiredToleration time.Duration) error) error
}

type Follower interface {
	// Runs when the member is a follower of another member. The context is
	// cancelled if this node is notified a member on another node (possibly
	// this node) has become the leader. Returning nil or context.Canceled is
	// a signal to a Coordinator that the followship was successful. Returning
	// another value of error won't necessarily do anything, but may cause a
	// calling Coordinator to try and become the leader.
	Follow(ctx context.Context, leader string) error
}

type Member interface {
	Leader
	Follower
}

type Coordinator interface {
	Run(ctx context.Context, m Member) error
}
