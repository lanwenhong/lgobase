//go:build gpool_add_server

package main

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/server"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

// 服务实现
type SvrHandler struct {
	//Rp *proto.RequestIDProtocol
}

func (sh *SvrHandler) Add(ctx context.Context, a int32, b int32) (int32, error) {
	//rid := gpool.GetRequestID(ctx)
	//ctx = context.WithValue(ctx, "trace_id", util.NewRequestID())
	c := a + b
	starttime := time.Now()
	logger.Debug(ctx, "add server request", "func", "Add", "result", c, "cost", time.Since(starttime))
	//time.Sleep(2 * time.Second)
	return c, nil
}

func (sh *SvrHandler) Add1(ctx context.Context, magic int16, ver int16, ext map[string]string, a int32, b int32) (int32, error) {
	rid := gpool.GetRequestID(ctx)
	logger.Debug(ctx, "add server request", "func", "Add1", "request_id", rid, "magic", magic, "version", ver, "extension_count", len(ext))
	c := a + b
	logger.Debug(ctx, "add server response", "func", "Add1", "result", c)
	return c, nil

}

func (sh *SvrHandler) PostUser(ctx context.Context, req *server.GetUserRequest) (int32, error) {
	rid := gpool.GetRequestID(ctx)
	logger.Debug(ctx, "post user request", "request_id", rid, "request", req)
	return 0, nil
}

func main() {
	ctx := context.WithValue(context.Background(), "trace_id", util.NewProcessID())
	// 1. 创建服务端监听 socket
	serverSocket, err := thrift.NewTServerSocket("localhost:9090")
	if err != nil {
		panic(err)
	}

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//CtxValueKey:  "trace_id,request_id,client_service,depth",
	}
	logger.Newglog("./", "add.log", "add.log.err", myconf)

	// 2. 创建 Framed 传输工厂（核心：服务端需与客户端一致使用帧模式）
	//transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	transportFactory := thrift.NewTBufferedTransportFactory(8192)

	// 3. 创建底层协议工厂（如二进制协议）
	rawProtoFactory := thrift.NewTBinaryProtocolFactoryDefault()

	// 5. 创建处理器并启动服务
	sh := &SvrHandler{}
	processor := server.NewServerTestProcessor(sh)
	processor1 := gpool.NewExtProcessor(processor, rawProtoFactory)

	server := thrift.NewTSimpleServer4(
		processor1,
		//processor,
		serverSocket,
		transportFactory,
		rawProtoFactory,
	)

	logger.Debug(ctx, "start thrift server", "port", 9090, "transport", "buffered", "protocol", "binary", "extension", true)
	if err := server.Serve(); err != nil {
		panic(err)
	}
}
