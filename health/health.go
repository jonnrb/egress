package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/context/ctxhttp"
)

type HealthChecker struct {
	c chan chan error
}

func New(ctx context.Context) *HealthChecker {
	hc := &HealthChecker{
		c: make(chan chan error),
	}
	go hc.loop(ctx)
	return hc
}

func (hc *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
