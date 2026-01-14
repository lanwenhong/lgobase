package logger

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
)

func TestFileRotate(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_NUM,
		//ColorFull:    true,
		Stdout:       false,
		Loglevel:     logger.DEBUG,
		MaxFileSize:  int64(2000 * 1024),
		MaxFileCount: int32(10),
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	ctx := context.Background()
	for i := 0; i < 2000; i++ {
		logger.Debugf(ctx, "aaaaaaaaaaaaaaaaaaaaaaaaaa")

	}
}
