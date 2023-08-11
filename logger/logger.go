package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	_VER string = "1.0.2"
)

type LEVEL int

var logLevel LEVEL = 1
var maxFileSize int64
var maxFileCount int32
var dailyRolling bool = true
var consoleAppender bool = true
var RollingFile bool = false
var LogObj *FILE = nil

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
	_date        *time.Time
	mu           *sync.RWMutex
	logfile      *os.File
	lg           *log.Logger

	logfile_err *os.File
	lg_err      *log.Logger
}

func SetConsole(isConsole bool) {
	consoleAppender = isConsole
}

func SetLevel(_level LEVEL) {
	logLevel = _level
}

func SetRollingFile(fileDir, fileName string, maxNumber int32, maxSize int64, _unit UNIT) {
	maxFileCount = maxNumber
	maxFileSize = maxSize * int64(_unit)
	RollingFile = true
	dailyRolling = false
	LogObj = &FILE{dir: fileDir, filename: fileName, isCover: false, mu: new(sync.RWMutex)}
	LogObj.mu.Lock()
	defer LogObj.mu.Unlock()
	for i := 1; i <= int(maxNumber); i++ {
		if isExist(fileDir + "/" + fileName + "." + strconv.Itoa(i)) {
			LogObj._suffix = i
		} else {
			break
		}
	}
	if !LogObj.isMustRename() {
		LogObj.logfile, _ = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		LogObj.lg = log.New(LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	} else {
		LogObj.rename()
	}
	go fileMonitor()
}

func SetRollingDaily(fileDir, fileName string, fileName_err string) {
	RollingFile = false
	dailyRolling = true
	t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))

	LogObj = &FILE{dir: fileDir, filename: fileName, filename_err: fileName_err, _date: &t, isCover: false, mu: new(sync.RWMutex)}
	LogObj.mu.Lock()
	defer LogObj.mu.Unlock()

	if !LogObj.isMustRename() {
		LogObj.logfile, _ = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		LogObj.lg = log.New(LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

		LogObj.logfile_err, _ = os.OpenFile(fileDir+"/"+fileName_err, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		LogObj.lg_err = log.New(LogObj.logfile_err, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	} else {
		LogObj.rename()
	}
}

func console(s ...interface{}) {
	if consoleAppender {
		_, file, line, _ := runtime.Caller(2)
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		log.Println(file, strconv.Itoa(line), s)
	}
}

func console_new(v string) {
	if consoleAppender {
		_, file, line, _ := runtime.Caller(2)
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
		var pv = file + " " + strconv.Itoa(line) + " " + v
		log.Println(pv)
	}
}

func catchError() {
	if err := recover(); err != nil {
		log.Println("err", err)
	}
}

func Debug(v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}

	if logLevel <= DEBUG {
		if LogObj != nil {
			LogObj.lg.Output(2, fmt.Sprintln("debug", v))
		}
		console("debug", v)
	}
}

func Debugf(format string, v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}

	if logLevel <= DEBUG {
		if LogObj != nil {
			//LogObj.lg.Output(2, fmt.Sprintln("debug", v))
			LogObj.lg.Output(2, "[debug] "+fmt.Sprintf(format, v...))
		}
		//console("debug", v)
		stdv := "[debug] " + fmt.Sprintf(format, v...)
		console_new(stdv)

	}
}

func Info(v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= INFO {
		if LogObj != nil {
			LogObj.lg.Output(2, "[INFO] "+fmt.Sprintln(v))
		}
		stdv := "[INFO] " + fmt.Sprintln(v)
		console_new(stdv)
	}
}

func Infof(format string, v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= INFO {
		if LogObj != nil {
			LogObj.lg.Output(2, "[INFO] "+fmt.Sprintf(format, v...))
		}
		stdv := "[INFO] " + fmt.Sprintf(format, v...)
		console_new(stdv)
	}
}

func Warnf(format string, v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}

	if logLevel <= WARN {
		if LogObj != nil {
			LogObj.lg.Output(2, "[WARN] "+fmt.Sprintf(format, v...))
			if dailyRolling {
				LogObj.lg_err.Output(2, "[WARN] "+fmt.Sprintf(format, v...))
			}
		}
		stdv := "[WARN] " + fmt.Sprintf(format, v...)
		console_new(stdv)
	}
}

func Warn(v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}

	if logLevel <= WARN {
		if LogObj != nil {
			LogObj.lg.Output(2, fmt.Sprintln("WARN", v))
			if dailyRolling {
				LogObj.lg_err.Output(2, fmt.Sprintln("WARN", v))
			}
		}
		stdv := "[warn] " + fmt.Sprintln(v)
		console_new(stdv)
	}
}

func Errorf(format string, v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= ERROR {
		if LogObj != nil {
			LogObj.lg.Output(2, "[ERROR] "+fmt.Sprintf(format, v...))
			if dailyRolling {
				LogObj.lg_err.Output(2, "[ERROR] "+fmt.Sprintf(format, v...))
			}
		}
		stdv := "[ERROR] " + fmt.Sprintf(format, v...)
		console_new(stdv)

	}
}

func Error(v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= ERROR {
		if LogObj != nil {
			LogObj.lg.Output(2, fmt.Sprintln("ERROR", v))
			if dailyRolling {
				LogObj.lg_err.Output(2, fmt.Sprintln("ERROR", v))
			}

		}
		stdv := "[error] " + fmt.Sprintln(v)
		console_new(stdv)

	}
}

func Fatalf(format string, v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= FATAL {
		if LogObj != nil {
			LogObj.lg.Output(2, "[FATAL] "+fmt.Sprintf(format, v...))
			if dailyRolling {
				LogObj.lg_err.Output(2, "[FATAL] "+fmt.Sprintf(format, v...))
			}

		}
		stdv := "[FATAL] " + fmt.Sprintf(format, v...)
		console_new(stdv)

	}
}

func Fatal(v ...interface{}) {
	if dailyRolling {
		fileCheck()
	}
	defer catchError()
	if LogObj != nil {
		LogObj.mu.RLock()
		defer LogObj.mu.RUnlock()
	}
	if logLevel <= FATAL {
		if LogObj != nil {
			LogObj.lg.Output(2, fmt.Sprintln("FATAL", v))
			if dailyRolling {
				LogObj.lg_err.Output(2, fmt.Sprintln("FATAL", v))
			}

		}
		stdv := "[fatal] " + fmt.Sprintln(v)
		console_new(stdv)
	}
}

func (f *FILE) isMustRename() bool {
	if dailyRolling {
		t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
		if t.After(*f._date) {
			return true
		}
	} else {
		if maxFileCount > 1 {
			if fileSize(f.dir+"/"+f.filename) >= maxFileSize {
				return true
			}
		}
	}
	return false
}

func (f *FILE) rename() {
	if dailyRolling {
		fn := f.dir + "/" + f.filename + "." + f._date.Format(DATEFORMAT)
		fn_err := f.dir + "/" + f.filename_err + "." + f._date.Format(DATEFORMAT)

		if !isExist(fn) && !isExist(fn_err) && f.isMustRename() {
			if f.logfile != nil && f.logfile_err != nil {
				f.logfile.Close()
				f.logfile_err.Close()
			}
			err := os.Rename(f.dir+"/"+f.filename, fn)
			if err != nil {
				f.lg.Println("rename err", err.Error())
			}
			err = os.Rename(f.dir+"/"+f.filename_err, fn_err)
			t, _ := time.Parse(DATEFORMAT, time.Now().Format(DATEFORMAT))
			f._date = &t

			f.logfile, _ = os.Create(f.dir + "/" + f.filename)
			f.lg = log.New(LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
			f.logfile_err, _ = os.Create(f.dir + "/" + f.filename_err)
			f.lg_err = log.New(LogObj.logfile_err, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
		}
	} else {
		f.coverNextOne()
	}
}

func (f *FILE) nextSuffix() int {
	return int(f._suffix%int(maxFileCount) + 1)
}

func (f *FILE) coverNextOne() {
	f._suffix = f.nextSuffix()
	if f.logfile != nil {
		f.logfile.Close()
	}
	if isExist(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix))) {
		os.Remove(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix)))
	}
	os.Rename(f.dir+"/"+f.filename, f.dir+"/"+f.filename+"."+strconv.Itoa(int(f._suffix)))
	f.logfile, _ = os.Create(f.dir + "/" + f.filename)
	f.lg = log.New(LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
}

func fileSize(file string) int64 {
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

func fileMonitor() {
	timer := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-timer.C:
			fileCheck()
		}
	}
}

func fileCheck() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	if LogObj != nil && LogObj.isMustRename() {
		LogObj.mu.Lock()
		defer LogObj.mu.Unlock()
		LogObj.rename()
	}
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
