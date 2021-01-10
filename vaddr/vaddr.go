package vaddr

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Wrapper interface {
	Start() error
	Stop() error
}

type Active interface {
	Run(ctx context.Context) error
}

type WrapperStruct struct {
	StartFunc func() error
	StopFunc  func() error
}

func (s WrapperStruct) Start() error {
	return s.StartFunc()
}

func (s WrapperStruct) Stop() error {
	return s.StopFunc()
}

type ActiveFunc func(ctx context.Context) error

func (f ActiveFunc) Run(ctx context.Context) error {
	return f(ctx)
}

type Suite struct {
	Wrappers []Wrapper
	Actives  []Active
}

func (s Suite) Run(ctx context.Context) error {
	st := suiteState{Suite: s}
	st.run(ctx)
	return st.err()
}

type suiteState struct {
	Suite
	activeWrappers []Wrapper
	errs           []error
}

func (s *suiteState) run(ctx context.Context) {
	s.startWrappers()
	defer s.stopWrappers()
	if s.ok() {
		s.runActives(ctx)
	}
}

func (s *suiteState) ok() bool {
	return len(s.errs) == 0
}

func (s *suiteState) err() error {
	// TODO: join.
	if len(s.errs) != 0 {
		return s.errs[0]
	}
	return nil
}

func (s *suiteState) startWrappers() {
	s.activeWrappers = make([]Wrapper, len(s.Wrappers))[:0]
	for _, w := range s.Wrappers {
		err := w.Start()
		if err != nil {
			s.errs = append(s.errs, err)
			return
		}
		s.activeWrappers = append(s.activeWrappers, w)
	}
}

func (s *suiteState) runActives(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	defer cancel()

	for i := range s.Actives {
		a := s.Actives[i]
		eg.Go(func() error {
			return a.Run(ctx)
		})
	}

	err := eg.Wait()
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

func (s *suiteState) stopWrappers() {
	for _, w := range reversedWrappers(s.activeWrappers) {
		err := w.Stop()
		if err != nil {
			s.errs = append(s.errs, err)
		}
	}
}

func reversedWrappers(ws []Wrapper) []Wrapper {
	ws = append([]Wrapper{}, ws...)
	for i := len(ws)/2 - 1; i >= 0; i-- {
		opp := len(ws) - 1 - i
		ws[i], ws[opp] = ws[opp], ws[i]
	}
	return ws
}
