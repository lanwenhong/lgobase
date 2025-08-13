package gconfig

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
)

func TestGparse(t *testing.T) {
	t.Log("start")
	g_cf := gconfig.NewGconf("test1.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}
	if se, ok := g_cf.Gcf["section1"]; ok {
		if test1, ok := se["test1"]; ok {
			t.Logf("get test1: %s", test1)
		} else {
			t.Errorf("test1 not found")
		}

		if test2, ok := se["test2"]; ok {
			t.Logf("get test2: %s", test2)
		} else {
			t.Errorf("test2 not found")
		}

		if test3, ok := se["test3"]; ok {
			t.Logf("get test3: %s", test3)
			if ex, ok := g_cf.GlineExtend["test3"]; ok {
				t.Logf("get test3 ex: %s", ex)
			} else {
				t.Errorf("test3 extend not found")
			}
		} else {
			t.Errorf("test3 not found")
		}

	} else {
		t.Errorf("section1 not found")
	}
}

func buildRule(ctx context.Context, ex map[string]map[string][]string) {
	logger.Debugf(ctx, "ex: %v", ex)

	//logger.Debugf(ctx, "when: %s", ex[0])
	//logger.Debugf(ctx, "Salience: %s", ex[1])
}

func TestRule(t *testing.T) {
	ctx := context.Background()
	t.Log("start")
	g_cf := gconfig.NewGconf("test_rule.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}
	//g_cf.AddExd("test1", "test1 = 192.168.100.105/1000")
	if _, ok := g_cf.Gcf["section1"]; ok {
		buildRule(ctx, g_cf.GlineExtend)
	}
}
