package logger_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
	"github.com/lanwenhong/lgobase/logger"
)

func TestDBLoggerTextFormatWithMysql(t *testing.T) {
	fmt.Println()
	fmt.Println("========== DB logger text stdout colorful sample ==========")

	ctx := context.WithValue(context.Background(), "trace_id", "trace-db-text")
	ctx = context.WithValue(ctx, "request_id", "request-db-text")

	logger.Newglog(t.TempDir(), "db_text.log", "db_text.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		CtxValueKey:  "trace_id,request_id",
		Format:       logger.TEXT_FORMAT,
	})

	dbConf := dbenc.DbConfNew(ctx, "../dbpool/db.ini")
	dbs := dbpool.DbpoolNew(dbConf)
	if err := dbs.Add(ctx, "db_text_sample", "qfconf://test1?maxopen=1000&maxidle=30", dbpool.USE_GORM); err != nil {
		t.Fatal(err)
	}

	tdb := dbs.OrmPools["db_text_sample"]
	if err := tdb.WithContext(ctx).Exec(`
CREATE TABLE IF NOT EXISTS users (
	id BIGINT PRIMARY KEY,
	username VARCHAR(128) NOT NULL,
	ctime BIGINT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tdb.WithContext(ctx).Exec(`
INSERT INTO users (id, username, ctime)
VALUES (?, ?, UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE username = VALUES(username), ctime = VALUES(ctime)`,
		1, "codex_mysql_text_sample").Error; err != nil {
		t.Fatal(err)
	}

	var ret []map[string]interface{}
	if err := tdb.WithContext(ctx).Raw(
		"select id, username, FROM_UNIXTIME(ctime, '%Y-%m-%d %H:%i:%s') as ctime from users where id=?",
		1,
	).Scan(&ret).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("mysql text sample result: %+v", ret)
}

func TestDBLoggerInfoLevelHidesInternalDebugWithMysql(t *testing.T) {
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

	ctx := context.WithValue(context.Background(), "trace_id", "trace-db-info")
	ctx = context.WithValue(ctx, "request_id", "request-db-info")

	logger.Newglog(dir, "db_info.log", "db_info.log.err", &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     false,
		Loglevel:     logger.INFO,
		CtxValueKey:  "trace_id,request_id",
		Format:       logger.TEXT_FORMAT,
	})

	dbConf := dbenc.DbConfNew(ctx, "../dbpool/db.ini")
	dbs := dbpool.DbpoolNew(dbConf)
	if err := dbs.Add(ctx, "db_info_sample", "qfconf://test1?maxopen=1000&maxidle=30", dbpool.USE_GORM); err != nil {
		t.Fatal(err)
	}

	var ret []map[string]interface{}
	if err := dbs.OrmPools["db_info_sample"].WithContext(ctx).Raw(
		"select id, username, FROM_UNIXTIME(ctime, '%Y-%m-%d %H:%i:%s') as ctime from users where id=?",
		1,
	).Scan(&ret).Error; err != nil {
		t.Fatal(err)
	}
	if err := stdoutFile.Sync(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	logs := string(data)
	if strings.Contains(logs, "[DEBUG]") || strings.Contains(logs, "db url:") || strings.Contains(logs, "mk:") {
		t.Fatalf("INFO level should hide lgobase DEBUG logs, got:\n%s", logs)
	}
	if !strings.Contains(logs, "[INFO] SQLLOG") {
		t.Fatalf("INFO level should keep DB SQL info log, got:\n%s", logs)
	}
}
