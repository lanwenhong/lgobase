package gpool

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

type internalNetworkRetryClient struct {
	transport thrift.TTransport
}

func newInternalNetworkRetryClient(transport thrift.TTransport, _ thrift.TProtocolFactory) *internalNetworkRetryClient {
	return &internalNetworkRetryClient{transport: transport}
}

func (c *internalNetworkRetryClient) call(ctx context.Context) error {
	if _, err := c.transport.Write([]byte{1}); err != nil {
		return err
	}
	if err := c.transport.Flush(ctx); err != nil {
		return err
	}

	var response [1]byte
	_, err := io.ReadFull(c.transport, response[:])
	return err
}

func TestGpoolThriftCall2RetriesAfterServerClosesTCPConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	var accepted atomic.Int32
	serverDone := make(chan error, 1)
	go func() {
		for attempt := 0; attempt < 2; attempt++ {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				serverDone <- acceptErr
				return
			}
			accepted.Add(1)
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

			var request [1]byte
			if _, readErr := io.ReadFull(conn, request[:]); readErr != nil {
				_ = conn.Close()
				serverDone <- readErr
				return
			}

			if attempt == 0 {
				// Simulate a server that drops the connection after receiving the
				// request but before returning a response.
				_ = conn.Close()
				continue
			}

			_, writeErr := conn.Write([]byte{1})
			_ = conn.Close()
			if writeErr != nil {
				serverDone <- writeErr
				return
			}
		}
		serverDone <- nil
	}()

	conf := &GPoolConfig[internalNetworkRetryClient]{
		MaxConns:      1,
		MaxIdleConns:  1,
		MaxWaiters:    1,
		RPCMaxRetries: 1,
		Cfunc:         CreateThriftBufferConn[internalNetworkRetryClient],
		Nc:            newInternalNetworkRetryClient,
	}
	pool := &Gpool[internalNetworkRetryClient]{}
	pool.GpoolInit2(context.Background(), host, port, 1_000, conf)
	t.Cleanup(func() {
		if closeErr := pool.Close(context.Background()); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	calls := 0
	err = pool.ThriftCall2(context.Background(), func(client interface{}) (string, error) {
		calls++
		return "network-retry", client.(*internalNetworkRetryClient).call(context.Background())
	})
	if err != nil {
		t.Fatalf("ThriftCall2() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("RPC calls = %d, want 2", calls)
	}
	if got := accepted.Load(); got != 2 {
		t.Fatalf("accepted TCP connections = %d, want 2", got)
	}

	select {
	case serverErr := <-serverDone:
		if serverErr != nil {
			t.Fatalf("server error = %v", serverErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal(fmt.Errorf("server did not finish"))
	}
}
