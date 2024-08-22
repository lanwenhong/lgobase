package dbpool

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
	
	dlog "gorm.io/gorm/logger"
	
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
	"github.com/lanwenhong/lgobase/logger"
)

type Serverid struct {
	Svrid uint64 `gorm:"column:@@server_id"`
}

type UuidShort struct {
	UuidShort uint64 `gorm:"column:uuid_short()"`
}

type TT1 struct {
	Tid   int64  `gorm:"column:id"`
	Item1 string `gorm:"column:item1"`
	Item2 string `gorm:"column:item2"`
}

type User struct {
	ID        uint          `gorm:"primaryKey;AUTO_INCREMENT"`
	Name      string        `gorm:"type:varchar(200);column:name;index:idx_name"`
	Age       sql.NullInt64 `gorm:"column:age"`
	Mobile    string        `gorm:"type:varchar(16);column:name;index:idx_mobile,unique"`
	Sex       string        `gorm:"type:varchar(2);column:sex"`
	CreatedAt time.Time     `gorm:"column:createat;type:datetime(0)"`
	UpdatedAt time.Time     `gorm:"column:updateat;type:datetime(0)"`
	DeletedAt time.Time     `gorm:"column:deleteat;type:datetime(0)"`
}

func NewRequestID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

func TestLoadDbconf(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "qf_push", tk, dbpool.USE_SQLX)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadOrmConfig(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuery(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	t_ret := TT1{}
	tdb.First(&t_ret, 1)
	t.Log(t_ret.Tid)
	t.Log(t_ret.Item1)
	t.Log(t_ret.Item2)
}

func TestCreate(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  false,       // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	tdb.AutoMigrate(&User{})
	u := User{
		Name:      "wowow",
		Age:       sql.NullInt64{11, true},
		Mobile:    "18010583872",
		Sex:       "M",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: time.Now(),
	}
	tdb.Create(&u)
}

func TestCreateMulti(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  false,       // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	
	users := []User{
		{
			Name:      "wowow1",
			Age:       sql.NullInt64{13, true},
			Mobile:    "18010583873",
			Sex:       "F",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: time.Now(),
		},
		
		{
			Name:      "wowow2",
			Age:       sql.NullInt64{14, true},
			Mobile:    "18010583874",
			Sex:       "M",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: time.Now(),
		},
		
		{
			Name:      "wowow3",
			Age:       sql.NullInt64{15, true},
			Mobile:    "18010583875",
			Sex:       "M",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: time.Now(),
		},
	}
	tdb.Create(&users)
}

func TestQuery1(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	
	user := User{}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	tdb.WithContext(ctx).Where("name = ?", "wowow").First(&user)
	
	t.Log(user)
	c_tm := user.CreatedAt.Format("2006-01-02 15:04:05")
	t.Log(c_tm)
}

func TestQuery2(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	
	users := []User{}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	//tdb.WithContext(ctx).Where("name = ?", "wowow").First(&user)
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2023, 8, 12, 4, 12, 31, 0, time.Local)
	logger.Debug(ctx, "start: %s end: %s", start, end)
	tdb.WithContext(ctx).Where("createat between ? and ?", start, end).Find(&users)
	
	t.Log(users)
	//c_tm := user.CreatedAt.Format("2006-01-02 15:04:05")
	//t.Log(c_tm)
}

func TestGendiByDb(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	
	//users := []User{}
	s := []Serverid{}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	tdb.Raw("select @@server_id").Scan(&s)
	logger.Debugf(ctx, "serverid: %d", s[0].Svrid)
	
	su := []UuidShort{}
	tdb.Raw("select uuid_short()").Scan(&su)
	logger.Debugf(ctx, "su: %d", su[0].UuidShort)
	seq := su[0].UuidShort % 65535
	
	tt := time.Now().Unix()
	msec := tt * 1000
	logger.Debugf(ctx, "tt: %d", msec)
	
	id := uint64(msec)<<22 + s[0].Svrid<<16 + uint64(seq)
	logger.Debugf(ctx, "id: %d", id)
}

func TestQueryMap(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}
	
	//users := []User{}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	//logger.SetConsole(true)
	//logger.SetRollingDaily("./", "test.log", "test.log.err")
	//loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	//logger.SetLevel(loglevel)
	
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}
	
	db_conf := dbenc.DbConfNew(ctx, "db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://usercenter?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "usercenter", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["usercenter"]
	var ret []map[string]interface{}
	tdb.Raw("select id, username, FROM_UNIXTIME(ctime, '%Y-%m-%d %H:%i:%s') as ctime from users where id=?", 1).Scan(&ret)
	t.Log(ret)
	
	var re []map[string]interface{}
	var ilist interface{} = nil
	ilist = []int64{7133368332320837558, 7060864841283604340}
	tdb.Table("users").Select("*").Where("id in ?", ilist).Scan(&re)
	
	var re1 map[string]interface{}
	xid := 7133368332320837558
	var iid interface{} = nil
	iid = xid
	tdb.Table("users").Select("*").Where("id = ?", iid).Find(&re1)
	t.Log(re1)
}
