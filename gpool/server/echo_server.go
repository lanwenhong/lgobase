package main

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
)

type EchoServer struct {
}

func (e *EchoServer) Echo(ctx context.Context, req *echo.EchoReq) (*echo.EchoRes, error) {
	//fmt.Printf("message from client: %v\n", req.GetMsg())

	res := &echo.EchoRes{
		Msg: "success",
	}

	return res, nil
}

func main() {

	transport, err := thrift.NewTServerSocket(":9898")
	if err != nil {
		fmt.Println(err.Error())
		return
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
	fmt.Println("server start")
	if err = server.Serve(); err != nil {
		fmt.Println(err.Error())
	}
}
