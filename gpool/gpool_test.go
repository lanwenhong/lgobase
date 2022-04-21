package gpool

import (
    "fmt"
    "testing"
    "time"
    "context"
    "github.com/lanwenhong/lgobase/gpool/gen-go/echo"
    "github.com/lanwenhong/lgobase/gpool/gen-go/example"
    "github.com/lanwenhong/lgobase/gpool"
	"github.com/apache/thrift/lib/go/thrift"
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

func (e *ExampleServer) Add(ctx context.Context, a int32, b int32)(int32, error) {
    c := a + b
    return c, nil
}

func (e *ExampleServer) Echo(ctx context.Context, req string)(ret *example.Myret, r error) {
    ret = example.NewMyret()
    ret.Ret = req
    r = nil
    return
}


func TestBufferClient(t *testing.T) {
	go func(){
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
    time.Sleep(3 * time.Second)
    

    ctx, cancel := context.WithTimeout(context.Background(),10 * time.Second)
    defer cancel()

    gp := &gpool.Gpool[echo.EchoClient]{}
    gp.GpoolInit("127.0.0.1", 9898, 3, 10, 5, gpool.CreateThriftBufferConn[echo.EchoClient], echo.NewEchoClientFactory)

    gc, _ := gp.Get()
    defer gc.Close()
    req := &echo.EchoReq{Msg:"You are welcome."}
    client := gc.Gc.GetThrfitClient()
    ret, err := client.Echo(ctx, req)
    if err != nil {
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

    ctx, cancel := context.WithTimeout(context.Background(),10 * time.Second)
    defer cancel()

    gp := &gpool.Gpool[example.ExampleClient]{}
    gp.GpoolInit("127.0.0.1", 9899, 3, 10, 5, gpool.CreateThriftFramedConn[example.ExampleClient], example.NewExampleClientFactory)

    gc, _ := gp.Get()
    defer gc.Close()
    client := gc.Gc.GetThrfitClient()
    c , err := client.Add(ctx, 1, 2)
    if err != nil {
        t.Fatal(err.Error())
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
