package logger

import (
	"context"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"gorm.io/gorm"
	dlog "gorm.io/gorm/logger"
)

func testdb() (string, int64) {
	return "ssssssss", 1
}

func TestDblog(t *testing.T) {

	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mylog := logger.New(nil, d_conf)
	mylog.Info(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Warn(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Error(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)

}

func TestDbColorlog(t *testing.T) {

	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mylog := logger.New(nil, d_conf)
	mylog.Info(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Warn(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Error(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
}

func TestDbTraceErrLog(t *testing.T) {
	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mylog := logger.New(nil, d_conf)
	stringTime := "2017-08-30 16:40:41"
	loc, _ := time.LoadLocation("Local")
	begin, _ := time.ParseInLocation("2006-01-02 15:04:05", stringTime, loc)

	mylog.Trace(ctx, begin, testdb, gorm.ErrInvalidField)

}

func TestDBTraceWarnLog(t *testing.T) {
	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mylog := logger.New(nil, d_conf)
	stringTime := "2017-08-30 16:40:41"
	loc, _ := time.LoadLocation("Local")
	begin, _ := time.ParseInLocation("2006-01-02 15:04:05", stringTime, loc)

	mylog.Trace(ctx, begin, testdb, nil)
	mylog.Trace(ctx, time.Now(), testdb, nil)
}

/*func TestLog(t *testing.T) {
	logger.SetConsole(true)
	logger.SetRollingDaily("./", "test.log", "test.log.err")
	loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	logger.SetLevel(loglevel)
	logger.Debugf("this is debug")
	logger.Debug("this is debug")

}*/

func TestLogfile(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAYLY,
		Stdout:       false,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	logger.Debug("it is a test")
	logger.Info("it is a test")
	logger.Warn("it is a test")
	logger.Warnf("it is a test")
	logger.Error("it is a error")
}

func TestLogRatate(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAYLY,
		Stdout:       false,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	logger.Info("test")
	logger.Info("test")
	logger.Info("test")
	logger.Warn("test")

	time.Sleep(2 * time.Second)
	stringTime := "2023-08-13 16:40:41"
	loc, _ := time.LoadLocation("Local")
	rotate, _ := time.ParseInLocation("2006-01-02 15:04:05", stringTime, loc)
	logger.Gfilelog.LogObj.Ldate = &rotate

	logger.Info("rotate")
	logger.Warn("rotate")
}
