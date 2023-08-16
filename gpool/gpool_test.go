package gpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
	"github.com/lanwenhong/lgobase/gpool/gen-go/example"
)

type EchoServer struct {
}

func (e *EchoServer) Echo(ctx context.Context, req *echo.EchoReq) (*echo.EchoRes, error) {
	fmt.Printf("message from client: %v\n", req.GetMsg())

	res := &echo.EchoRes{
		Msg: "success",
	}

	return res, nil
}

type ExampleServer struct {
}

func (e *ExampleServer) Add(ctx context.Context, a int32, b int32) (int32, error) {
	c := a + b
	return c, nil
}

func (e *ExampleServer) Echo(ctx context.Context, req string) (ret *example.Myret, r error) {
	ret = example.NewMyret()
	ret.Ret = req
	r = nil
	return
}

func TestBufferClient(t *testing.T) {
	/*go func() {
		transport, err := thrift.NewTServerSocket(":9898")
		if err != nil {
			t.Fatal(err.Error())
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
			t.Fatal(err.Error())
		}

	}()
	time.Sleep(3 * time.Second)*/

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	//var rpcerr error = nil
	t.Log(gp)
	gc, err := gp.Get()
	if err != nil {
		t.Fatal(err.Error())
	}
	var rpc_err error = nil
	defer gc.Close(rpc_err)

	req := &echo.EchoReq{Msg: "You are welcome."}
	client := gc.Gc.GetThrfitClient()
	ret, rpc_err := client.Echo(ctx, req)
	//rpcerr = err
	if rpc_err != nil {
		t.Fatal(err.Error())
	}
	t.Log("rpc get: ", ret.Msg)

}

func TestFramedClient(t *testing.T) {
	go func() {
		transport, err := thrift.NewTServerSocket(":9899")
		if err != nil {
			t.Fatal(err.Error())
		}
		handler := &ExampleServer{}
		processor := example.NewExampleProcessor(handler)
		transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
		protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
		server := thrift.NewTSimpleServer4(
			processor,
			transport,
			transportFactory,
			protocolFactory,
		)
		if err = server.Serve(); err != nil {
			t.Fatal(err.Error())
		}

	}()
	time.Sleep(3 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 10, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)

	var rpc_err error = nil
	gc, _ := gp.Get()
	defer gc.Close(rpc_err)

	client := gc.Gc.GetThrfitClient()
	c, rpc_err := client.Add(ctx, 1, 2)
	if rpc_err != nil {
		t.Fatal(rpc_err.Error())
	}
	t.Log("rpc get c: ", c)

	if c != 3 {
		t.Fatal("value error")
	}
	ret, err := client.Echo(ctx, "ganni")
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log("rcp get: ", ret.Ret)

}

func TestGpoolReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 10, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)
	for i := 0; i < 15; i++ {
		gc, err := gp.Get()
		client := gc.Gc.GetThrfitClient()
		ret, err := client.Echo(ctx, "ganni")
		gc.Close(err)
		if err != nil {
			t.Log(err.Error())
			time.Sleep(1 * time.Second)
			continue
		}
		if err == nil {
			t.Log(ret.Ret)
		}
		time.Sleep(1 * time.Second)
	}
}

func TestGpoolList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 201, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)

	cs := make(chan string)
	for i := 0; i < 200; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				gc, err := gp.Get()
				if err != nil {
					cs <- "error"
					break
				}
				t.Log("<<<<<DEBUG>>>>>")
				client := gc.Gc.GetThrfitClient()
				ret, err := client.Echo(ctx, "ganni")
				gc.Close(err)
				if err != nil {
					t.Log(err.Error())
					cs <- "ferror"
				}
				t.Log("+++++++++++++", ret.Ret)
			}
			cs <- "ok"
		}()

	}

	i := 0
	for s := range cs {
		t.Log(s)
		i++
		if i >= 200 {
			break
		}
		t.Log("<<<<<<>>>>>", i)
	}

	if gp.GetFreeLen() != 5 {
		t.Fatal("free len error")
	}
}
