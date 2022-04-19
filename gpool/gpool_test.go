package gpool

import (
    "fmt"
    "testing"
    "time"
    "context"
    "github.com/lanwenhong/lgobase/gpool/gen-go/echo"
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
    _, err := client.Echo(ctx, req)
    if err != nil {
        t.Fatal(err.Error())
    }

}
