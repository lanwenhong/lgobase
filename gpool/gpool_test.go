package gpool

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
	"github.com/lanwenhong/lgobase/gpool/gen-go/example"
	"github.com/lanwenhong/lgobase/logger"
)

func NewRequestID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

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

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	/*gc, err := gp.Get(ctx)
	if err != nil {
		t.Fatal(err.Error())
	}*/
	req := &echo.EchoReq{Msg: "You are welcome."}
	//ret, err := gpool.ThriftCall[echo.EchoClient](ctx, gc, "Echo", req)
	ret, err := gp.ThriftCall(ctx, "Echo", req)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log("rpc get: ", ret.(*echo.EchoRes).Msg)
}

func Test1BufferClient(t *testing.T) {
	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		ColorFull:    true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	gp := &gpool.Gpool[echo.EchoClient]{}
	gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

	/*gc, err := gp.Get(ctx)
	if err != nil {
		t.Fatal(err.Error())
	}*/
	req := &echo.EchoReq{Msg: "You are welcome."}
	//ret, err := gpool.ThriftCall[echo.EchoClient](ctx, gc, "Echo", req)
	ret, err := gp.ThriftCall(ctx, "Echo", req)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log("rpc get: ", ret.(*echo.EchoRes).Msg)

	time.Sleep(5 * time.Second)

	/*gc, err = gp.Get(ctx)
	if err != nil {
		t.Fatal(err.Error())
	}*/
	req = &echo.EchoReq{Msg: "You are welcome."}
	ret, err = gp.ThriftCall(ctx, "Echo", req)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Log("rpc2 get: ", ret.(*echo.EchoRes).Msg)

}

func TestFramedClient(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
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

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()

	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 10, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)

	var rpc_err error = nil
	gc, _ := gp.Get(ctx)
	defer gc.Close(ctx, rpc_err)

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
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())

	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 10, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)
	for i := 0; i < 15; i++ {
		gc, err := gp.Get(ctx)
		client := gc.Gc.GetThrfitClient()
		ret, err := client.Echo(ctx, "ganni")
		gc.Close(ctx, err)
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
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	ctx := context.WithValue(context.Background(), "trace_id", NewRequestID())
	gp := &gpool.Gpool[example.ExampleClient]{}
	gp.GpoolInit("127.0.0.1", 9899, 3, 201, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)

	cs := make(chan string)
	for i := 0; i < 200; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				gc, err := gp.Get(ctx)
				if err != nil {
					cs <- "error"
					break
				}
				t.Log("<<<<<DEBUG>>>>>")
				client := gc.Gc.GetThrfitClient()
				ret, err := client.Echo(ctx, "ganni")
				gc.Close(ctx, err)
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
