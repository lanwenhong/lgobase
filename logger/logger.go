package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	dlog "gorm.io/gorm/logger"
)

const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlueBold    = "\033[34;1m"
	MagentaBold = "\033[35;1m"
	RedBold     = "\033[31;1m"
	YellowBold  = "\033[33;1m"
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

//var LogObj *FILE = nil

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
	lg           *log.Logger

	logfile_err *os.File
	lg_err      *log.Logger
}

type Glogconf struct {
	//maxFileSize     int64
	MaxFileSize int64
	//maxFileCount    int32
	MaxFileCount int32
	//dailyRolling    bool
	RotateMethod int
	//consoleAppender bool
	RollingFile bool
	Stdout      bool
	Colorful    bool
	Goid        bool
	Loglevel    LEVEL
}

type Glog struct {
	LogObj     *FILE
	Logconf    *Glogconf
	LogormConf *dlog.Config
}

var Gfilelog = NewDefaultGLog()

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
func NewDefaultGLog() (res *Glog) {
	res = &Glog{
		LogObj: &FILE{
			lg: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
		},
		Logconf: &Glogconf{
			RollingFile: false,
			Stdout:      true,
			Colorful:    true,
			Loglevel:    DEBUG,
		},
		LogormConf: &dlog.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  dlog.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	}
	res.SetRollingFile("", "", true)
	return
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
	glog := &Glog{
		Logconf:    glog_conf,
		LogormConf: dconfig,
	}
	if glog_conf.RotateMethod == ROTATE_FILE_NUM {
		glog.SetRollingFile(fileDir, fileName, glog_conf.Stdout)
	} else {
		glog.SetRollingDaily(fileDir, fileName, fileNameErr, glog_conf.Stdout)
		//glog.SetRollingDaily(fileName, fileNameErr, glog_conf.Stdout)
	}
	Gfilelog = glog
	return glog
}

func SetConsole(isConsole bool) {
	consoleAppender = isConsole
}

func SetLevel(_level LEVEL) {
	logLevel = _level
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
	/*defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()*/
	if glog.LogObj != nil && glog.isMustRename() {
		glog.LogObj.mu.Lock()
		defer glog.LogObj.mu.Unlock()
		glog.rename()
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
	for i := 1; i <= int(glog.Logconf.MaxFileCount); i++ {
		if isExist(fileDir + "/" + fileName + "." + strconv.Itoa(i)) {
			glog.LogObj._suffix = i
		} else {
			break
		}
	}
	if !glog.isMustRename() {
		if !stdout {
			glog.LogObj.logfile, _ = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.LogObj.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		} else {
			glog.LogObj.lg = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		}
	} else {
		glog.rename()
	}
	go glog.fileMonitor()

	return nil
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
			glog.LogObj.logfile, err = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.LogObj.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LstdFlags)

			glog.LogObj.logfile_err, err = os.OpenFile(fileDir+"/"+fileName_err, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.LogObj.lg_err = log.New(glog.LogObj.logfile_err, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LstdFlags)
		} else {
			glog.LogObj.lg = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
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
	if m := ctx.Value("trace_id"); m != nil {
		if value, ok := m.(string); ok {
			builder.WriteString(value)
			builder.WriteString(" ")
		}
	} else {
		builder.WriteString("- ")
	}
	if m := ctx.Value("request_id"); m != nil {
		if value, ok := m.(string); ok {
			builder.WriteString(value)
		}
	} else {
		builder.WriteString("-")
	}
	trace_id = builder.String()
	return trace_id
}

func pDebugWithGid(ctx context.Context, depth int, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	//values := []interface{}{}
	values := make([]interface{}, 0, 100)
	if trace_id != "" {
		//p_fmt = Red + "trace_id-%s [DEBUG] " + fmtstr + Reset
		//p_fmt = "trace_id %s [DEBUG] " + fmtstr
		p_fmt = "%s [DEBUG] " + fmtstr
		values = append(values, trace_id)
	} else {
		p_fmt = "[DEBUG] " + fmtstr
	}

	if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful {
		p_fmt = Green + p_fmt + Reset
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= DEBUG {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}

}

func pDebug(ctx context.Context, depth int, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	prefix := ""
	if trace_id != "" {
		//prefix = fmt.Sprintf("trace_id %s [DEBUG]", trace_id)
		prefix = fmt.Sprintf("%s [DEBUG]", trace_id)
	} else {
		prefix = fmt.Sprintf("%s", "[DEBUG]")
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		//Gfilelog.LogObj.mu.RLock()
		//defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful && Gfilelog.Logconf.Loglevel <= DEBUG {
			values := []interface{}{Green + prefix}
			values = append(values, v...)
			values = append(values, Reset)
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(values...))
		} else if Gfilelog.Logconf.Loglevel <= DEBUG {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		}
	}

}

func Debug(ctx context.Context, v ...interface{}) {
	pDebug(ctx, 3, v...)

}

func Debugf(ctx context.Context, fmtstr string, v ...interface{}) {
	pDebugWithGid(ctx, 3, fmtstr, v...)
}

func pInfoWithGid(ctx context.Context, depth int, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	//values := []interface{}{}
	values := make([]interface{}, 0, 100)
	if trace_id != "" {
		//p_fmt = "trace_id %s [INFO] " + fmtstr
		p_fmt = "%s [INFO] " + fmtstr
		values = append(values, trace_id)
	} else {
		p_fmt = "[INFO] " + fmtstr
	}

	if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful {
		p_fmt = Green + p_fmt + Reset
	}
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= INFO {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
		}
	}
}

func pInfo(ctx context.Context, depth int, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	//prefix := fmt.Sprintf("trace_id-%s [INFO]", trace_id)
	prefix := ""
	if trace_id != "" {
		//prefix = fmt.Sprintf("trace_id %s [INFO]", trace_id)
		prefix = fmt.Sprintf("%s [INFO]", trace_id)
	} else {
		prefix = fmt.Sprintf("%s", "[INFO]")
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		//Gfilelog.LogObj.mu.RLock()
		//defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful && Gfilelog.Logconf.Loglevel <= INFO {
			values := []interface{}{Green + prefix}
			values = append(values, v...)
			values = append(values, Reset)
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(values...))
		} else if Gfilelog.Logconf.Loglevel <= INFO {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		}
	}
}

func Info(ctx context.Context, v ...interface{}) {
	pInfo(ctx, 3, v...)
}

func Infof(ctx context.Context, fmtstr string, v ...interface{}) {
	pInfoWithGid(ctx, 3, fmtstr, v...)

}

func pWarnWithGid(ctx context.Context, depth int, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	//values := []interface{}{}
	values := make([]interface{}, 0, 100)
	if trace_id != "" {
		// p_fmt = Red + "trace_id-%s [WARN] " + fmtstr + Reset
		//p_fmt = "trace_id %s [WARN] " + fmtstr
		p_fmt = "%s [WARN] " + fmtstr
		values = append(values, trace_id)
	} else {
		p_fmt = "[WARN] " + fmtstr
	}

	if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful {
		p_fmt = Yellow + p_fmt + Reset
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
			if !Gfilelog.Logconf.Stdout {
				Gfilelog.LogObj.lg_err.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
			}
		}
	}
}

func pWarn(ctx context.Context, depth int, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	//prefix := fmt.Sprintf("trace_id-%s [WARN]", trace_id)
	prefix := ""
	if trace_id != "" {
		//prefix = fmt.Sprintf("trace_id %s [WARN]", trace_id)
		prefix = fmt.Sprintf("%s [WARN]", trace_id)
	} else {
		prefix = fmt.Sprintf("%s", "[WARN]")
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		//Gfilelog.LogObj.mu.RLock()
		//defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful && Gfilelog.Logconf.Loglevel <= WARN {
			values := []interface{}{Yellow + prefix}
			values = append(values, v...)
			values = append(values, Reset)
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(values...))
		} else if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		} else if Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
			Gfilelog.LogObj.lg_err.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		}
	}
}

func Warn(ctx context.Context, v ...interface{}) {
	pWarn(ctx, 3, v...)
}

func Warnf(ctx context.Context, fmtstr string, v ...interface{}) {
	pWarnWithGid(ctx, 3, fmtstr, v...)
}

func pErrorWithGid(ctx context.Context, depth int, fmtstr string, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	p_fmt := ""
	//values := []interface{}{}
	values := make([]interface{}, 0, 100)
	if trace_id != "" {
		//p_fmt = "trace_id %s [ERROR] " + fmtstr
		p_fmt = "%s [ERROR] " + fmtstr
		values = append(values, trace_id)
	} else {
		p_fmt = "[ERROR] " + fmtstr
	}

	if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful {
		p_fmt = Red + p_fmt + Reset
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.Logconf.Loglevel <= ERROR {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
			if !Gfilelog.Logconf.Stdout {
				Gfilelog.LogObj.lg_err.Output(depth, fmt.Sprintf(p_fmt, append(values, v...)...))
			}
		}
	}
}

func pError(ctx context.Context, depth int, v ...interface{}) {
	trace_id := getIdsInLog(ctx)
	//prefix := fmt.Sprintf("trace_id-%s [ERROR]", trace_id)
	prefix := ""
	if trace_id != "" {
		//prefix = fmt.Sprintf("trace_id %s [ERROR]", trace_id)
		prefix = fmt.Sprintf("%s [ERROR]", trace_id)
	} else {
		prefix = fmt.Sprintf("%s", "[ERROR]")
	}

	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		//Gfilelog.LogObj.mu.RLock()
		//defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Colorful && Gfilelog.Logconf.Loglevel <= ERROR {
			values := []interface{}{Red + prefix}
			values = append(values, v...)
			values = append(values, Reset)
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(values...))
		} else if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Loglevel <= ERROR {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		} else if Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
			Gfilelog.LogObj.lg_err.Output(depth, fmt.Sprintln(append([]interface{}{prefix}, v...)...))
		}
	}
}

func Error(ctx context.Context, v ...interface{}) {
	pError(ctx, 3, v...)
}

func Errorf(ctx context.Context, fmtstr string, v ...interface{}) {
	pErrorWithGid(ctx, 3, fmtstr, v...)
}

func (glog *Glog) isMustRename() bool {
	/*if glog.Logconf.Stdout {
		return false
	}*/
	if glog.Logconf.RotateMethod == ROTATE_FILE_DAILY {
		t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
		if t.After(*(glog.LogObj.Ldate)) {
			return true
		}
	} else {
		if glog.Logconf.MaxFileCount > 1 {
			if glog.fileSize(glog.LogObj.dir+"/"+glog.LogObj.filename) >= glog.Logconf.MaxFileSize {
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

		//fn := f.dir + "/" + f.filename + "." + f.Ldate.Format(DATEFORMAT)
		builder.WriteString(f.filename)
		builder.WriteString(".")
		builder.WriteString(f.Ldate.Format(DATEFORMAT))
		fn := filepath.Join(f.dir, builder.String())
		builder.Reset()

		//fn_err := f.dir + "/" + f.filename_err + "." + f.Ldate.Format(DATEFORMAT)
		builder.WriteString(f.filename_err)
		builder.WriteString(".")
		builder.WriteString(f.Ldate.Format(DATEFORMAT))
		fn_err := filepath.Join(f.dir, builder.String())

		if !isExist(fn) && !isExist(fn_err) && glog.isMustRename() {
			if f.logfile != nil && f.logfile_err != nil {
				f.logfile.Close()
				f.logfile_err.Close()
			}
			//err := os.Rename(f.dir+"/"+f.filename, fn)
			err := os.Rename(filepath.Join(f.dir, f.filename), fn)
			if err != nil {
				f.lg.Println("rename err", err.Error())
			}
			//err = os.Rename(f.dir+"/"+f.filename_err, fn_err)
			err = os.Rename(filepath.Join(f.dir, f.filename_err), fn_err)
			t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
			f.Ldate = &t

			f.logfile, _ = os.Create(filepath.Join(f.dir, f.filename))
			f.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
			f.logfile_err, _ = os.Create(filepath.Join(f.dir, f.filename_err))
			f.lg_err = log.New(glog.LogObj.logfile_err, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
		}
	} else {
		glog.coverNextOne()
	}
}

func (f *FILE) nextSuffix() int {
	return int(f._suffix%int(maxFileCount) + 1)
}

func (glog *Glog) coverNextOne() {
	f := glog.LogObj
	f._suffix = f.nextSuffix()
	if f.logfile != nil {
		f.logfile.Close()
	}

	var builder strings.Builder
	builder.Grow(100)

	//if isExist(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix))) {
	if isExist(filepath.Join(f.dir, builder.String())) {
		builder.WriteString(f.filename + "." + strconv.Itoa(int(f._suffix)))
		//os.Remove(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix)))
		os.Remove(filepath.Join(f.dir, builder.String()))
		builder.Reset()
	}

	builder.WriteString(f.filename + "." + strconv.Itoa(int(f._suffix)))
	//os.Rename(f.dir+"/"+f.filename, f.dir+"/"+f.filename+"."+strconv.Itoa(int(f._suffix)))
	os.Rename(filepath.Join(f.dir, f.filename), filepath.Join(f.dir, builder.String()))
	f.logfile, _ = os.Create(filepath.Join(f.dir, f.filename))
	f.lg = log.New(f.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
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
