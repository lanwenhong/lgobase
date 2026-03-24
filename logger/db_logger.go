package logger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"time"

	dlog "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var (
	infoStr      = "%s %s [INFO] "
	warnStr      = "%s %s [WARN] "
	errStr       = "%s %s [ERROR] "
	traceStr     = "%s %s [INFO] time=%.3fms|rows=%v|%s"
	traceWarnStr = "%s %s [WARN] %s|time=%.3fms|rows=%v|%s"
	traceErrStr  = "%s %s [ERROR] %s|time=%.3fms|rows=%v|%s"

	infoStr_no_trace      = "%s [INFO] "
	warnStr_no_trace      = "%s [WARN] "
	errStr_no_trace       = "%s [ERROR] "
	traceStr_no_trace     = "%s [INFO] time=%.3fms|rows=%v|%s"
	traceWarnStr_no_trace = "%s [WARN] %s|time=%.3fms|rows=%v|%s"
	traceErrStr_no_trace  = "%s [ERROR] %s|time=%.3fms|rows=%v|%s"
)

type MyDBlogger struct {
	//Logobj *FILE
	glog *Glog
	//Writer
	Writer *slog.Logger
	dlog.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

func FileWithShortLineNumV2(fullPathWithLine string) string {

	//fullPathWithLine := utils.FileWithLineNum()
	// 2. 分割路径和行号（按最后一个冒号分割）
	// 示例：把 "/a/b/c.go:10" 拆成 "/a/b/c.go" 和 "10"
	sepIndex := -1
	for i := len(fullPathWithLine) - 1; i >= 0; i-- {
		if fullPathWithLine[i] == ':' {
			sepIndex = i
			break
		}
	}
	if sepIndex == -1 {
		// 分割失败则返回原生结果（兜底）
		return fullPathWithLine
	}

	// 3. 截取文件名（path.Base 去掉路径前缀） + 行号
	filePath := fullPathWithLine[:sepIndex]
	lineNum := fullPathWithLine[sepIndex:]
	shortFileName := path.Base(filePath) // 如 "/a/b/c.go" → "c.go"
	return shortFileName + lineNum
}

func New(glog *Glog, config dlog.Config) dlog.Interface {
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
		w := NewCustomLogger(os.Stdout, glog.LogObj.logfile, nil, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		slog.SetDefault(w)
		return &MyDBlogger{
			glog:         nil,
			Writer:       w,
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
	//cmd := []interface{}{}
	srcFileLineNum := FileWithShortLineNumV2(utils.FileWithLineNum())
	data = append(data, "call_line")
	data = append(data, srcFileLineNum)
	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
	}
	if l.LogLevel >= dlog.Info {
		slog.Default().InfoContext(ctx, msg, data...)
	}

}

func (l MyDBlogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	srcFileLineNum := FileWithShortLineNumV2(utils.FileWithLineNum())
	data = append(data, "call_line")
	data = append(data, srcFileLineNum)

	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
	}
	if l.LogLevel >= dlog.Warn {
		if l.Config.Colorful {
			slog.Default().WarnContext(ctx, msg, data...)
		} else {
			slog.Default().WarnContext(ctx, msg, data...)
		}
	}
}

func (l MyDBlogger) Error(ctx context.Context, msg string, data ...interface{}) {
	srcFileLineNum := FileWithShortLineNumV2(utils.FileWithLineNum())
	data = append(data, "call_line")
	data = append(data, srcFileLineNum)
	if l.glog != nil && l.glog.LogObj != nil {
		l.glog.fileCheck()
	}
	if l.LogLevel >= dlog.Error {
		if l.Config.Colorful {
			slog.Default().ErrorContext(ctx, msg, data...)
		} else {
			slog.Default().ErrorContext(ctx, msg, data...)
		}
	}
}

func (l MyDBlogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.glog != nil && l.glog.LogObj != nil {
		l.fileCheck()
	}
	if l.LogLevel <= dlog.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= dlog.Error && (!errors.Is(err, dlog.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		slog.Default().ErrorContext(ctx, "SQLERRLOG", "sql", sql, "time", float64(elapsed.Nanoseconds())/1e6, "rows", rows, "err", err, "call_line", FileWithShortLineNumV2(utils.FileWithLineNum()))

	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= dlog.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		slog.Default().WarnContext(ctx, "SQLSLOWLOG", "slowlog", slowLog, "rows", rows, "sql", sql, "time", float64(elapsed.Nanoseconds())/1e6, "call_line", FileWithShortLineNumV2(utils.FileWithLineNum()))
	case l.LogLevel == dlog.Info:
		sql, rows := fc()
		slog.Default().InfoContext(ctx, "SQLLOG", "rows", rows, "sql", sql, "time", float64(elapsed.Nanoseconds())/1e6, "call_line", FileWithShortLineNumV2(utils.FileWithLineNum()))
	}
}
