package vaddrha

import (
	"context"
	"time"

	"go.jonnrb.io/egress/ha"
	"go.jonnrb.io/egress/vaddr"
)

type Member struct {
	VAddr vaddr.Active
}

func New(va vaddr.Active) Member {
	return Member{va}
}

func (m Member) Lead(
	ctx context.Context,
	isLeaseAcceptable func(expiredToleration time.Duration) error) error {
	return m.VAddr.Run(ctx)
}

func (m Member) Follow(ctx context.Context, leader string) error {
	return ha.TrivialFollower{}.Follow(ctx, leader)
}
