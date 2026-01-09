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

var (
	/*infoStr      = "trace_id-%s [INFO] call_at=%s|"
	warnStr      = "trace_id-%s [WARN] call_at=%s|"
	errStr       = "trace_id-%s [ERROR] call_at=%s|"
	traceStr     = "trace_id-%s [INFO] call_at=%s|time=%.3fms|rows=%v|%s"
	traceWarnStr = "trace_id-%s [WARN] call_at=%s|%s|time=%.3fms|rows=%v|%s"
	traceErrStr  = "trace_id-%s [ERROR] call_at=%s|%s|time=%.3fms|rows=%v|%s"*/

	infoStr      = "%s [INFO] call_at=%s|"
	warnStr      = "%s [WARN] call_at=%s|"
	errStr       = "%s [ERROR] call_at=%s|"
	traceStr     = "%s [INFO] call_at=%s|time=%.3fms|rows=%v|%s"
	traceWarnStr = "%s [WARN] call_at=%s|%s|time=%.3fms|rows=%v|%s"
	traceErrStr  = "%s [ERROR] call_at=%s|%s|time=%.3fms|rows=%v|%s"

	infoStr_no_trace      = "[INFO] call_at=%s|"
	warnStr_no_trace      = "[WARN] call_at=%s|"
	errStr_no_trace       = "[ERROR] call_at=%s|"
	traceStr_no_trace     = "[INFO] call_at=%s|time=%.3fms|rows=%v|%s"
	traceWarnStr_no_trace = "[WARN] call_at=%s|%s|time=%.3fms|rows=%v|%s"
	traceErrStr_no_trace  = "[ERROR] call_at=%s|%s|time=%.3fms|rows=%v|%s"
)

type Writer interface {
	Printf(string, ...interface{})
	Output(calldepth int, s string) error
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

	if config.Colorful {
		//infoStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Green + "[info] " + dlog.Reset
		infoStr = dlog.Green + "%s [INFO] " + "call_at=%s" + "|" + dlog.Reset

		//warnStr = dlog.BlueBold + "%s\n" + dlog.Reset + dlog.Magenta + "[warn] " + dlog.Reset
		//warnStr = dlog.BlueBold + "%s" + dlog.Reset + dlog.Magenta + " [warn] " + dlog.Reset
		warnStr = dlog.Yellow + "%s [WARN] " + "call_at=%s" + "|" + dlog.Reset

		//errStr = dlog.Magenta + "%s\n" + dlog.Reset + dlog.Red + "[error] " + dlog.Reset
		errStr = dlog.Red + "%s [ERROR] " + "call_at=%s" + "|" + dlog.Reset

		//traceStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		//traceStr = dlog.Green + "call_at=%s" + " [info] " + "time=%.3fms" + "|" + "rows=%v" + "|%s" + dlog.Reset
		traceStr = dlog.Green + "%s [INFO] " + "call_at=%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|%s" + dlog.Reset

		//traceWarnStr = dlog.Yellow + "call_at=%s" + " [warn] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset
		traceWarnStr = dlog.Yellow + "%s [WARN] " + "call_at=%s" + "|" + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset

		//traceErrStr = dlog.RedBold + "%s " + dlog.MagentaBold + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		//traceErrStr = dlog.RedBold + "call_at=%s" + " [error] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset
		traceErrStr = dlog.RedBold + "%s [ERROR] " + "call_at=%s" + "|" + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset

		infoStr_no_trace = dlog.Green + "[INFO] " + "call_at=%s" + "|" + dlog.Reset

		//warnStr = dlog.BlueBold + "%s\n" + dlog.Reset + dlog.Magenta + "[warn] " + dlog.Reset
		//warnStr = dlog.BlueBold + "%s" + dlog.Reset + dlog.Magenta + " [warn] " + dlog.Reset
		warnStr_no_trace = dlog.Yellow + "[WARN] " + "call_at=%s" + "|" + dlog.Reset

		//errStr = dlog.Magenta + "%s\n" + dlog.Reset + dlog.Red + "[error] " + dlog.Reset
		errStr_no_trace = dlog.Red + "[ERROR] " + "call_at=%s" + "|" + dlog.Reset

		//traceStr = dlog.Green + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		//traceStr = dlog.Green + "call_at=%s" + " [info] " + "time=%.3fms" + "|" + "rows=%v" + "|%s" + dlog.Reset
		traceStr_no_trace = dlog.Green + "[INFO] " + "call_at=%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|%s" + dlog.Reset

		//traceWarnStr = dlog.Yellow + "call_at=%s" + " [warn] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset
		traceWarnStr_no_trace = dlog.Yellow + "[WARN] " + "call_at=%s" + "|" + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset

		//traceErrStr = dlog.RedBold + "%s " + dlog.MagentaBold + "%s\n" + dlog.Reset + dlog.Yellow + "[%.3fms] " + dlog.BlueBold + "[rows:%v]" + dlog.Reset + " %s"
		//traceErrStr = dlog.RedBold + "call_at=%s" + " [error] " + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset
		traceErrStr_no_trace = dlog.RedBold + "[ERROR] " + "call_at=%s" + "|" + "%s" + "|" + "time=%.3fms" + "|" + "rows=%v" + "|" + "%s" + dlog.Reset

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
			Writer:       log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.Lshortfile),
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

func (l *MyDBlogger) fileCheck() {
	l.glog.fileCheck()
	l.Writer = l.glog.LogObj.lg
}

func (l *MyDBlogger) LogMode(level dlog.LogLevel) dlog.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

func (l MyDBlogger) Info(ctx context.Context, msg string, data ...interface{}) {
	/*trace_id := ""
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			trace_id = value
		}
	}*/
	trace_id := getIdsInLog(ctx)
	cmd := []interface{}{}

	if trace_id != "" {
		cmd = append(cmd, trace_id)
	} else {
		l.infoStr = infoStr_no_trace
	}

	cmd = append(cmd, utils.FileWithLineNum())
	cmd = append(cmd, data...)
	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
		/*if l.glog.LogObj != nil {
			l.glog.LogObj.mu.RLock()
			defer l.glog.LogObj.mu.RUnlock()
		}*/
	}

	if l.LogLevel >= dlog.Info {
		//fmt.Println(msg)
		//fmt.Println(l.infoStr)
		if l.Config.Colorful {
			//l.Printf(l.infoStr+dlog.Green+msg+dlog.Reset, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.infoStr+dlog.Green+msg+dlog.Reset, cmd...)
		} else {
			//l.Printf(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.infoStr+msg, cmd...)
		}
	}
}

func (l MyDBlogger) Warn(ctx context.Context, msg string, data ...interface{}) {

	/*trace_id := ""
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			trace_id = value
		}
	}*/
	trace_id := getIdsInLog(ctx)
	cmd := []interface{}{}

	if trace_id != "" {
		cmd = append(cmd, trace_id)
	} else {
		l.warnStr = warnStr_no_trace
	}

	cmd = append(cmd, utils.FileWithLineNum())
	cmd = append(cmd, data...)

	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
		/*if l.glog.LogObj != nil {
			l.glog.LogObj.mu.RLock()
			defer l.glog.LogObj.mu.RUnlock()
		}*/
	}
	if l.LogLevel >= dlog.Warn {
		if l.Config.Colorful {
			//l.Printf(l.warnStr+dlog.Yellow+msg+dlog.Reset, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.warnStr+dlog.Yellow+msg+dlog.Reset, cmd...)
		} else {
			//l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.warnStr+msg, cmd...)
		}
	}
}

func (l MyDBlogger) Error(ctx context.Context, msg string, data ...interface{}) {

	/*trace_id := ""
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			trace_id = value
		}
	}*/
	trace_id := getIdsInLog(ctx)
	cmd := []interface{}{}
	if trace_id != "" {
		cmd = append(cmd, trace_id)
	} else {
		l.errStr = errStr_no_trace
	}

	cmd = append(cmd, utils.FileWithLineNum())
	cmd = append(cmd, data...)

	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
		/*if l.glog.LogObj != nil {
			l.glog.LogObj.mu.RLock()
			defer l.glog.LogObj.mu.RUnlock()
		}*/
	}
	if l.LogLevel >= dlog.Error {
		if l.Config.Colorful {
			//l.Printf(l.errStr+dlog.Red+msg+dlog.Reset, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.errStr+dlog.Red+msg+dlog.Reset, cmd...)
		} else {
			//l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum()}, data...)...)
			l.Printf(l.errStr+msg, cmd...)
		}
	}
}

func (l MyDBlogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {

	/*trace_id := ""
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			trace_id = value
		}
	}*/
	trace_id := getIdsInLog(ctx)
	//prefix := ""
	//v := []interface{}{}
	v := make([]interface{}, 0, 100)
	if trace_id == "" {
		l.traceStr = traceStr_no_trace
		l.traceWarnStr = traceWarnStr_no_trace
		l.traceErrStr = traceErrStr_no_trace
	} else {
		v = append(v, trace_id)
	}

	if l.glog != nil && l.glog.LogObj != nil {
		l.fileCheck()
		//l.glog.fileCheck()
		/*if l.glog.LogObj != nil {
			l.glog.LogObj.mu.RLock()
			defer l.glog.LogObj.mu.RUnlock()
		}*/
	}
	if l.LogLevel <= dlog.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= dlog.Error && (!errors.Is(err, dlog.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		v = append(v, utils.FileWithLineNum())
		v = append(v, err)
		v = append(v, float64(elapsed.Nanoseconds())/1e6)
		if rows == -1 {
			//l.Printf(l.traceErrStr, trace_id, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			//v = fmt.Sprintf(l.traceErrStr, trace_id, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			v = append(v, "-")
			v = append(v, sql)
			l.Printf(l.traceErrStr, v...)
		} else {
			//l.Printf(l.traceErrStr, trace_id, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			//v = fmt.Sprintf(l.traceErrStr, trace_id, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			v = append(v, rows)
			v = append(v, sql)
			l.Printf(l.traceErrStr, v...)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= dlog.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		v = append(v, utils.FileWithLineNum())
		v = append(v, slowLog)
		v = append(v, float64(elapsed.Nanoseconds())/1e6)
		if rows == -1 {
			v = append(v, "-")
			v = append(v, sql)
			//l.Printf(l.traceWarnStr, trace_id, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			l.Printf(l.traceWarnStr, v...)
		} else {
			v = append(v, rows)
			v = append(v, sql)
			//l.Printf(l.traceWarnStr, trace_id, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			l.Printf(l.traceWarnStr, v...)
		}
	case l.LogLevel == dlog.Info:
		sql, rows := fc()
		v = append(v, utils.FileWithLineNum())
		v = append(v, float64(elapsed.Nanoseconds())/1e6)
		if rows == -1 {
			v = append(v, "-")
			v = append(v, sql)
			//l.Printf(l.traceStr, trace_id, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
			l.Printf(l.traceStr, v...)
		} else {
			v = append(v, rows)
			v = append(v, sql)
			l.Printf(l.traceStr, v...)
			//l.Printf(l.traceStr, trace_id, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
