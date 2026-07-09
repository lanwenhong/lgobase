package logger_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
)

func readLastLogLine(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("empty log file %s", path)
	}
	return lines[len(lines)-1]
}

func TestGlogconfDefaultFormatWritesJSON(t *testing.T) {
	dir := t.TempDir()
	fileName := "default_json.log"
	logger.Newglog(dir, fileName, "default_json.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-json")
	logger.Info(ctx, "json hello", "uid", 7)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("default log format should be json, got %q: %v", line, err)
	}
	if fields["msg"] != "json hello" {
		t.Fatalf("msg = %v", fields["msg"])
	}
	if fields["trace_id"] != "trace-json" {
		t.Fatalf("trace_id = %v", fields["trace_id"])
	}
	if fields["uid"] != float64(7) {
		t.Fatalf("uid = %v", fields["uid"])
	}
}

func TestGlogconfJSONRenamesBuiltinTimeKey(t *testing.T) {
	dir := t.TempDir()
	fileName := "json_time_key.log"
	logger.Newglog(dir, fileName, "json_time_key.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})

	logger.Info(context.Background(), "json time key sample", "uid", 7)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("default log format should be json, got %q: %v", line, err)
	}
	if _, ok := fields["log_time"]; !ok {
		t.Fatalf("json log should contain log_time, got %q", line)
	}
	if _, ok := fields["time"]; ok {
		t.Fatalf("builtin slog time key should be renamed, got %q", line)
	}
}

func TestGlogconfJSONKeepsBusinessTimeAttr(t *testing.T) {
	dir := t.TempDir()
	fileName := "json_business_time.log"
	logger.Newglog(dir, fileName, "json_business_time.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})

	logger.Info(context.Background(), "json business time sample", "time", "business-time")

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("default log format should be json, got %q: %v", line, err)
	}
	if _, ok := fields["log_time"]; !ok {
		t.Fatalf("json log should contain log_time, got %q", line)
	}
	if fields["time"] != "business-time" {
		t.Fatalf("business time attr should stay as time, got %q", line)
	}
}

func TestGlogconfTextFormatWritesText(t *testing.T) {
	dir := t.TempDir()
	fileName := "text.log"
	logger.Newglog(dir, fileName, "text.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id,request_id",
		Format:       logger.TEXT_FORMAT,
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-text")
	logger.Info(ctx, "text hello", "uid", 7)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	if json.Valid([]byte(line)) {
		t.Fatalf("text log format should not be json: %q", line)
	}
	for _, want := range []string{"trace-text - [INFO] text hello", "uid=7"} {
		if !strings.Contains(line, want) {
			t.Fatalf("text log line %q does not contain %q", line, want)
		}
	}
	if strings.Contains(line, "trace_id=trace-text") {
		t.Fatalf("text log ctx fields should be written before message, got %q", line)
	}
}

func TestGlogconfTextFormatRespectsLogLevel(t *testing.T) {
	dir := t.TempDir()
	fileName := "text_level.log"
	logger.Newglog(dir, fileName, "text_level.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.INFO,
		CtxValueKey:  "trace_id",
		Format:       logger.TEXT_FORMAT,
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-level")
	logger.Debug(ctx, "debug hidden", "uid", 7)
	logger.Info(ctx, "info shown", "uid", 8)

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "debug hidden") {
		t.Fatalf("debug log should be filtered when level is INFO: %q", logs)
	}
	if !strings.Contains(logs, "info shown") {
		t.Fatalf("info log should be written when level is INFO: %q", logs)
	}
}

func TestGlogconfJSONFileRespectsInfoLogLevel(t *testing.T) {
	dir := t.TempDir()
	fileName := "json_info_level.log"
	logger.Newglog(dir, fileName, "json_info_level.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.INFO,
		CtxValueKey:  "trace_id",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-json-level")
	logger.Debug(ctx, "json debug hidden", "uid", 7)
	logger.Info(ctx, "json info shown", "uid", 8)

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "json debug hidden") {
		t.Fatalf("json debug log should be filtered when level is INFO: %q", logs)
	}
	if !strings.Contains(logs, "json info shown") {
		t.Fatalf("json info log should be written when level is INFO: %q", logs)
	}
}

func TestNewglogJSONLevelDoesNotInheritPreviousLogger(t *testing.T) {
	firstDir := t.TempDir()
	logger.Newglog(firstDir, "first_debug.log", "first_debug.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})

	secondDir := t.TempDir()
	fileName := "second_info.log"
	logger.Newglog(secondDir, fileName, "second_info.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.INFO,
		CtxValueKey:  "trace_id",
	})

	logger.Debug(context.Background(), "newglog inherited debug hidden", "uid", 7)
	logger.Info(context.Background(), "newglog info shown", "uid", 8)

	data, err := os.ReadFile(filepath.Join(secondDir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "newglog inherited debug hidden") {
		t.Fatalf("new INFO logger should not inherit previous DEBUG level, got %q", logs)
	}
	if !strings.Contains(logs, "newglog info shown") {
		t.Fatalf("new INFO logger should keep info logs, got %q", logs)
	}
}

func TestSetLevelUpdatesActiveLogger(t *testing.T) {
	dir := t.TempDir()
	fileName := "set_level.log"
	logger.Newglog(dir, fileName, "set_level.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})
	t.Cleanup(func() {
		logger.SetLevel(logger.DEBUG)
	})

	logger.SetLevel(logger.INFO)
	logger.Debug(context.Background(), "set level debug hidden", "uid", 7)
	logger.Debugf(context.Background(), "set level debugf hidden uid=%d", 8)
	logger.Info(context.Background(), "set level info shown", "uid", 9)

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "set level debug hidden") || strings.Contains(logs, "set level debugf hidden") {
		t.Fatalf("SetLevel(INFO) should hide debug logs, got %q", logs)
	}
	if !strings.Contains(logs, "set level info shown") {
		t.Fatalf("SetLevel(INFO) should keep info logs, got %q", logs)
	}
}

func TestGlogconfJSONColorfulStdoutRespectsInfoLogLevel(t *testing.T) {
	dir := t.TempDir()
	stdoutPath := filepath.Join(dir, "stdout.log")
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()

	originalStdout := os.Stdout
	os.Stdout = stdoutFile
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	logger.Newglog(dir, "json_stdout_info_level.log", "json_stdout_info_level.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.INFO,
		CtxValueKey:  "trace_id",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-json-stdout-level")
	logger.Debug(ctx, "json colorful stdout debug hidden", "uid", 7)
	logger.Info(ctx, "json colorful stdout info shown", "uid", 8)

	data, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "json colorful stdout debug hidden") {
		t.Fatalf("json colorful stdout debug log should be filtered when level is INFO: %q", logs)
	}
	if !strings.Contains(logs, "json colorful stdout info shown") {
		t.Fatalf("json colorful stdout info log should be written when level is INFO: %q", logs)
	}
}

func TestGlogconfTextFormatDebugfFormatsMessage(t *testing.T) {
	dir := t.TempDir()
	fileName := "text_debugf.log"
	logger.Newglog(dir, fileName, "text_debugf.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
		Format:       logger.TEXT_FORMAT,
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-debug")
	logger.Debugf(ctx, "debug number %d", 9)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	if strings.Contains(line, "%!(EXTRA") {
		t.Fatalf("debugf text log should not contain fmt extra marker: %q", line)
	}
	for _, want := range []string{"trace-debug [DEBUG] debug number 9"} {
		if !strings.Contains(line, want) {
			t.Fatalf("debugf text log line %q does not contain %q", line, want)
		}
	}
}

func TestGlogconfTextFormatUsesPlaceholdersForMissingCtx(t *testing.T) {
	dir := t.TempDir()
	fileName := "text_missing_ctx.log"
	logger.Newglog(dir, fileName, "text_missing_ctx.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id,request_id",
		Format:       logger.TEXT_FORMAT,
	})

	logger.Debug(context.Background(), "missing ctx sample", "uid", 7)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	for _, want := range []string{"- - [DEBUG] missing ctx sample", "uid=7"} {
		if !strings.Contains(line, want) {
			t.Fatalf("text log line %q does not contain %q", line, want)
		}
	}
}

func TestGlogconfTextFormatDesensitizesAttrs(t *testing.T) {
	dir := t.TempDir()
	fileName := "text_desensitize.log"
	logger.Newglog(dir, fileName, "text_desensitize.log.err", &logger.Glogconf{
		RotateMethod:     logger.ROTATE_FILE_DAILY,
		Stdout:           false,
		Colorful:         false,
		Loglevel:         logger.DEBUG,
		CtxValueKey:      "trace_id,request_id",
		Format:           logger.TEXT_FORMAT,
		DesensitizeField: "password,cardNo",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "trace-desensitize")
	logger.Info(ctx, "text desensitize sample",
		"password", "secret-password",
		"cardNo", "6222021234567890123",
		"uid", 1001,
	)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	for _, raw := range []string{"secret-password", "6222021234567890123"} {
		if strings.Contains(line, raw) {
			t.Fatalf("text log line should not contain raw sensitive value %q: %q", raw, line)
		}
	}
	for _, want := range []string{
		"trace-desensitize - [INFO] text desensitize sample",
		"password=******",
		"cardNo=******",
		"uid=1001",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("text log line %q does not contain %q", line, want)
		}
	}
}
