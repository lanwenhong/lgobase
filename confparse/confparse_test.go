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
		logger.Debugf(ctx, "cfg: %v", cfg)
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
	logger.Debugf(ctx, "dayuconf: %v", dayuconf)
	/*logger.Debugf(ctx, "payserver0: %v", tConf.Cfg.Gcf["section1"]["pay_server"][0])
	k := "pay_server = " + tConf.Cfg.Gcf["section1"]["pay_server"][0]
	logger.Debugf(ctx, "payserver0 conf: %v", tConf.Cfg.GlineExtend["pay_server"][k])*/
}
