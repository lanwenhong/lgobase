package logger

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func TestDesensitizeComplex(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Colorful:         true,
		Stdout:           true,
		Loglevel:         logger.DEBUG,
		DesensitizeField: "xx,password",
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	logger.Info(ctx, "全结构脱敏测试（含slice）",
		// 普通字段
		"phone", "13811112222",
		"password", "testpwd",
		"xx", "111111",

		// Map
		"user", map[string]any{
			"username": "test",
			"phone":    "13933334444",
			"xx":       "111",
			"info": map[string]any{
				"token": "abcd1234",
			},
		},

		// ===================== Slice 测试（关键） =====================
		"userList", []map[string]any{
			{"name": "张三", "phone": "13800001111", "password": "123"},
			{"name": "李四", "phone": "13800002222", "idCard": "110123456"},
		},

		// JSON 字符串
		"jsonStr", `{"phone":"13888889999","password":"mypass", "xx": 111}`,

		// XML
		"xmlStr", `<user><name>张三</name><phone>13877778888</phone><password>ppp</password></user>`,
	)
}
