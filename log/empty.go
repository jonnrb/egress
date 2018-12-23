package log

// Empty FatalLog isn't implemented since that doesn't make much sense.

type emptyE struct{}

func (emptyE) Error(...interface{}) {}

func (emptyE) Errorf(string, ...interface{}) {}

type emptyW struct{}

func (emptyW) Warning(...interface{}) {}

func (emptyW) Warningf(string, ...interface{}) {}

type emptyI struct{}

func (emptyI) Info(...interface{}) {}

func (emptyI) Infof(string, ...interface{}) {}
