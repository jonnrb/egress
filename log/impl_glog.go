package log

import (
	"fmt"

	"github.com/golang/glog"
)

func get() Log {
	return glogger{}
}

func getV(level Level) InfoLog {
	if glog.V(glog.Level(level)) {
		return glogger{}
	} else {
		return emptyI{}
	}
}

type glogger struct{}

func (glogger) Fatal(args ...interface{}) {
	glog.FatalDepth(3, fmt.Sprintln(args...))
}

func (glogger) Fatalf(format string, args ...interface{}) {
	glog.FatalDepth(3, fmt.Sprintf(format, args...))
}

func (glogger) Error(args ...interface{}) {
	glog.ErrorDepth(3, fmt.Sprintln(args...))
}

func (glogger) Errorf(format string, args ...interface{}) {
	glog.ErrorDepth(3, fmt.Sprintf(format, args...))
}

func (glogger) Warning(args ...interface{}) {
	glog.WarningDepth(3, fmt.Sprintln(args...))
}

func (glogger) Warningf(format string, args ...interface{}) {
	glog.WarningDepth(3, fmt.Sprintf(format, args...))
}

func (glogger) Info(args ...interface{}) {
	glog.InfoDepth(3, fmt.Sprintln(args...))
}

func (glogger) Infof(format string, args ...interface{}) {
	glog.InfoDepth(3, fmt.Sprintf(format, args...))
}
