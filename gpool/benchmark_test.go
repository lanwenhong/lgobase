package gpool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
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

func BenchmarkBufferClient(b *testing.B) {
	/*go func() {
		transport, err := thrift.NewTServerSocket(":9898")
		if err != nil {
			b.Fatal(err.Error())
		}
		handler := &EchoServer{}
		processor := echo.NewEchoProcessor(handler)
		transportFactory := thrift.NewTBufferedTransportFactory(8192)
		protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
		server := thrift.NewTSimpleServer4(
			processor,
			transport,
			transportFactory,
			protocolFactory,
		)
		if err = server.Serve(); err != nil {
			b.Fatal(err.Error())
		}

	}()
	time.Sleep(3 * time.Second)
	b.ResetTimer()*/

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
}

func BenchmarkBufferClient2(b *testing.B) {
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
}

func Benchmark3BufferClient(b *testing.B) {
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
}
