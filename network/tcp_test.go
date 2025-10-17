package network

import (
	"context"
	"fmt"
	"testing"
	"time"

	"encoding/hex"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/network"
	"github.com/lanwenhong/lgobase/util"
)

func TestEcho(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
	a := "sssssssssssssss"
	alen := len(a)
	logger.Debugf(ctx, "alen: %d", alen)
	slen := fmt.Sprintf("%04X", alen)

	logger.Debugf(ctx, "slen: %s", slen)

	header, _ := hex.DecodeString(slen)
	b := []byte{}
	b = append(b, header...)
	b = append(b, a...)

	cTimeout := 3 * time.Second
	rTimeout := 3 * time.Second
	wTimeout := 3 * time.Second

	conn := network.NewTcpConn("127.0.0.1:8080", cTimeout, rTimeout, wTimeout)
	err := conn.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetOptLinger(ctx, 0)

	conn.Writen(ctx, b)

}
