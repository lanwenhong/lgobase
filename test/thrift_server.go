package main

import (

    "fmt"
    "context"
    "github.com/apache/thrift/lib/go/thrift"
    "github.com/lanwenhong/lgobase/test/gen-go/echo"
    //"github.com/lanwenhong/lgobase/logger"
    //"github.com/lanwenhong/lgobase/test/mytest" 

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

func main() {
    //ctx,cancel := context.WithTimeout(context.Background(),10 * time.Second)
    //defer cancel()
    //mytest.Mytest()
    transport, err := thrift.NewTServerSocket(":9898")
    if err != nil {
        panic(err)
    }

    handler := &EchoServer{}
    processor := echo.NewEchoProcessor(handler)

    transportFactory := thrift.NewTBufferedTransportFactory(8192)
    protocolFactory := thrift.NewTCompactProtocolFactory()
    server := thrift.NewTSimpleServer4(
        processor,
        transport,
        transportFactory,
        protocolFactory,
    )

    if err := server.Serve(); err != nil {
        panic(err)
    }
    /*logger.SetConsole(true)
    logger.SetRollingDaily("./",  "my.log", "my.log.err")
    loglevel, _ := logger.LoggerLevelIndex("DEBUG")
    logger.SetLevel(loglevel)
    logger.Debugf("xxxx")*/
}
