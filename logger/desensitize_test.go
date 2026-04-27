package logger

import (
	"context"
	"strings"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

type TestStru struct {
	CardNo string
}

func TestDesensitizeComplex(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Colorful:         true,
		Stdout:           true,
		Loglevel:         logger.DEBUG,
		DesensitizeField: "xx,password,cardNo,CardNo",
	}
	log := logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	log.RegisterDesensitizeFunc(ctx, "cardNo", func(v any) any {
		s := v.(string)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "-", "")

		if len(s) <= 10 {
			return s // 太短不脱敏
		}
		pre := s[:6]
		suf := s[len(s)-4:]
		return pre + "******" + suf
	})

	log.RegisterDesensitizeFunc(ctx, "CardNo", func(v any) any {
		s := v.(string)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "-", "")

		if len(s) <= 10 {
			return s // 太短不脱敏
		}
		pre := s[:6]
		suf := s[len(s)-4:]
		return pre + "******" + suf
	})

	ts := TestStru{
		CardNo: "11111111111111111111",
	}
	logger.Info(ctx, "全结构脱敏测试（含slice）",
		// 普通字段
		"phone", "13811112222",
		"password", "testpwd",
		"xx", "111111",

		"testStru", ts,
		// Map
		"user", map[string]any{
			"username": "test",
			"phone":    "13933334444",
			"xx":       "111",
			"info": map[string]any{
				"token": "abcd1234",
			},
			//"jsonStr": `{"phone":"13888889999","password":"mypass", "xx": 111}`,
			"jsonStr": `{"phone": "13888889999", "password": "mypass", "xx": 111, "fff": [{"xx": 111}]}`,
		},

		// ===================== Slice 测试（关键） =====================
		"userList", []map[string]any{
			{"name": "张三", "phone": "13800001111", "password": "123"},
			{"name": "李四", "phone": "13800002222", "idCard": "110123456"},
		},

		// JSON 字符串
		"jsonStr", `{"phone":"13888889999","password":"mypass", "xx": 111}`,

		// XML
		"xmlStr", `<user><name>张三</name><phone>13877778888</phone><password>ppp</password><cardNo>6222021234567890123</cardNo></user>`,
	)
}

func TestDesensitizeTradeData(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Colorful:         true,
		Stdout:           true,
		Loglevel:         logger.DEBUG,
		DesensitizeField: "expired_date,iccdata,trackData,cardNo",
	}
	log := logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", util.NewRequestID())
	log.RegisterDesensitizeFunc(ctx, "cardNo", func(v any) any {
		s := v.(string)
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "-", "")

		if len(s) <= 10 {
			return s // 太短不脱敏
		}
		pre := s[:6]
		suf := s[len(s)-4:]
		return pre + "******" + suf
	})
	//trade := `{"account_device_id":"25CXCASY4355","app_name":"hjsh","application_label":"VISA CREDIT","appver":"4.35.10.129","busicd":"802808","businm":"","cardaid":"A0000000031010","cardbin":"433668","cardscheme":"VISA","clientip":"10.1.39.26","clisn":"063267","clitm":"2026-03-30 17:34:27","contact":"711300115250001","domain":"api-int.qfapi.com","entry_mode":"072","extend_info":{"cardseqnum":"001","entry_mode":"072","expired_date":"2202","iccdata":"9F2608E0277F69509ECF889F2701809F100706010A03A000009F3704C6EAF7C19F36020BDD950500000000009A032603309C01009F02060000000896005F2A020344820220009F1A0203449F3303E0B8C89F3501228407A00000000310109F0902008C9B0200009F34031E03009F1E0843415359343335359F03060000000000005F340101","macString":"0E6D6C6E922F36B9","pinBlock":"","trackData":"02AAD53F8658315FD40B0D8D3C541EB26BEC38340AC89646"},"lnglat":["0","0"],"network":"wifi","opuid":"","os":"Android","osver":"10","phonemodel":"A8S","requestid":"3BexL8Heh3k4UKcJnBRzhNchTrH","sdk_type":"qfpay_sdk","sdk_version":"v1.0.10_test_2","terminal_entry_capability":"E0277F69509ECF88","terminalid":"25CXCASY4355","tip_amt":"","trade_type":"trade","txamt":"89600","txcurrcd":"344","txdtm":"2026-03-30 17:34:27","txzone":"+0800","udid":"860568073578747","userid":"1130011527"}`
	trade := `{"cardNo": "6222021234567890123", "entry_mode": "072", "userid": "1130011527", "domain": "api-int.qfapi.com", "tip_amt": "", "app_name": "hjsh", "txdtm": "2026-03-30 17:34:27", "txcurrcd": "344", "businm": "", "sdk_type": "qfpay_sdk", "lnglat": ["0", "0"], "cardaid": "A0000000031010", "appver": "4.35.10.129", "trade_type": "trade", "requestid": "3BexL8Heh3k4UKcJnBRzhNchTrH", "phonemodel": "A8S", "txzone": "+0800", "clientip": "10.1.39.26", "cardscheme": "VISA", "terminal_entry_capability": "E0277F69509ECF88", "udid": "860568073578747", "network": "wifi", "application_label": "VISA CREDIT", "opuid": "", "terminalid": "25CXCASY4355", "cardbin": "433668", "txamt": "89600", "busicd": "802808", "clisn": "063267", "sdk_version": "v1.0.10_test_2", "contact": "711300115250001", "extend_info":"{\"cardseqnum\":\"001\",\"entry_mode\":\"072\",\"expired_date\":\"2202\",\"iccdata\":\"9F2608E0277F69509ECF889F2701809F100706010A03A000009F3704C6EAF7C19F36020BDD950500000000009A032603309C01009F02060000000896005F2A020344820220009F1A0203449F3303E0B8C89F3501228407A00000000310109F0902008C9B0200009F34031E03009F1E0843415359343335359F03060000000000005F340101\",\"macString\":\"0E6D6C6E922F36B9\",\"pinBlock\":\"\",\"trackData\":\"02AAD53F8658315FD40B0D8D3C541EB26BEC38340AC89646\"}", "account_device_id": "25CXCASY4355", "osver": "10", "clitm": "2026-03-30 17:34:27", "os": "Android"}`
	/*var obj map[string]any
	err := json.Unmarshal([]byte(trade), &obj)
	if err != nil {
		logger.Warn(ctx, "test", "err", err.Error())
	}
	logger.Info(ctx, "test", "obj", obj)*/
	logger.Info(ctx, "脱敏交易数据", "tradeData", trade, "cardNo", "6222021234567890123")
}
