package gpool

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
	"github.com/lanwenhong/lgobase/gpool/gen-go/server"
	"github.com/lanwenhong/lgobase/logger"
)

func NewRequestID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

type EchoServer struct {
}

func (e *EchoServer) Echo(ctx context.Context, req *echo.EchoReq) (*echo.EchoRes, error) {
	//fmt.Printf("message from client: %v\n", req.GetMsg())

	res := &echo.EchoRes{
		Msg: "success",
	}

	return res, nil
}

/*func BenchmarkBufferClient(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	for n := 0; n < b.N; n++ {
		gc, err := gp.Get(ctx)
		if err != nil {
			b.Fatal(err.Error())
		}
		//defer gc.Close()
		req := &echo.EchoReq{Msg: "You are welcome."}
		client := gc.Gc.GetThrfitClient()
		ret, err := client.Echo(ctx, req)
		gc.Close(ctx, err)
		//rpcerr = err
		if err != nil {
			b.Fatal(err.Error())
		}
		b.Log("rpc get: ", ret.Msg)
	}
}*/

/*func BenchmarkBufferClient2(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 1000, 500, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gc, err := gp.Get(ctx)
			if err != nil {
				b.Fatal(err.Error())
			}
			//defer gc.Close()
			req := &echo.EchoReq{Msg: "You are welcome."}
			client := gc.Gc.GetThrfitClient()
			ret, err := client.Echo(ctx, req)
			gc.Close(ctx, err)
			//rpcerr = err
			if err != nil {
				b.Fatal(err.Error())
			}
			b.Log("rpc get: ", ret)
		}
	})
}*/

/*func Benchmark3BufferClient(b *testing.B) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 100, 50, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	for n := 0; n < b.N; n++ {
		req := &echo.EchoReq{Msg: "You are welcome."}
		ret, err := gp.ThriftCall(ctx, "Echo", req)
		if err != nil {
			b.Fatal(err.Error())
		}
		b.Log("rpc get: ", ret.(*echo.EchoRes).Msg)
	}
}*/

func BenchmarkThriftExt(b *testing.B) {
	ctx := context.Background()

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       false,
		Colorful:     true,
		Loglevel:     logger.INFO,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	g_conf := &gpool.GPoolConfig[server.ServerTestClient]{
		Addrs: "127.0.0.1:9090/30000",
		//Cfunc: gpool.CreateThriftFramedConnThriftExt[server.ServerTestClient],
		Cfunc: gpool.CreateThriftFramedConn[server.ServerTestClient],
		//Cfunc: gpool.CreateThriftBufferConnThriftExt[server.ServerTestClient],
		//Cfunc: gpool.CreateThriftBufferConn[server.ServerTestClient],
		MaxConns:     1000,
		MaxIdleConns: 500,
		Nc:           server.NewServerTestClientFactory,
	}
	addPool := gpool.NewRpcPoolSelector[server.ServerTestClient](ctx, g_conf)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ctx = context.WithValue(ctx, "trace_id", uuid.New().String())
		//process := func(ctx context.Context, client interface{}) (string, error) {
		process := func(client interface{}) (string, error) {
			c := client.(*server.ServerTestClient)
			r, err := c.Add(ctx, 1, 1)
			if err != nil {
				logger.Warnf(ctx, "err: %s", err.Error())
			}
			logger.Debugf(ctx, "r: %d", r)
			return "add", err
		}

		//ctx = gpool.NewExtContext(ctx)
		//addPool.ThriftExtCall(ctx, process)
		addPool.ThriftCall(ctx, process)
	}

}
