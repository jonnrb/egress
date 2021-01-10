package vaddr

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Splits a Suite into a Wrapper that starts and stops all of the constituent
// wrappers in order and an Active that concurrently runs all of the constituent
// actives.
func Split(s Suite) (Wrapper, Active) {
	return &suiteSplitWrappers{Suite: s}, suiteSplitActives(s)
}

func HasActive(s Suite) bool {
	return len(s.Actives) != 0
}

func HasWrapper(s Suite) bool {
	return len(s.Wrappers) != 0
}

type suiteSplitWrappers struct {
	Suite
	activeWrappers []Wrapper
	errs           []error
}

func (s *suiteSplitWrappers) Start() error {
	s.activeWrappers = make([]Wrapper, len(s.Wrappers))[:0]
	for _, w := range s.Wrappers {
		err := w.Start()
		if err != nil {
			s.errs = append(s.errs, err)
			s.Stop()
			return s.err()
		}
		s.activeWrappers = append(s.activeWrappers, w)
	}
	return nil
}

func (s *suiteSplitWrappers) Stop() error {
	for _, w := range reversedWrappers(s.activeWrappers) {
		err := w.Stop()
		if err != nil {
			s.errs = append(s.errs, err)
		}
	}
	return s.err()
}

type suiteSplitActives Suite

func (s suiteSplitActives) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	defer cancel()

	for i := range s.Actives {
		a := s.Actives[i]
		eg.Go(func() error {
			return a.Run(ctx)
		})
	}

	return eg.Wait()
}

func (s *suiteSplitWrappers) err() error {
	// TODO: join.
	if len(s.errs) != 0 {
		return s.errs[0]
	}
	return nil
}

func reversedWrappers(ws []Wrapper) []Wrapper {
	ws = append([]Wrapper{}, ws...)
	for i := len(ws)/2 - 1; i >= 0; i-- {
		opp := len(ws) - 1 - i
		ws[i], ws[opp] = ws[opp], ws[i]
	}
	return ws
}
