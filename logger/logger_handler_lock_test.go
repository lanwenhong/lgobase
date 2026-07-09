package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTextHandlerUsesLogObjLock(t *testing.T) {
	glog := Newglog(t.TempDir(), "text_lock.log", "text_lock.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     DEBUG,
		Format:       TEXT_FORMAT,
	})

	handler := glog.GetLogger().Handler()
	hv := reflect.ValueOf(handler)
	if hv.Kind() != reflect.Pointer {
		t.Fatalf("handler kind = %s", hv.Kind())
	}
	mu := hv.Elem().FieldByName("Mu")
	if !mu.IsValid() {
		t.Fatalf("handler %T has no Mu field", handler)
	}
	if mu.Kind() != reflect.Pointer {
		t.Fatalf("handler Mu kind = %s, want pointer", mu.Kind())
	}
	if mu.Pointer() != reflect.ValueOf(glog.LogObj.mu).Pointer() {
		t.Fatalf("text handler should use LogObj.mu")
	}
}

func TestTextHandlerWritesCurrentLogObjFile(t *testing.T) {
	dir := t.TempDir()
	glog := Newglog(dir, "old.log", "old.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     DEBUG,
		Format:       TEXT_FORMAT,
	})

	handler := glog.GetLogger().Handler().(*CustomHandler)
	oldFile := glog.LogObj.logfile
	newPath := filepath.Join(dir, "new.log")
	newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer newFile.Close()

	glog.LogObj.mu.Lock()
	glog.LogObj.logfile = newFile
	glog.LogObj.mu.Unlock()

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "after rotate", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	newData, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newData), "after rotate") {
		t.Fatalf("current log file should contain record, got %q", string(newData))
	}

	oldData, err := os.ReadFile(oldFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oldData), "after rotate") {
		t.Fatalf("stale log file should not contain record, got %q", string(oldData))
	}
}

func TestJSONHandlerWritesCurrentLogObjFile(t *testing.T) {
	dir := t.TempDir()
	glog := Newglog(dir, "old_json.log", "old_json.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     false,
		Loglevel:     DEBUG,
	})

	handler := glog.GetLogger().Handler().(*MyModifyHandler)
	oldFile := glog.LogObj.logfile
	newPath := filepath.Join(dir, "new_json.log")
	newFile, err := os.OpenFile(newPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer newFile.Close()

	glog.LogObj.mu.Lock()
	glog.LogObj.logfile = newFile
	glog.LogObj.mu.Unlock()

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "json after rotate", 0)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	newData, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newData), "json after rotate") {
		t.Fatalf("current json log file should contain record, got %q", string(newData))
	}

	oldData, err := os.ReadFile(oldFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oldData), "json after rotate") {
		t.Fatalf("stale json log file should not contain record, got %q", string(oldData))
	}
}

func TestJSONHandlerStdoutUsesLogObjLock(t *testing.T) {
	dir := t.TempDir()
	glog := Newglog(dir, "stdout_json.log", "stdout_json.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     false,
		Loglevel:     DEBUG,
	})
	stdoutFile, err := os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()

	handler := newMyModifyHandler(stdoutFile, glog.LogObj, nil, nil)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "json stdout lock", 0)

	glog.LogObj.mu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- handler.Handle(context.Background(), record)
	}()

	select {
	case err := <-done:
		glog.LogObj.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("json stdout write completed while LogObj.mu was locked")
	case <-time.After(50 * time.Millisecond):
	}

	glog.LogObj.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "json stdout lock") {
		t.Fatalf("stdout should contain json record, got %q", string(data))
	}
}

func TestTextHandlerStdoutUsesLogObjLock(t *testing.T) {
	dir := t.TempDir()
	glog := Newglog(dir, "stdout_text.log", "stdout_text.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     false,
		Loglevel:     DEBUG,
		Format:       TEXT_FORMAT,
	})
	stdoutFile, err := os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()

	handler := newCustomLogger(stdoutFile, glog.LogObj, nil, nil, glog.LogObj.mu, 0).Handler()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "text stdout lock", 0)

	glog.LogObj.mu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- handler.Handle(context.Background(), record)
	}()

	select {
	case err := <-done:
		glog.LogObj.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("text stdout write completed while LogObj.mu was locked")
	case <-time.After(50 * time.Millisecond):
	}

	glog.LogObj.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "text stdout lock") {
		t.Fatalf("stdout should contain text record, got %q", string(data))
	}
}

func TestJSONHandlerColorfulStdoutUsesLogObjLock(t *testing.T) {
	dir := t.TempDir()
	glog := Newglog(dir, "stdout_color_json.log", "stdout_color_json.log.err", &Glogconf{
		RotateMethod: ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     DEBUG,
	})
	stdoutFile, err := os.OpenFile(filepath.Join(dir, "stdout_color.log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()

	handler := newMyModifyHandler(stdoutFile, glog.LogObj, nil, nil)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "json colorful stdout lock", 0)

	glog.LogObj.mu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- handler.Handle(context.Background(), record)
	}()

	select {
	case err := <-done:
		glog.LogObj.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("json colorful stdout write completed while LogObj.mu was locked")
	case <-time.After(50 * time.Millisecond):
	}

	glog.LogObj.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "json colorful stdout lock") {
		t.Fatalf("stdout should contain colorful json record, got %q", string(data))
	}
}
