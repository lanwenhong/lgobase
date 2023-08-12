package logger

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
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
	ColorFull   bool
	Loglevel    int
}

type Glog struct {
	LogObj  *FILE
	Logconf *Glogconf
}

var Gfilelog *Glog = nil

func Newglog(fileDir string, fileName string, fileNameErr string, glog_conf *Glogconf) *Glog {
	glog := &Glog{
		Logconf: glog_conf,
	}
	if glog_conf.RotateMethod == ROTATE_FILE_NUM {
		glog.SetRollingFile(fileDir, fileName, glog_conf.Stdout)
	} else {
		glog.SetRollingDaily(fileDir, fileName, fileNameErr, glog_conf.Stdout)
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
			glog.LogObj.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
		} else {
			glog.LogObj.lg = log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
		}
	} else {
		glog.rename()
	}
	go glog.fileMonitor()

	return nil
}

func (glog *Glog) SetRollingDaily(fileDir, fileName, fileName_err string, stdout bool) {
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
			fmt.Println("==0000000000000===")
			glog.LogObj.logfile, _ = os.OpenFile(fileDir+"/"+fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			//glog.LogObj.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Llongfile|log.LstdFlags)
			glog.LogObj.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.LstdFlags)

			glog.LogObj.logfile_err, _ = os.OpenFile(fileDir+"/"+fileName_err, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			glog.LogObj.lg_err = log.New(glog.LogObj.logfile_err, "", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Llongfile|log.LstdFlags)
		} else {
			glog.LogObj.lg = log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
		}
	} else {
		glog.rename()
	}

}

func catchError() {
	if err := recover(); err != nil {
		log.Println("err", err)
	}
}

func Debug(fmtstr string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		if Gfilelog.LogObj != nil {
			Gfilelog.LogObj.mu.RLock()
			defer Gfilelog.LogObj.mu.RUnlock()
		}
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.ColorFull && Gfilelog.Logconf.Loglevel <= DEBUG {
			Gfilelog.LogObj.lg.Printf(Green+"[DEBUG] "+fmtstr+Reset, v...)
		} else if Gfilelog.Logconf.Loglevel <= DEBUG {
			Gfilelog.LogObj.lg.Printf("[DEBUG] "+fmtstr, v...)
		}
	}
}

func Debugf(fmtstr string, v ...interface{}) {
	Debug(fmtstr, v...)
}

func Info(fmtstr string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		Gfilelog.LogObj.mu.RLock()
		defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.ColorFull && Gfilelog.Logconf.Loglevel <= INFO {
			Gfilelog.LogObj.lg.Printf(Green+"[INFO] "+fmtstr+Reset, v...)
		} else if Gfilelog.Logconf.Loglevel <= INFO {
			Gfilelog.LogObj.lg.Printf("[INFO] "+fmtstr, v...)
		}
	}
}

func Infof(fmtstr string, v ...interface{}) {
	Info(fmtstr, v...)
}

func Warn(fmtstr string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		Gfilelog.LogObj.mu.RLock()
		defer Gfilelog.LogObj.mu.RUnlock()

		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.ColorFull && Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Printf(Yellow+"[WARN] "+fmtstr+Reset, v...)
		} else if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Printf("[WARN] "+fmtstr, v...)
		} else if Gfilelog.Logconf.Loglevel <= WARN {
			Gfilelog.LogObj.lg.Printf("[WARN] "+fmtstr, v...)
			Gfilelog.LogObj.lg_err.Printf("[WARN] "+fmtstr, v...)
		}
	}
}
func Warnf(fmtstr string, v ...interface{}) {
	Warn(fmtstr, v...)
}

func Error(fmtstr string, v ...interface{}) {
	if Gfilelog != nil && Gfilelog.LogObj != nil {
		Gfilelog.fileCheck()
		Gfilelog.LogObj.mu.RLock()
		defer Gfilelog.LogObj.mu.RUnlock()
		if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.ColorFull && Gfilelog.Logconf.Loglevel <= ERROR {
			Gfilelog.LogObj.lg.Printf(Red+"[ERROR] "+fmtstr+Reset, v...)

		} else if Gfilelog.Logconf.Stdout && Gfilelog.Logconf.Loglevel <= ERROR {
			Gfilelog.LogObj.lg.Printf("[ERROR] "+fmtstr, v...)
		} else if Gfilelog.Logconf.Loglevel <= ERROR {
			Gfilelog.LogObj.lg.Printf("[ERROR] "+fmtstr, v...)
			Gfilelog.LogObj.lg_err.Printf("[ERROR] "+fmtstr, v...)

		}
	}
}

func Errorf(fmtstr string, v ...interface{}) {
	Error(fmtstr, v...)
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
	f := glog.LogObj
	if glog.Logconf.RotateMethod == ROTATE_FILE_DAILY {
		fn := f.dir + "/" + f.filename + "." + f.Ldate.Format(DATEFORMAT)
		fn_err := f.dir + "/" + f.filename_err + "." + f.Ldate.Format(DATEFORMAT)

		if !isExist(fn) && !isExist(fn_err) && glog.isMustRename() {
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
			f.Ldate = &t

			f.logfile, _ = os.Create(f.dir + "/" + f.filename)
			f.lg = log.New(glog.LogObj.logfile, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
			f.logfile_err, _ = os.Create(f.dir + "/" + f.filename_err)
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
	if isExist(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix))) {
		os.Remove(f.dir + "/" + f.filename + "." + strconv.Itoa(int(f._suffix)))
	}
	os.Rename(f.dir+"/"+f.filename, f.dir+"/"+f.filename+"."+strconv.Itoa(int(f._suffix)))
	f.logfile, _ = os.Create(f.dir + "/" + f.filename)
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
