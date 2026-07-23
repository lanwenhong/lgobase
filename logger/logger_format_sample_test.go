package logger_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
)

func TestGenerateLoggerFormatSampleFiles(t *testing.T) {
	for _, name := range []string{
		"format_json.log",
		"format_json.log.err",
		"format_text.log",
		"format_text.log.err",
	} {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	ctx := context.WithValue(context.Background(), "trace_id", "trace-sample")

	logger.Newglog(".", "format_json.log", "format_json.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id,request_id",
	})
	logger.Debug(ctx, "json debug sample", "uid", 1001, "name", "alice")
	logger.Info(ctx, "json info sample", "uid", 1002, "cost_ms", 12.34)
	logger.Warn(ctx, "json warn sample", "uid", 1003, "reason", "slow")
	logger.Error(ctx, "json error sample", "uid", 1004, "err", "sample error")

	logger.Newglog(".", "format_text.log", "format_text.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           false,
		Colorful:         false,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		Format:           logger.TEXT_FORMAT,
		DesensitizeField: "password,cardNo",
	})
	logger.Debug(ctx, "text debug sample", "uid", 1001, "name", "alice")
	logger.Info(ctx, "text info sample", "uid", 1002)
	logger.Warn(ctx, "text warn sample", "uid", 1003, "reason", "slow")
	logger.Error(ctx, "text error sample", "uid", 1004, "err", "sample error")
	logger.Info(ctx, "text desensitize sample",
		"password", "secret-password",
		"cardNo", "6222021234567890123",
		"uid", 1005,
	)
}

func TestGenerateLoggerStdoutSample(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "trace-stdout")
	ctx = context.WithValue(ctx, "request_id", "request-stdout")

	fmt.Println()
	fmt.Println("========== JSON stdout, colorful=false ==========")
	logger.Newglog(t.TempDir(), "stdout_json.log", "stdout_json.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           true,
		Colorful:         false,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		DesensitizeField: "password,cardNo",
	})
	logger.Debug(ctx, "json stdout debug sample", "uid", 1001, "name", "alice")
	logger.Info(ctx, "json stdout info sample", "password", "secret-password", "cardNo", "6222021234567890123")
	logger.Warn(ctx, "json stdout warn sample", "reason", "slow")
	logger.Error(ctx, "json stdout error sample", "err", "sample error")

	fmt.Println()
	fmt.Println("========== JSON stdout, colorful=true ==========")
	logger.Newglog(t.TempDir(), "stdout_json_color.log", "stdout_json_color.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           true,
		Colorful:         true,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		DesensitizeField: "password,cardNo",
	})
	logger.Debug(ctx, "json colorful debug sample", "uid", 1001)
	logger.Info(ctx, "json colorful info sample", "password", "secret-password")
	logger.Warn(ctx, "json colorful warn sample", "reason", "slow")
	logger.Error(ctx, "json colorful error sample", "err", "sample error")

	fmt.Println()
	fmt.Println("========== TEXT stdout, colorful=false ==========")
	logger.Newglog(t.TempDir(), "stdout_text.log", "stdout_text.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           true,
		Colorful:         false,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		Format:           logger.TEXT_FORMAT,
		DesensitizeField: "password,cardNo",
	})
	logger.Debug(ctx, "text stdout debug sample", "uid", 1001, "name", "alice")
	logger.Info(ctx, "text stdout info sample", "uid", 1002)
	logger.Warn(ctx, "text stdout warn sample", "reason", "slow")
	logger.Error(ctx, "text stdout error sample", "password", "secret-password", "cardNo", "6222021234567890123")

	fmt.Println()
	fmt.Println("========== TEXT stdout, colorful=true ==========")
	logger.Newglog(t.TempDir(), "stdout_text_color.log", "stdout_text_color.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           true,
		Colorful:         true,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		Format:           logger.TEXT_FORMAT,
		DesensitizeField: "password,cardNo",
	})
	logger.Debug(ctx, "text colorful debug sample", "uid", 1001)
	logger.Info(ctx, "text colorful info sample", "password", "secret-password")
	logger.Warn(ctx, "text colorful warn sample", "reason", "slow")
	logger.Error(ctx, "text colorful error sample", "err", "sample error")
}
