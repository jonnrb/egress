package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.jonnrb.io/egress/ha"
	"golang.org/x/net/context/ctxhttp"
)

type HealthChecker struct {
	c chan chan error
	*haObserver
}

func New(ctx context.Context, haHandler func(m ha.Member)) *HealthChecker {
	hc := &HealthChecker{
		c: make(chan chan error),
	}
	if haHandler != nil {
		hc.haObserver = &haObserver{}
		haHandler(hc.haObserver)
	}
	go hc.loop(ctx)
	return hc
}

func (hc *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if hc.haObserver != nil {
		isLeader, ok := hc.haObserver.getStatus()
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, "ha down\n")
			return
		}
		if !isLeader {
			io.WriteString(w, "OK (ha is follower)\n")
			return
		}
	}

	var err error
	c := make(chan error, 1)
	select {
	case hc.c <- c:
		select {
		case err = <-c:
		case <-ctx.Done():
			err = ctx.Err()
		}
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("%v\n", err.Error()))
	} else {
		io.WriteString(w, "OK\n")
	}
}

func (hc *HealthChecker) loop(ctx context.Context) {
	for ret := range hc.c {
		func() {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err := httpHeadCheck(ctx)
			ret <- err
		}()
	}
}

func httpHeadCheck(ctx context.Context) error {
	if _, err := ctxhttp.Head(ctx, nil, "https://google.com/"); err != nil {
		return err
	} else {
		return nil
	}
}

type haObserver struct {
	mu         sync.Mutex
	isLeader   bool
	isFollower bool
}

func (m *haObserver) getStatus() (isLeader, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.isLeader, m.isLeader || m.isFollower
}

func (m *haObserver) Lead(ctx context.Context, _ func(_ time.Duration) error) error {
	return m.observe(ctx, &m.isLeader)
}

func (m *haObserver) Follow(ctx context.Context, _ string) error {
	return m.observe(ctx, &m.isFollower)
}

func (m *haObserver) observe(ctx context.Context, p *bool) error {
	m.set(true, p)
	<-ctx.Done()
	m.set(false, p)
	return ctx.Err()
}

func (m *haObserver) set(val bool, p *bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	*p = val
}
