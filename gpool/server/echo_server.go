//go:build gpool_echo_server

package main

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/gpool/gen-go/echo"
	"github.com/lanwenhong/lgobase/logger"
)

type EchoServer struct {
}

func (e *EchoServer) Ping(ctx context.Context) (string, error) {
	return "OK", nil
}

func (e *EchoServer) Echo(ctx context.Context, req *echo.EchoReq) (*echo.EchoRes, error) {
	uuid := uuid.New()
	v := uuid.String()
	//fmt.Println(v)
	cctx := context.WithValue(ctx, "trace_id", v)
	res := &echo.EchoRes{
		Msg: "success",
	}
	logger.Info(cctx, "echo request completed", "status", "success")

	//time.Sleep(4 * time.Second)
	return res, nil
}

func main() {

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		Goid:         true,
	}

	logger.Newglog("./", "test.log", "test.log.err", myconf)
	/*transport, err := thrift.NewTServerSocket(":9898")
	if err != nil {
		fmt.Println(err.Error())
		return
	}*/

	transport, err := thrift.NewTServerSocketTimeout(":9897", time.Duration(500000*time.Millisecond))
	if err != nil {
		logger.Error(context.Background(), "create echo Thrift server socket failed", "addr", ":9897", "err", err)
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
	logger.Info(context.Background(), "start echo Thrift server", "addr", ":9897", "transport", "buffered", "protocol", "binary")
	if err = server.Serve(); err != nil {
		logger.Error(context.Background(), "serve echo Thrift server failed", "addr", ":9897", "err", err)
	}
}
