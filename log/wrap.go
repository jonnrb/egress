package log

// Top-level calls always do a 1-func wrap around an implementation returned by
// getX?(). This exists so that those implementations can return XYZLog
// implementations to users and always consider their callers to be two stack
// frames above them.

type wrap struct {
	Log
}

func (w wrap) Fatal(args ...interface{}) {
	w.Log.Fatal(args...)
}

func (w wrap) Fatalf(format string, args ...interface{}) {
	w.Log.Fatalf(format, args...)
}

func (w wrap) Error(args ...interface{}) {
	w.Log.Error(args...)
}

func (w wrap) Errorf(format string, args ...interface{}) {
	w.Log.Errorf(format, args...)
}

func (w wrap) Warning(args ...interface{}) {
	w.Log.Warning(args...)
}

func (w wrap) Warningf(format string, args ...interface{}) {
	w.Log.Warningf(format, args...)
}

func (w wrap) Info(args ...interface{}) {
	w.Log.Info(args...)
}

func (w wrap) Infof(format string, args ...interface{}) {
	w.Log.Infof(format, args...)
}

type wrapF struct {
	FatalLog
}

func (w wrapF) Fatal(args ...interface{}) {
	w.FatalLog.Fatal(args...)
}

func (w wrapF) Fatalf(format string, args ...interface{}) {
	w.FatalLog.Fatalf(format, args...)
}

type wrapE struct {
	ErrorLog
}

func (w wrapE) Error(args ...interface{}) {
	w.ErrorLog.Error(args...)
}

func (w wrapE) Errorf(format string, args ...interface{}) {
	w.ErrorLog.Errorf(format, args...)
}

type wrapW struct {
	WarningLog
}

func (w wrapW) Warning(args ...interface{}) {
	w.WarningLog.Warning(args...)
}

func (w wrapW) Warningf(format string, args ...interface{}) {
	w.WarningLog.Warningf(format, args...)
}

type wrapI struct {
	i InfoLog
}

func (w wrapI) Info(args ...interface{}) {
	w.i.Info(args...)
}

func (w wrapI) Infof(format string, args ...interface{}) {
	w.i.Infof(format, args...)
}
