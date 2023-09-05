package gconfig

import (
	"testing"

	"github.com/lanwenhong/lgobase/gconfig"
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
		} else {
			t.Errorf("test3 not found")
		}

	} else {
		t.Errorf("section1 not found")
	}
}
