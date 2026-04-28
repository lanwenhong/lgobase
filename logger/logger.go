package logger

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	dlog "gorm.io/gorm/logger"
)

const (
	ROTATE_FILE_DAILY = iota + 100
	ROTATE_FILE_NUM
)

type LEVEL int

var logLevel LEVEL = 1
var maxFileSize int64
var maxFileCount int32
var dailyRolling bool = true
var consoleAppender bool = true
var RollingFile bool = false

const DATEFORMAT = "2006-01-02"

type UNIT int64

const (
	_       = iota
	KB UNIT = 1 << (iota * 10)
	MB
	GB
	TB
)
const (
	//ALL LEVEL = iota
	ALL = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
	OFF
)

type FILE struct {
	dir          string
	filename     string
	filename_err string
	_suffix      int
	isCover      bool
	Ldate        *time.Time
	mu           *sync.RWMutex
	logfile      *os.File
	lg           *slog.Logger

	logfile_err *os.File
}

type Glogconf struct {
	MaxFileSize      int64
	MaxFileCount     int32
	RotateMethod     int
	RollingFile      bool
	Stdout           bool
	Colorful         bool
	Goid             bool
	Loglevel         LEVEL
	CtxValueKey      string
	Format           int
	DesensitizeField string
}

type Glog struct {
	LogObj              *FILE
	Logconf             *Glogconf
	LogormConf          *dlog.Config
	LogTags             []string
	DesensitizeFieldMap map[string]bool
	DesensitizeFuncMap  map[string]DesensitizeFunc
	JsonMatchRegex      *regexp.Regexp
	XmlMatchRegex       *regexp.Regexp
}

// var Gfilelog = NewDefaultGLog()
var Gfilelog *Glog = nil
var GfilelogIgnore = NewDefaultGLog()

func GetGoid() uint64 {
	var (
		buf [64]byte
		n   = runtime.Stack(buf[:], false)
		stk = strings.TrimPrefix(string(buf[:n]), "goroutine")
	)

	idField := strings.Fields(stk)[0]
	id, err := strconv.ParseUint(idField, 10, 64)
	if err != nil {
		panic(fmt.Errorf("can not get goroutine id: %v", err))
	}
	return id
}

func GetstrGoid() string {
	if Gfilelog != nil && Gfilelog.Logconf != nil && !Gfilelog.Logconf.Goid {
		return "?"
	}
	var (
		buf [64]byte
		n   = runtime.Stack(buf[:], false)
		stk = strings.TrimPrefix(string(buf[:n]), "goroutine")
	)

	idField := strings.Fields(stk)[0]
	return idField
}

// NewDefaultGLog
// 生成默认的日志配置
func NewDefaultGLog() *Glog {
	originalHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   false, // 关闭官方长路径
		Level:       slog.LevelDebug,
		ReplaceAttr: DesensitizeReplaceAttr,
	})
	res := &Glog{
		LogObj: &FILE{
			lg: slog.New(NewMyModifyHandler(os.Stdout, nil, nil, originalHandler)),
		},
		Logconf: &Glogconf{
			RollingFile: false,
			Stdout:      true,
			Colorful:    true,
			Loglevel:    DEBUG,
			CtxValueKey: "trace_id,request_id,client_service,trace_depth",
		},
		LogormConf: &dlog.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  dlog.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	}
	res.DesensitizeFuncMap = make(map[string]DesensitizeFunc)
	res.LogTags = strings.Split(res.Logconf.CtxValueKey, ",")
	res.loadDesensitizeField()
	res.JsonMatchRegex = regexp.MustCompile(`(?s)^\s*(\{.*\}|\[.*\])\s*$`)
	res.XmlMatchRegex = regexp.MustCompile(`(?s)^\s*<\w+.*>.*</\w+>\s*$`)

	Gfilelog = res
	/*originalHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   false, // 关闭官方长路径
		Level:       slog.LevelDebug,
		ReplaceAttr: DesensitizeReplaceAttr,
	})
	res.LogObj = &FILE{
		lg: slog.New(NewMyModifyHandler(os.Stdout, nil, nil, originalHandler)),
	}*/

	res.SetRollingFile("", "", true)
	slog.SetDefault(res.LogObj.lg)
	return res
}

func Newglog(fileDir string, fileName string, fileNameErr string, glog_conf *Glogconf) *Glog {
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second,        // 慢 SQL 阈值
		LogLevel:                  dlog.Info,          // 日志级别
		IgnoreRecordNotFoundError: true,               // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  glog_conf.Colorful, // 禁用彩色打印
	}
	switch glog_conf.Loglevel {
	case DEBUG, INFO:
		dconfig.LogLevel = dlog.Info
	case WARN:
		dconfig.LogLevel = dlog.Warn
	case ERROR:
		dconfig.LogLevel = dlog.Error
	}
	if glog_conf.CtxValueKey == "" {
		glog_conf.CtxValueKey = "trace_id,request_id,client_service,trace_depth"
	}
	glog := &Glog{
		Logconf:    glog_conf,
		LogormConf: dconfig,
	}
	glog.DesensitizeFuncMap = make(map[string]DesensitizeFunc)
	glog.loadDesensitizeField()
	glog.JsonMatchRegex = regexp.MustCompile(`^\s*(\{.*\}|\[.*\])\s*$`)
	glog.XmlMatchRegex = regexp.MustCompile(`(?s)^\s*<\w+.*>.*</\w+>\s*$`)
	if glog_conf.RotateMethod == ROTATE_FILE_NUM {
		glog.SetRollingFile(fileDir, fileName, glog_conf.Stdout)
	} else {
		glog.SetRollingDaily(fileDir, fileName, fileNameErr, glog_conf.Stdout)
	}
	glog.LogTags = strings.Split(glog.Logconf.CtxValueKey, ",")
	Gfilelog = glog
	return glog
}

func SetConsole(isConsole bool) {
	consoleAppender = isConsole
}

func SetLevel(_level LEVEL) {
	logLevel = _level
}

func (glog *Glog) loadDesensitizeField() {
	if glog.Logconf.DesensitizeField != "" {
		glog.DesensitizeFieldMap = make(map[string]bool)
		field := strings.Split(glog.Logconf.DesensitizeField, ",")
		for _, f := range field {
			glog.DesensitizeFieldMap[f] = true
		}
	}
	//fmt.Println(glog.DesensitizeFieldMap)
}

func (glog *Glog) fileMonitor() {
	timer := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-timer.C:
			glog.fileCheck()
		}
	}
}

func (glog *Glog) fileCheck() {
	if glog.LogObj != nil && glog.isMustRename() {
		glog.LogObj.mu.Lock()
		defer glog.LogObj.mu.Unlock()
		glog.rename()
	}
}

func (glog *Glog) getSlogLevel() slog.Level {
	var sLevel = slog.LevelDebug

	switch glog.Logconf.Loglevel {
	case DEBUG:
		sLevel = slog.LevelDebug
	case INFO:
		sLevel = slog.LevelInfo
	case WARN:
		sLevel = slog.LevelWarn
	case ERROR:
		sLevel = slog.LevelError

	}
	return sLevel
}

func (glog *Glog) setSlog(errFile *os.File) {
	if glog.Logconf.Format == TEXT_FORMAT {
		glog.LogObj.lg = NewCustomLogger(os.Stdout, glog.LogObj.logfile, errFile, log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		slog.SetDefault(glog.LogObj.lg)
	} else {
		originalHandler := slog.NewJSONHandler(glog.LogObj.logfile, &slog.HandlerOptions{
			AddSource:   false, // 关闭官方长路径
			Level:       glog.getSlogLevel(),
			ReplaceAttr: DesensitizeReplaceAttr,
			//Level:     slog.LevelDebug,
		})
		glog.LogObj.lg = slog.New(NewMyModifyHandler(os.Stdout, glog.LogObj.logfile, errFile, originalHandler))
		slog.SetDefault(glog.LogObj.lg)
	}
}

func (glog *Glog) SetRollingFile(fileDir, fileName string, stdout bool) error {
	glog.LogObj = &FILE{
		dir:      fileDir,
		filename: fileName,
		isCover:  false,
		mu:       new(sync.RWMutex),
	}
	glog.LogObj.mu.Lock()
	defer glog.LogObj.mu.Unlock()

	var builder strings.Builder
	builder.Grow(100)
	for i := 1; i <= int(glog.Logconf.MaxFileCount); i++ {
		builder.WriteString(fileName)
		builder.WriteString(".")
		builder.WriteString(strconv.Itoa(i))
		fName := builder.String()
		if isExist(filepath.Join(fileDir, fName)) {
			glog.LogObj._suffix = i
		} else {
			break
		}
		builder.Reset()
	}
	if !glog.isMustRename() {
		if !stdout {
			glog.LogObj.logfile, _ = os.OpenFile(filepath.Join(fileDir, fileName), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.setSlog(nil)
		} else {
			glog.setSlog(nil)
		}
	} else {
		glog.rename()
	}
	return nil
}

func (glog *Glog) GetLogger() *slog.Logger {
	return glog.LogObj.lg
}

func (glog *Glog) RegisterDesensitizeFunc(ctx context.Context, k string, f DesensitizeFunc) {
	glog.DesensitizeFuncMap[k] = f
}

func (glog *Glog) SetRollingDaily(fileDir, fileName, fileName_err string, stdout bool) error {
	var err error = nil
	t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))

	glog.LogObj = &FILE{
		dir:          fileDir,
		filename:     fileName,
		filename_err: fileName_err,
		Ldate:        &t,
		isCover:      false,
		mu:           new(sync.RWMutex),
	}

	glog.LogObj.mu.Lock()
	defer glog.LogObj.mu.Unlock()

	if !glog.isMustRename() {
		if !stdout {
			glog.LogObj.logfile, err = os.OpenFile(filepath.Join(fileDir, fileName), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.LogObj.logfile_err, err = os.OpenFile(filepath.Join(fileDir, fileName_err), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.setSlog(glog.LogObj.logfile_err)
		} else {
			glog.setSlog(nil)
		}
	} else {
		glog.rename()
	}
	return err
}

func catchError() {
	if err := recover(); err != nil {
		log.Println("err", err)
	}
}

func getIdsInLog(ctx context.Context) string {
	var builder strings.Builder
	builder.Grow(100)
	trace_id := ""
	plen := len(Gfilelog.LogTags)
	for index, k := range Gfilelog.LogTags {
		if m := ctx.Value(k); m != nil {
			if value, ok := m.(string); ok {
				builder.WriteString(value)
			}
		} else {
			builder.WriteString("-")
		}
		if index != plen-1 {
			builder.WriteString(" ")
		}
	}
	trace_id = builder.String()
	return trace_id
}

func Debug(ctx context.Context, msg string, v ...any) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		slog.Default().DebugContext(ctx, msg, v...)
	}
}

func Debugf(ctx context.Context, fmtstr string, v ...interface{}) {
	p_fmt := ""
	values := make([]interface{}, 0, 100)
	if Gfilelog.Logconf.Format == TEXT_FORMAT {
		trace_id := getIdsInLog(ctx)
		if trace_id != "" {
			values = append(values, trace_id)
		} else {
			p_fmt = "[DEBUG] " + fmtstr
		}
	} else {
		p_fmt = fmtstr
	}
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= DEBUG {
			slog.Default().DebugContext(ctx, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}
}

func Info(ctx context.Context, msg string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		slog.Default().InfoContext(ctx, msg, v...)
	}
}

func Infof(ctx context.Context, fmtstr string, v ...interface{}) {
	p_fmt := ""
	values := make([]interface{}, 0, 100)
	if Gfilelog.Logconf.Format == TEXT_FORMAT {
		trace_id := getIdsInLog(ctx)
		if trace_id != "" {
			p_fmt = "%s [INFO] " + fmtstr
			values = append(values, trace_id)
		} else {
			p_fmt = "[INFO] " + fmtstr
		}
	} else {
		p_fmt = fmtstr
	}
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= INFO {
			slog.Default().InfoContext(ctx, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}
}

func Warn(ctx context.Context, msg string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		slog.Default().WarnContext(ctx, msg, v...)
	}
}

func Warnf(ctx context.Context, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	values := make([]interface{}, 0, 100)
	if Gfilelog.Logconf.Format == TEXT_FORMAT {
		if trace_id != "" {
			p_fmt = "%s [WARN] " + fmtstr
			values = append(values, trace_id)
		} else {
			p_fmt = "[WARN] " + fmtstr
		}
	} else {
		p_fmt = fmtstr
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= WARN {
			slog.Default().WarnContext(ctx, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}

}

func Error(ctx context.Context, msg string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		slog.Default().ErrorContext(ctx, msg, v...)
	}
}

func Errorf(ctx context.Context, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	values := make([]interface{}, 0, 100)

	if Gfilelog.Logconf.Format == TEXT_FORMAT {
		if trace_id != "" {
			p_fmt = "%s [ERROR] " + fmtstr
			values = append(values, trace_id)
		} else {
			p_fmt = "[ERROR] " + fmtstr
		}
	} else {
		p_fmt = fmtstr
	}
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= ERROR {
			slog.Default().ErrorContext(ctx, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}

}

func (glog *Glog) isMustRename() bool {
	if glog.Logconf.RotateMethod == ROTATE_FILE_DAILY {
		t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
		if t.After(*(glog.LogObj.Ldate)) {
			return true
		}
	} else {
		if glog.Logconf.MaxFileCount > 1 {
			if glog.fileSize(filepath.Join(glog.LogObj.dir, glog.LogObj.filename)) >= glog.Logconf.MaxFileSize {
				return true
			}
		}
	}
	return false
}

func (glog *Glog) rename() {
	if glog.Logconf.Stdout {
		return
	}

	f := glog.LogObj
	if glog.Logconf.RotateMethod == ROTATE_FILE_DAILY {
		var builder strings.Builder
		builder.Grow(100)
		builder.WriteString(f.filename)
		builder.WriteString(".")
		builder.WriteString(f.Ldate.Format(DATEFORMAT))
		fn := filepath.Join(f.dir, builder.String())
		builder.Reset()

		builder.WriteString(f.filename_err)
		builder.WriteString(".")
		builder.WriteString(f.Ldate.Format(DATEFORMAT))
		fn_err := filepath.Join(f.dir, builder.String())

		if !isExist(fn) && !isExist(fn_err) && glog.isMustRename() {
			if f.logfile != nil && f.logfile_err != nil {
				f.logfile.Close()
				f.logfile_err.Close()
			}
			err := os.Rename(filepath.Join(f.dir, f.filename), fn)
			if err != nil {
				slog.Error("rename err: %s", err.Error())
			}
			err = os.Rename(filepath.Join(f.dir, f.filename_err), fn_err)
			t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
			f.Ldate = &t

			f.logfile, _ = os.Create(filepath.Join(f.dir, f.filename))
			f.logfile_err, _ = os.Create(filepath.Join(f.dir, f.filename_err))
			glog.setSlog(f.logfile_err)
			slog.SetDefault(f.lg)
		}
	} else {
		if glog.isMustRename() {
			glog.coverNextOne()
		}
	}
}

func (glog *Glog) nextSuffix() int {
	f := glog.LogObj
	f._suffix = int(f._suffix%int(glog.Logconf.MaxFileCount) + 1)
	return f._suffix
}

func (glog *Glog) coverNextOne() {
	f := glog.LogObj
	glog.nextSuffix()
	if f.logfile != nil {
		f.logfile.Close()
	}

	var builder strings.Builder
	builder.Grow(100)

	builder.WriteString(f.filename)
	builder.WriteString(".")
	builder.WriteString(strconv.Itoa(int(f._suffix)))
	if isExist(filepath.Join(f.dir, builder.String())) {
		os.Remove(filepath.Join(f.dir, builder.String()))
	}
	os.Rename(filepath.Join(f.dir, f.filename), filepath.Join(f.dir, builder.String()))
	f.logfile, _ = os.Create(filepath.Join(f.dir, f.filename))
	glog.setSlog(nil)
	slog.SetDefault(f.lg)
}

func (glog *Glog) fileSize(file string) int64 {
	fmt.Println("fileSize", file)
	f, e := os.Stat(file)
	if e != nil {
		fmt.Println(e.Error())
		return 0
	}
	return f.Size()
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func LoggerLevelIndex(key string) (LEVEL, bool) {
	index := make(map[string]LEVEL)
	index["ALL"] = ALL
	index["DEBUG"] = DEBUG
	index["INFO"] = INFO
	index["WARN"] = WARN
	index["ERROR"] = ERROR
	index["FATAL"] = FATAL
	index["OFF"] = OFF

	level, ok := index[key]

	return level, ok
}
