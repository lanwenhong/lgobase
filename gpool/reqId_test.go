package gpool

import (
	"context"
	"sync"
	"testing"

	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/gpool/gen-go/server"
	"github.com/lanwenhong/lgobase/logger"
)

func TestReqId(t *testing.T) {
	ctx := context.Background()
	g_conf := &gpool.GPoolConfig[server.ServerTestClient]{
		Addrs: "127.0.0.1:9090/30000",
		Cfunc: gpool.CreateThriftFramedConnWithReqId[server.ServerTestClient],
		Nc:    server.NewServerTestClientFactory,
	}
	addPool := gpool.NewRpcPoolSelector[server.ServerTestClient](ctx, g_conf)

	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				process := func(ctx context.Context, client interface{}) (string, error) {
					c := client.(*server.ServerTestClient)
					r, err := c.Add(ctx, 1, 1)
					if err != nil {
						logger.Warnf(ctx, "err: %s", err.Error())
					}
					logger.Debugf(ctx, "r: %d", r)
					return "add", err
				}

				addPool.ThriftCallWithReqId(ctx, "", process)

			}

		}()
	}
	wg.Wait()
}
