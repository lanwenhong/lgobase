package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	dlog "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type Writer interface {
	Printf(string, ...interface{})
}

type MyDBlogger struct {
	//Logobj *FILE
	glog *Glog
	Writer
	dlog.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

// func New(logobj *FILE, config dlog.Config) dlog.Interface {
func New(glog *Glog, config dlog.Config) dlog.Interface {

	var (
		//infoStr = "%s\n[info] "
		infoStr = "%s [info] "
		//warnStr = "%s\n[warn] "
		warnStr = "%s [warn] "
		//errStr  = "%s\n[error] "
		errStr = "%s [error] "
		//traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceStr = "%s [info] time=%.3fms|rows=%v|%s"
		//traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		//traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s [warn] %s|time=%.3fms|rows=%v|%s"
		//traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr = "%s [error] %s|time=%.3fms|rows=%v|%s"
	)

	if config.Colorful {
		//infoStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Green + "[info] " + dlog.Reset
		infoStr = dlog.Green + "%s" + " [info] " + dlog.Reset

		//warnStr = dlog.BlueBold + "%s\n" + dlog.Reset + dlog.Magenta + "[warn] " + dlog.Reset
		//warnStr = dlog.BlueBold + "%s" + dlog.Reset + dlog.Magenta + " [warn] " + dlog.Reset
		warnStr = dlog.Yellow + "%s" + " [warn] " + dlog.Reset

		//errStr = dlog.Magenta + "%s\n" + dlog.Reset + dlog.Red + "[error] " + dlog.Reset
		errStr = dlog.Magenta + "%s" + " [error] " + dlog.Reset

		//traceStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		traceStr = dlog.Green + "%s" + " [info] " + "time=%.3fms" + "|" + "rows=%v" + "|%s" + dlog.Reset

		traceWarnStr = dlog.Yellow + "%s" + " [warn] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset

		//traceErrStr = dlog.RedBold + "%s " + dlog.MagentaBold + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		traceErrStr = dlog.RedBold + "%s" + " [error] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset
	}
	if glog != nil && glog.LogObj != nil {
		return &MyDBlogger{
			glog:         glog,
			Writer:       glog.LogObj.lg,
			Config:       config,
			infoStr:      infoStr,
			warnStr:      warnStr,
			errStr:       errStr,
			traceStr:     traceStr,
			traceWarnStr: traceWarnStr,
			traceErrStr:  traceErrStr,
		}
	} else {
		return &MyDBlogger{
			glog:         nil,
			Writer:       log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds),
			Config:       config,
			infoStr:      infoStr,
			warnStr:      warnStr,
			errStr:       errStr,
			traceStr:     traceStr,
			traceWarnStr: traceWarnStr,
			traceErrStr:  traceErrStr,
		}
	}
}

func (l *MyDBlogger) LogMode(level dlog.LogLevel) dlog.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

func (l MyDBlogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
		if l.glog.LogObj != nil {
			l.glog.LogObj.mu.RLock()
			defer l.glog.LogObj.mu.RUnlock()
		}
	}
	if l.LogLevel >= dlog.Info {
		fmt.Println(msg)
		fmt.Println(l.infoStr)
		l.Printf(l.infoStr+dlog.Green+msg+dlog.Reset, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.glog.fileCheck()
	if l.glog.LogObj != nil {
		l.glog.LogObj.mu.RLock()
		defer l.glog.LogObj.mu.RUnlock()
	}

	if l.LogLevel >= dlog.Warn {
		l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.glog.fileCheck()
	if l.glog.LogObj != nil {
		l.glog.LogObj.mu.RLock()
		defer l.glog.LogObj.mu.RUnlock()
	}
	if l.LogLevel >= dlog.Error {
		l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.glog.fileCheck()
	if l.glog.LogObj != nil {
		l.glog.LogObj.mu.RLock()
		defer l.glog.LogObj.mu.RUnlock()
	}
	if l.LogLevel <= dlog.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= dlog.Error && (!errors.Is(err, dlog.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= dlog.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case l.LogLevel == dlog.Info:
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
