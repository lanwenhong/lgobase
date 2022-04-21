package main

import (
    "fmt"
    "context"
    //"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
    "github.com/lanwenhong/lgobase/gpool/gen-go/example"
    //"github.com/lanwenhong/lgobase/gpool"
    "github.com/apache/thrift/lib/go/thrift"
)

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

func main() {
    transport, err := thrift.NewTServerSocket(":9899")
	if err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
	}  
}
