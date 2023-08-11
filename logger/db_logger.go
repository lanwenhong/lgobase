package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	dlog "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type Writer interface {
	Printf(string, ...interface{})
}

type MyDBlogger struct {
	Logobj *FILE
	Writer
	dlog.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

func New(logobj *FILE, config dlog.Config) dlog.Interface {

	var (
		infoStr      = "%s\n[info] "
		warnStr      = "%s\n[warn] "
		errStr       = "%s\n[error] "
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Green + "[info] " + dlog.Reset
		warnStr = dlog.BlueBold + "%s\n" + dlog.Reset + dlog.Magenta + "[warn] " + dlog.Reset
		errStr = dlog.Magenta + "%s\n" + dlog.Reset + dlog.Red + "[error] " + dlog.Reset
		traceStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		traceWarnStr = dlog.Green + "%s " + dlog.Yellow + "%s\n" + dlog.Reset + dlog.RedBold + "[%.3fms] " + dlog.Yellow + "[rows:%v]" + dlog.Magenta + " %s" + dlog.Reset
		traceErrStr = dlog.RedBold + "%s " + dlog.MagentaBold + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
	}
	if logobj == nil {

	}
	if logobj != nil {
		return &MyDBlogger{
			Logobj:       logobj,
			Writer:       logobj.lg,
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
			Logobj:       nil,
			Writer:       nil,
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
	fileCheck()
	if l.Logobj != nil {
		l.Logobj.mu.RLock()
		defer l.Logobj.mu.RUnlock()
	}
	if l.LogLevel >= dlog.Info {
		//l.Printf(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		l.Print(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	fileCheck()
	if l.Logobj != nil {
		l.Logobj.mu.RLock()
		defer l.Logobj.mu.RUnlock()
	}

	if l.LogLevel >= dlog.Warn {
		//l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		l.Print(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Error(ctx context.Context, msg string, data ...interface{}) {
	fileCheck()
	if l.Logobj != nil {
		l.Logobj.mu.RLock()
		defer l.Logobj.mu.RUnlock()
	}
	if l.LogLevel <= dlog.Error {
		//l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
		l.Print(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
	}
}

func (l MyDBlogger) Print(format string, v ...any) {
	if l.Logobj != nil {
		l.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

func (l MyDBlogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	fileCheck()
	if l.Logobj != nil {
		l.Logobj.mu.RLock()
		defer l.Logobj.mu.RUnlock()
	}
	if l.LogLevel <= dlog.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= dlog.Error && (!errors.Is(err, dlog.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			/*if l.Logobj != nil {
				l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				log.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			}*/
			l.Print(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			/*if l.Logobj != nil {
				l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			} else {
				log.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}*/
			l.Print(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= dlog.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			/*if l.Logobj != nil {
				l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				log.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			}*/
			l.Print(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			/*if l.Logobj != nil {
				l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			} else {
				log.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}*/
			l.Print(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case l.LogLevel == dlog.Info:
		sql, rows := fc()
		if rows == -1 {
			/*if l.Logobj != nil {
				l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				log.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			}*/
			l.Print(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			/*if l.Logobj != nil {
				l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
			} else {
				log.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}*/
			l.Print(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
