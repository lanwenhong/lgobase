package dbpool

import (
	"context"
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"strconv"
	"strings"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	dlog "gorm.io/gorm/logger"
)

const (
	USE_SQLX = iota
	USE_GORM
)

type Dbpool struct {
	Tset     *dbenc.DbConf
	Pools    map[string]*sqlx.DB
	OrmPools map[string]*gorm.DB

	//Logobj   *logger.FILE
	Logobj   *logger.Glog
	GormConf *dlog.Config
}

func DbpoolNew(conf *dbenc.DbConf) *Dbpool {
	dbpool := new(Dbpool)
	dbpool.Tset = conf
	dbpool.Pools = make(map[string]*sqlx.DB)
	dbpool.OrmPools = make(map[string]*gorm.DB)
	dbpool.Logobj = logger.Gfilelog
	dbpool.GormConf = logger.Gfilelog.LogormConf
	return dbpool
}

// func (dbpool *Dbpool) SetormLog(logobj *logger.FILE, gormConf *dlog.Config) {
func (dbpool *Dbpool) SetormLog(ctx context.Context, gormConf *dlog.Config) {
	dbpool.Logobj = logger.Gfilelog
	//dbpool.GormConf = gormConf
	if gormConf != nil {
		dbpool.GormConf = gormConf
	} else {
		dbpool.GormConf = logger.Gfilelog.LogormConf
	}
}

func (dbpool *Dbpool) Add(ctx context.Context, db string, url string, model int) error {
	xdata := strings.Split(url, "?")
	logger.Debugf(ctx, "xdata: %v", xdata)

	if len(xdata) != 2 {
		return errors.New("url err not have ? url=" + url)
	}
	params := xdata[1]
	logger.Debugf(ctx, "params: %s", params)

	pdata := strings.Split(params, "&")

	if len(pdata) != 2 {
		return errors.New("param err pamam=" + params)
	}

	maxopen := 100
	maxidle := 50
	var err error
	err = nil
	for _, item := range pdata {
		x := strings.Split(item, "=")
		if x[0] == "maxopen" {
			maxopen, err = strconv.Atoi(x[1])
		} else if x[0] == "maxidle" {
			maxidle, err = strconv.Atoi(x[1])
		} else {
			return errors.New("param err pamam=" + params)
		}
	}
	logger.Debugf(ctx, "maxopen: %d maxidle: %d", maxopen, maxidle)
	token_prefix := xdata[0]

	tdata := strings.Split(token_prefix, "://")
	if len(tdata) != 2 {
		return errors.New("token err token=" + token_prefix)
	}
	token := tdata[1]
	logger.Debugf(ctx, "token: %s", token)
	dbc := dbpool.Tset.DbConfReadGroup(token)
	dburl := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=true", dbc["user"], dbc["pswd"], dbc["host"], dbc["port"], dbc["dtbs"])
	logger.Debugf(ctx, "db url: %s", dburl)
	if model == USE_SQLX {
		dbpool.Pools[db], err = sqlx.Connect("mysql", dburl)

		if err == nil {
			dbpool.Pools[db].SetMaxOpenConns(maxopen)
			dbpool.Pools[db].SetMaxIdleConns(maxidle)
		}
	} else if model == USE_GORM {
		mylog := logger.New(dbpool.Logobj, *dbpool.GormConf)
		//mylog := logger.New(dbpool.Logobj, *logger.Glog.LogormConf)
		dbpool.OrmPools[db], err = gorm.Open(mysql.Open(dburl), &gorm.Config{Logger: mylog})
		if err == nil {
			sqlDB, _ := dbpool.OrmPools[db].DB()
			sqlDB.SetMaxOpenConns(maxopen)
			sqlDB.SetMaxIdleConns(maxidle)
		}
	}
	return err
}
