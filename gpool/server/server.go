//go:build gpool_example_server

package main

import (
	"context"

	//"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
	"github.com/lanwenhong/lgobase/gpool/gen-go/example"
	"github.com/lanwenhong/lgobase/logger"

	//"github.com/lanwenhong/lgobase/gpool"
	"github.com/apache/thrift/lib/go/thrift"
)

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

func main() {
	ctx := context.Background()
	transport, err := thrift.NewTServerSocket(":9899")
	if err != nil {
		logger.Error(ctx, "create example Thrift server socket failed", "addr", ":9899", "err", err)
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
	logger.Debug(ctx, "start thrift server", "port", 9899, "transport", "framed", "protocol", "binary")
	if err = server.Serve(); err != nil {
		logger.Error(ctx, "serve example Thrift server failed", "addr", ":9899", "err", err)
	}
}
