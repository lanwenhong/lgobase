package util

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
	"github.com/lanwenhong/lgobase/util"
)

func TestGenid(t *testing.T) {

	id := util.GenXid()
	t.Log(id)
	id = util.GenKsuid()
	t.Log(id)

	id = util.GenBetterGUID()
	t.Log(id)

	id = util.GenUlid()
	t.Log(id)

	id = util.GenSonyflake()
	t.Log(id)

	id = util.GenSid()
	t.Log(id)

}

func TestGenidFromDB(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "1111")

	/*myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}

	logger.Newglog("./", "test.log", "test.log.err", myconf)
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}*/

	db_conf := dbenc.DbConfNew(ctx, "/home/lanwenhong/dev/go/lgobase/dbpool/db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	//dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	id, err := util.Genid(ctx, tdb)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(id)
}
