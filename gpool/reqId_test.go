package gpool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/server"
	"github.com/lanwenhong/lgobase/logger"
)

func TestReqId(t *testing.T) {
	ctx := context.Background()

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	g_conf := &gpool.GPoolConfig[server.ServerTestClient]{
		Addrs: "127.0.0.1:9090/30000",
		Cfunc: gpool.CreateThriftFramedConnThriftExt[server.ServerTestClient],
		//Cfunc: gpool.CreateThriftFramedConn[server.ServerTestClient],
		//Cfunc: gpool.CreateThriftBufferConnThriftExt[server.ServerTestClient],
		//Cfunc: gpool.CreateThriftBufferConn[server.ServerTestClient],
		Nc: server.NewServerTestClientFactory,

		MaxConns:     1000,
		MaxIdleConns: 500,
	}
	addPool := gpool.NewRpcPoolSelector[server.ServerTestClient](ctx, g_conf)

	wg := sync.WaitGroup{}
	for i := 0; i < 2; i++ {
		ctx = context.WithValue(ctx, "trace_id", uuid.New().String())
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				process := func(ctx context.Context, client interface{}) (string, error) {
					//process := func(client interface{}) (string, error) {
					c := client.(*server.ServerTestClient)
					r, err := c.Add(ctx, 1, 1)
					if err != nil {
						logger.Warnf(ctx, "err: %s", err.Error())
					}
					logger.Debugf(ctx, "r: %d", r)
					return "add", err
				}

				ctx = gpool.NewExtContext(ctx)
				addPool.ThriftExtCall(ctx, process)
				//addPool.ThriftCall(ctx, process)

			}

		}()
	}
	wg.Wait()
}

func TestPostStru(t *testing.T) {
	ctx := context.Background()
	g_conf := &gpool.GPoolConfig[server.ServerTestClient]{
		Addrs: "127.0.0.1:9090/30000",
		Cfunc: gpool.CreateThriftFramedConnThriftExt[server.ServerTestClient],
		Nc:    server.NewServerTestClientFactory,
	}
	addPool := gpool.NewRpcPoolSelector[server.ServerTestClient](ctx, g_conf)

	wg := sync.WaitGroup{}
	req := &server.GetUserRequest{
		UserID: 1111,
		Name:   "jjjjjj",
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				process := func(ctx context.Context, client interface{}) (string, error) {
					c := client.(*server.ServerTestClient)
					r, err := c.PostUser(ctx, req)
					if err != nil {
						logger.Warnf(ctx, "err: %s", err.Error())
					}
					logger.Debugf(ctx, "r: %d", r)
					return "add", err
				}

				addPool.ThriftExtCall(ctx, process)

			}

		}()
	}
	wg.Wait()

}

func TestGetConnTimeOut(t *testing.T) {
	ctx := context.Background()
	g_conf := &gpool.GPoolConfig[server.ServerTestClient]{
		Addrs:        "127.0.0.1:9090/3000",
		MaxConns:     1,
		MaxIdleConns: 1,
		Cfunc:        gpool.CreateThriftFramedConnThriftExt[server.ServerTestClient],
		Nc:           server.NewServerTestClientFactory,
	}
	addPool := gpool.NewRpcPoolSelector[server.ServerTestClient](ctx, g_conf)

	wg := sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			ctx = context.WithValue(ctx, "trace_id", uuid.New().String())
			defer wg.Done()
			for i := 0; i < 1; i++ {
				process := func(ctx context.Context, client interface{}) (string, error) {
					time.Sleep(5 * time.Second)
					c := client.(*server.ServerTestClient)
					r, err := c.Add(ctx, 1, 1)
					if err != nil {
						logger.Warnf(ctx, "err: %s", err.Error())
					}
					logger.Debugf(ctx, "r: %d", r)
					return "add", err
				}
				nCtx := gpool.NewExtContext(ctx)
				addPool.ThriftExtCall(nCtx, process)

			}

		}()
	}
	time.Sleep(1 * time.Second)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			ctx = context.WithValue(ctx, "trace_id", uuid.New().String())
			defer wg.Done()
			for i := 0; i < 1; i++ {
				process := func(ctx context.Context, client interface{}) (string, error) {
					c := client.(*server.ServerTestClient)
					r, err := c.Add(ctx, 1, 1)
					if err != nil {
						logger.Warnf(ctx, "err: %s", err.Error())
					}
					logger.Debugf(ctx, "r: %d", r)
					return "add", err
				}
				nCtx := gpool.NewExtContext(ctx)
				addPool.ThriftExtCall(nCtx, process)
			}

		}()
	}

	wg.Wait()
}
