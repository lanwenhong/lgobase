package logger_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	dlog "gorm.io/gorm/logger"
)

func TestDBLoggerInfoFormatsPrintfArgs(t *testing.T) {
	dir := t.TempDir()
	fileName := "db_logger_printf.log"
	glog := logger.Newglog(dir, fileName, "db_logger_printf.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})
	dbLogger := logger.New(glog, dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})

	dbLogger.Info(context.Background(), "db logger id=%d name=%s", 7, "alice")

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	if strings.Contains(line, "!BADKEY") {
		t.Fatalf("db logger printf args should not be written as slog attrs: %q", line)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("db logger line should be json, got %q: %v", line, err)
	}
	if fields["msg"] != "db logger id=7 name=alice" {
		t.Fatalf("msg = %v", fields["msg"])
	}
	if fields["source_db"] == "" {
		t.Fatalf("source_db should be set: %q", line)
	}
}

func TestDBLoggerInfoKeepsStructuredAttrs(t *testing.T) {
	dir := t.TempDir()
	fileName := "db_logger_attrs.log"
	glog := logger.Newglog(dir, fileName, "db_logger_attrs.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id",
	})
	dbLogger := logger.New(glog, dlog.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  dlog.Info,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})

	dbLogger.Info(context.Background(), "db logger attrs", "db1", 1, "db2", 2)

	line := readLastLogLine(t, filepath.Join(dir, fileName))
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("db logger line should be json, got %q: %v", line, err)
	}
	if fields["msg"] != "db logger attrs" {
		t.Fatalf("msg = %v", fields["msg"])
	}
	if fields["db1"] != float64(1) || fields["db2"] != float64(2) {
		t.Fatalf("structured attrs not written correctly: %q", line)
	}
}
