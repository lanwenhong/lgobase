package confparse

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"
	
	"github.com/lanwenhong/lgobase/gconfig"
)

func writeTestIni(fileName string) {
	// 创建一个模拟的文件内容
	fileContent := `
[test]
cache=1h2s
cnt=1
name=test
`
	// 将读取器模拟为文件
	f, _ := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	_, _ = f.Write([]byte(fileContent))
	_, _ = f.Seek(0, io.SeekStart)
}

func TestGconfParse(t *testing.T) {
	type confTest struct {
		Cache time.Duration `confpos:"test:cache" dtype:"base"`
		Cnt   int           `confpos:"test:cnt" dtype:"base"`
		Name  string        `confpos:"test:name" dtype:"base"`
	}
	
	mockFileName := "/tmp/lgobase_test_mock.ini"
	writeTestIni(mockFileName)
	conf := &confTest{}
	cfg := gconfig.NewGconf(mockFileName)
	if err := cfg.GconfParse(); err != nil {
		t.Error(err)
	}
	if err := CpaseNew("").CparseGo(conf, cfg); err != nil {
		t.Error(err)
	}
	
	fmt.Println(conf)
	if conf.Cache != time.Hour*1+time.Second*2 {
		t.Error("duration parse error")
	}
	
	if conf.Cnt != 1 {
		t.Error("int parse error")
	}
	
	if conf.Name != "test" {
		t.Error("string parse error")
	}
}
