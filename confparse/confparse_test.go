package confparse

import (
	"context"
	"strconv"
	"testing"

	"github.com/lanwenhong/lgobase/confparse"
	"github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
)

type PaySever struct {
	Rule    string `confpos:"seciont1:pay_server:0"`
	Salence int    `confpos:"seciont1:pay_server:1"`
}

type TestConf struct {
	TestA      int      `confpos:"section2:a" dtype:"base"`
	TestB      int      `confpos:"section2:b" dtype:"base"`
	TestC      int      `confpos:"section2:b" dtype:"complex"`
	Dayu       PaySever `confpos:"section1:pay_server" dtype:"complex"`
	BackRouter PaySever `confpos:"section2:pay_server" dtype:"complex"`
	Cfg        *gconfig.Gconf
}

type TestExtConf struct {
	MaxConnLife     int64  `mapkey:"MaxConnLife"`
	MaxConns        int    `mapkey:"MaxConns"`
	MaxIdleConnLife int    `mapkey:"MaxIdleConnLife"`
	MaxIdleConns    uint   `mapkey:"MaxIdleConns"`
	Salience        int    `mapkey:"Salience"`
	Proto           string `mapkey:"proto"`
}

func TestLoadConf(t *testing.T) {
	ctx := context.Background()
	filename := "test_rule.ini"
	cfg := gconfig.NewGconf(filename)
	err := cfg.GconfParse()
	if err != nil {
		t.Fatal(err)
	}
	cp := confparse.CpaseNew(filename)
	cp.Funcs["seciont3:c"] = func(ctx context.Context, stru interface{}, s []string) error {
		tConf := stru.(*TestConf)
		x, _ := strconv.ParseInt(s[0], 10, 64)
		tConf.TestC = int(x)
		logger.Debug(ctx, "test", "cfg", cfg)
		return nil
	}

	tConf := &TestConf{
		Cfg: cfg,
	}
	//err = cp.CparseGoWithCtx(ctx, tConf, cfg)
	err = cp.WithCtx(ctx).CparseGo(tConf, cfg)
	//err = cp.CparseGo(tConf, cfg)

	if err != nil {
		t.Fatal(err)
	}
	logger.Debugf(ctx, "conf: %v", tConf)
	dayuconf, _ := confparse.ParseExt(ctx, "section1", "pay_server", 0, cfg)
	logger.Debug(ctx, "conf test", "dayuconf", dayuconf)
	/*logger.Debugf(ctx, "payserver0: %v", tConf.Cfg.Gcf["section1"]["pay_server"][0])
	k := "pay_server = " + tConf.Cfg.Gcf["section1"]["pay_server"][0]
	logger.Debugf(ctx, "payserver0 conf: %v", tConf.Cfg.GlineExtend["pay_server"][k])*/
}

func TestLoadExtConf(t *testing.T) {
	ctx := context.Background()
	filename := "test_rule.ini"
	cfg := gconfig.NewGconf(filename)
	err := cfg.GconfParse()
	if err != nil {
		t.Fatal(err)
	}
	pay_server := cfg.GlineExtend["pay_server"]
	for k, _ := range pay_server {
		logger.Debug(ctx, "conf test", "k", k)
		ec := confparse.NewExtendConf("pay_server", k, 0)
		obj := &TestExtConf{}
		err = ec.ParseExtStru(ctx, obj, cfg)
		if err != nil {
			t.Fatal(err)
		}
		logger.Debug(ctx, "extend conf test", "obj", obj)
		//初始化gpool
	}
}
