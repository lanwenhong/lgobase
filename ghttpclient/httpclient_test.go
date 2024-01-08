package ghttpclient

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/ghttpclient"
	"github.com/lanwenhong/lgobase/logger"
)

func TestMidWare(t *testing.T) {

	lconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Loglevel:     logger.DEBUG,
		ColorFull:    true,
	}
	logger.Newglog("./", "test.log", "test.log.err", lconf)

	req := map[string]string{
		"fff": "111",
		"ttt": "222",
	}

	header := map[string]string{}

	//c := ghttpclient.NewHttpClient(3000)

	ctx := context.Background()
	logger.Infof(ctx, "xxx")
	c := ghttpclient.QfHttpClientNew(ghttpclient.INTER_PROTO_PUSHAPI, "127.0.0.1:8000", false)
	v, err := c.Get(ctx, "/", 3000, req, header)
	if err != nil {
		t.Fatal(ctx, "err: %s", err.Error())
		return
	}
	t.Log(string(v))
}
