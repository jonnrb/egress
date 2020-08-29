package vaddr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

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

func successfulWrapper(name string) func(func(string)) Wrapper {
	return func(recordAction func(a string)) Wrapper {
		return WrapperStruct{
			StartFunc: func() error {
				recordAction("start" + name)
				return nil
			},
			StopFunc: func() error {
				recordAction("stop" + name)
				return nil
			},
		}
	}
}

func failingInitWrapper(name string) func(func(string)) Wrapper {
	return func(recordAction func(a string)) Wrapper {
		return WrapperStruct{
			StartFunc: func() error {
				recordAction("start" + name)
				return errors.New("fail" + name)
			},
			StopFunc: func() error {
				recordAction("stop" + name)
				return nil
			},
		}
	}
}

func failingDeinitWrapper(name string) func(func(string)) Wrapper {
	return func(recordAction func(a string)) Wrapper {
		return WrapperStruct{
			StartFunc: func() error {
				recordAction("start" + name)
				return nil
			},
			StopFunc: func() error {
				recordAction("stop" + name)
				return errors.New("fail" + name)
			},
		}
	}
}

func successfulActive(name string) func(func(string)) Active {
	return func(recordAction func(a string)) Active {
		return ActiveFunc(func(ctx context.Context) error {
			recordAction("run" + name)
			return nil
		})
	}
}

func failingActive(name string) func(func(string)) Active {
	return func(recordAction func(a string)) Active {
		return ActiveFunc(func(ctx context.Context) error {
			recordAction("run" + name)
			return errors.New("fail" + name)
		})
	}
}

func newTestSuite(recordAction func(a string), entries ...interface{}) Suite {
	var s Suite
	for _, e := range entries {
		switch f := e.(type) {
		case func(func(string)) Wrapper:
			s.Wrappers = append(s.Wrappers, f(recordAction))
		case func(func(string)) Active:
			s.Actives = append(s.Actives, f(recordAction))
		default:
			panic(fmt.Sprintf("invalid type: %T", f))
		}
	}
	return s
}

type actionRecorder struct {
	a  []string
	mu sync.Mutex
}

func (r *actionRecorder) record(a string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.a = append(r.a, a)
}

func (r *actionRecorder) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return strings.Join(r.a, " ")
}

func TestSuite(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		successfulWrapper("W1"),
		successfulWrapper("W2"),
		successfulActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err != nil {
		t.Errorf("expected err == nil; got err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 runA1 runA2 stopW2 stopW1" &&
		a != "startW1 startW2 runA2 runA1 stopW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_earlyWrapperFailsOnInit(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		failingInitWrapper("W1"),
		successfulWrapper("W2"),
		successfulActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failW1" {
		t.Errorf("expected err == errors.New(\"failW1\"); got err == %v", err)
	}
	if a := r.get(); a != "startW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_lateWrapperFailsOnInit(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		successfulWrapper("W1"),
		failingInitWrapper("W2"),
		successfulActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failW2" {
		t.Errorf("expected err == errors.New(\"failW2\"); got err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_earlyWrapperFailsOnDeinit(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		failingDeinitWrapper("W1"),
		successfulWrapper("W2"),
		successfulActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failW1" {
		t.Errorf("expected err == errors.New(\"failW1\"); got err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 runA1 runA2 stopW2 stopW1" &&
		a != "startW1 startW2 runA2 runA1 stopW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_lateWrapperFailsOnDeinit(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		successfulWrapper("W1"),
		failingDeinitWrapper("W2"),
		successfulActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failW2" {
		t.Errorf("expected err == errors.New(\"failW2\"); got err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 runA1 runA2 stopW2 stopW1" &&
		a != "startW1 startW2 runA2 runA1 stopW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_failingAction(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		successfulWrapper("W1"),
		successfulWrapper("W2"),
		failingActive("A1"),
		successfulActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failA1" {
		t.Errorf("expected err == errors.New(\"failA1\"); got err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 runA1 runA2 stopW2 stopW1" &&
		a != "startW1 startW2 runA2 runA1 stopW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}

func TestSuite_multipleFailingAction(t *testing.T) {
	var r actionRecorder
	s := newTestSuite(
		r.record,
		successfulWrapper("W1"),
		successfulWrapper("W2"),
		failingActive("A1"),
		failingActive("A2"))

	err := s.Run(context.Background())

	if err.Error() != "failA1" && err.Error() != "failA2" {
		t.Errorf("got unexpected error: err == %v", err)
	}
	if a := r.get(); a != "startW1 startW2 runA1 runA2 stopW2 stopW1" &&
		a != "startW1 startW2 runA2 runA1 stopW2 stopW1" {
		t.Errorf("got bad action sequence: %v", a)
	}
}
