package dbenc_test

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/logger"
)

func TestLoadFile(t *testing.T) {
	file := "./db.conf"
	ctx := context.Background()
	dconf := dbenc.DbConfNew(ctx, file)
	xconf := dconf.DbConfReadGroup("qmm")
	logger.Debug(ctx, "database config test input", "config", xconf)
	yconf := dconf.DbConfReadGroup("qf_qudao_statistics_r")
	logger.Debug(ctx, "database config test output", "config", yconf)
}

func TestLoadFile2(t *testing.T) {
	file := "/Users/dc/Downloads/others/mydb.ini"
	ctx := context.Background()
	dconf := dbenc.DbConfNew(ctx, file)
	xconf := dconf.DbConfReadGroup("qf_risk_3")
	logger.Debug(ctx, "database config test input", "config", xconf)
	//yconf := dconf.DbConfReadGroup("qf_qudao_statistics_r")
	//logger.Debugf(ctx, "yconf: %v", yconf)
}
