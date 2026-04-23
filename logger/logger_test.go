package logger

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
	"github.com/lanwenhong/lgobase/logger"
	"gorm.io/gorm"
	dlog "gorm.io/gorm/logger"
)

func NewRequestID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

func testdb() (string, int64) {
	return "ssssssss", -1
}

func TestDblog(t *testing.T) {

	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//CtxValueKey:  "trace_id,request_id",
	}

	mylog := logger.New(logger.Newglog("./", "test.log", "test.log.err", myconf), d_conf)
	mylog.Info(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)
	mylog.Warn(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)
	mylog.Error(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)

	logger.Debugf(ctx, "test test test")

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

	mylog := logger.New(logger.NewDefaultGLog(), d_conf)
	mylog.Info(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)
	mylog.Warn(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)
	mylog.Error(ctx, "info test", "db1", 1, "db2", 2, "db3", 3)
}

func TestDbTraceErrLog(t *testing.T) {
	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	//ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	ctx := context.Background()

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id,request_id",
	}

	mylog := logger.New(logger.Newglog("./", "test.log", "test.log.err", myconf), d_conf)
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
		Colorful:                  false,
	}
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//CtxValueKey:  "trace_id,request_id",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mylog := logger.New(logger.Newglog("./", "test.log", "test.log.err", myconf), d_conf)
	stringTime := "2017-08-30 16:40:41"
	loc, _ := time.LoadLocation("Local")
	begin, _ := time.ParseInLocation("2006-01-02 15:04:05", stringTime, loc)

	mylog.Trace(ctx, begin, testdb, nil)
	mylog.Trace(ctx, time.Now(), testdb, nil)
	logger.Debug(ctx, "haha", "mytest", "test test test")
}

func TestDbWithfilelog(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	d_conf := dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mylog := logger.New(logger.Gfilelog, d_conf)
	mylog.Info(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Warn(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)
	mylog.Error(ctx, "xxxxxxxx%d%d%d", 1, 2, 3)

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
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	logger.Newglog("./", "test.log", "test.log.err", myconf)
	logger.Debugf(ctx, "it is a test")
	logger.Debugf(ctx, "it is a test")
	logger.Infof(ctx, "it is a test")
	logger.Infof(ctx, "it is a test")

	go func() {
		ctxx := context.WithValue(ctx, "trace_id", NewRequestID())
		logger.Debugf(ctxx, "child log")
	}()
	time.Sleep(2 * time.Second)
	logger.Warnf(ctx, "it is a test")
	logger.Warnf(ctx, "it is a test")
	logger.Errorf(ctx, "it is a error")
	logger.Errorf(ctx, "d=%d", 1)
}

func TestLogRatate(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db_conf := dbenc.DbConfNew(ctx, "../dbpool/db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	//dbs.SetormLog(ctx, db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "usercenter", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["usercenter"]
	var ret []map[string]interface{}
	tdb.Raw("select id, username, FROM_UNIXTIME(ctime, '%Y-%m-%d %H:%i:%s') as ctime from users where id=?", 1).Scan(&ret)
	//t.Log(ret)

	//logger.Debugf(ctx, 11, 12, 13)
	//logger.Info("test")
	//logger.Info("test")
	//logger.Warn("test")

	time.Sleep(2 * time.Second)
	stringTime := "2024-11-04 16:40:41"
	loc, _ := time.LoadLocation("Local")
	rotate, _ := time.ParseInLocation("2006-01-02 15:04:05", stringTime, loc)
	logger.Gfilelog.LogObj.Ldate = &rotate

	tdb.Raw("select id, username, FROM_UNIXTIME(ctime, '%Y-%m-%d %H:%i:%s') as ctime from users where id=?", 1).Scan(&ret)
	//logger.Debugf(ctx, 33, 44, "jjy")
	//logger.Warn(ctx, "rotate")
}

func TestLogDebug(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Colorful:         true,
		Stdout:           true,
		Loglevel:         logger.DEBUG,
		DesensitizeField: "xx",
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	//ctx := context.Background()
	logger.Debug(ctx, "test", "userid", 1, "userid2", 2)
	//logger.Debugf(ctx, "%s %d %d", "xxx", 1, 3)

	logger.Info(ctx, "test", "xx", "jujingyi", "uid", 1)
	//logger.Infof(ctx, "%s %s %d %d %d", "liushishi", "jujingyi", 111, 2, 3)

	logger.Warn(ctx, "test", "ffff", 12)
	//logger.Warnf(ctx, "%s %d %d", "ffff", 12, 13)

	logger.Error(ctx, "test", "xx", "jujingyi", "uid", 2)
	//logger.Errorf(ctx, "%s %s %d %d %d", "liushishi", "jujingyi", 111, 2, 3)
}
