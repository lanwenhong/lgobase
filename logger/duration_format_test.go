package logger

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatDurationUS(t *testing.T) {
	if got := FormatDurationUS(1234567 * time.Nanosecond); got != "1234us" {
		t.Fatalf("FormatDurationUS() = %q, want %q", got, "1234us")
	}
}

func TestJSONLoggerFormatsDurationWithMicrosecondUnit(t *testing.T) {
	directory := t.TempDir()
	Newglog(directory, "duration.json.log", "duration.json.error.log", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     INFO,
	})
	Info(context.Background(), "duration sample", "cost", 1234567*time.Nanosecond)

	data, err := os.ReadFile(directory + "/duration.json.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"cost":"1234us"`) {
		t.Fatalf("JSON duration has no microsecond unit: %s", data)
	}
}

func TestTextLoggerFormatsDurationWithMicrosecondUnit(t *testing.T) {
	directory := t.TempDir()
	Newglog(directory, "duration.text.log", "duration.text.error.log", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     INFO,
		Format:       TEXT_FORMAT,
	})
	Info(context.Background(), "duration sample", "cost", 1234567*time.Nanosecond)

	data, err := os.ReadFile(directory + "/duration.text.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cost=1234us") {
		t.Fatalf("text duration has no microsecond unit: %s", data)
	}
}
