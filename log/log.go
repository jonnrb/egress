package log // import "go.jonnrb.io/egress/log"

type Log interface {
	FatalLog
	ErrorLog
	WarningLog
	InfoLog
}

type Level int

type FatalLog interface {
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

type ErrorLog interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type WarningLog interface {
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
}

type InfoLog interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
}

func Fatal(args ...interface{}) {
	get().Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	get().Fatalf(format, args...)
}

func Error(args ...interface{}) {
	get().Error(args...)
}

func Errorf(format string, args ...interface{}) {
	get().Errorf(format, args...)
}

func Warning(args ...interface{}) {
	get().Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	get().Warningf(format, args...)
}

func Info(args ...interface{}) {
	get().Info(args...)
}

func Infof(format string, args ...interface{}) {
	get().Infof(format, args...)
}

func V(level Level) InfoLog {
	return wrapI{getV(level)}
}
