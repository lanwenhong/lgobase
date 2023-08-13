package dbpool

import (
	"database/sql"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
	"github.com/lanwenhong/lgobase/logger"
	dlog "gorm.io/gorm/logger"
)

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

func TestLoadDbconf(t *testing.T) {
	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("qf_push", tk, dbpool.USE_SQLX)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadOrmConfig(t *testing.T) {
	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuery(t *testing.T) {
	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("test1", tk, dbpool.USE_GORM)
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

	logger.SetConsole(true)
	logger.SetRollingDaily("./", "test.log", "test.log.err")
	loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	logger.SetLevel(loglevel)

	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  false,       // 禁用彩色打印
	}

	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(dconfig)

	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("test1", tk, dbpool.USE_GORM)

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
	logger.SetConsole(true)
	logger.SetRollingDaily("./", "test.log", "test.log.err")
	loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	logger.SetLevel(loglevel)

	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  false,       // 禁用彩色打印
	}

	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("test1", tk, dbpool.USE_GORM)
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
	user := User{}
	logger.SetConsole(true)
	logger.SetRollingDaily("./", "test.log", "test.log.err")
	loglevel, _ := logger.LoggerLevelIndex("DEBUG")
	logger.SetLevel(loglevel)

	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  false,       // 禁用彩色打印
	}

	db_conf := dbenc.DbConfNew("db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	dbs.SetormLog(dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add("test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	tdb.Where("name = ?", "wowow").First(&user)

	t.Log(user)
	c_tm := user.CreatedAt.Format("2006-01-02 15:04:05")
	t.Log(c_tm)
}