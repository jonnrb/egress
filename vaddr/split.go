package vaddr

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Splits a Suite into a Wrapper that starts and stops all of the constituent
// wrappers in order and an Active that concurrently runs all of the constituent
// actives.
func Split(s Suite) (*CombinedWrappers, CombinedActives) {
	return &CombinedWrappers{Wrappers: s.Wrappers}, CombinedActives(s.Actives)
}

func HasActive(s Suite) bool {
	return len(s.Actives) != 0
}

func HasWrapper(s Suite) bool {
	return len(s.Wrappers) != 0
}

type CombinedWrappers struct {
	Wrappers []Wrapper

	activeWrappers []Wrapper
	errs           []error
}

func (s *CombinedWrappers) Start() error {
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

func (s *CombinedWrappers) Stop() error {
	for _, w := range reversedWrappers(s.activeWrappers) {
		err := w.Stop()
		if err != nil {
			s.errs = append(s.errs, err)
		}
	}
	return s.err()
}

type CombinedActives []Active

func (s CombinedActives) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	defer cancel()

	for i := range s {
		a := s[i]
		eg.Go(func() error {
			return a.Run(ctx)
		})
	}

	return eg.Wait()
}

func (s *CombinedWrappers) err() error {
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
