package vaddr

import (
	"context"
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

func (s Suite) Run(ctx context.Context) (err error) {
	w, a := Split(s)

	err = w.Start()
	if err != nil {
		return
	}
	defer func() {
		errStop := w.Stop()
		if err == nil {
			err = errStop
		}
	}()

	err = a.Run(ctx)
	return
}
