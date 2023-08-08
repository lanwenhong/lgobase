package gpool

import (
	"context"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
)

func BenchmarkBufferClient(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	for n := 0; n < b.N; n++ {
		gc, err := gp.Get()
		if err != nil {
			b.Fatal(err.Error())
		}
		//defer gc.Close()
		req := &echo.EchoReq{Msg: "You are welcome."}
		client := gc.Gc.GetThrfitClient()
		ret, err := client.Echo(ctx, req)
		//rpcerr = err
		if err != nil {
			b.Fatal(err.Error())
		}
		b.Log("rpc get: ", ret.Msg)
		gc.Close()
	}
}
