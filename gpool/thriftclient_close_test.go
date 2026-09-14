package gpool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
)

type closeResultTransport struct {
	*thrift.TMemoryBuffer
	closeErr error
	open     bool
}

func (c *closeResultTransport) Close() error {
	c.open = false
	return c.closeErr
}

func (c *closeResultTransport) IsOpen() bool { return c.open }

func newCloseTestTConn(protocol int, transport thrift.TTransport) *TConn[internalTestClient] {
	c := &TConn[internalTestClient]{Protocol: protocol}
	if protocol == TH_PRO_FRAMED {
		c.Tft = thrift.NewTFramedTransport(transport)
	} else {
		c.Tbt = thrift.NewTBufferedTransport(transport, 128)
	}
	return c
}

func TestTConnCloseErrorSemantics(t *testing.T) {
	closeFailure := errors.New("physical close failed")
	lookalike := errors.New(net.ErrClosed.Error())
	for _, protocol := range []int{TH_PRO_FRAMED, TH_PRO_BUFFER} {
		for _, tt := range []struct {
			name string
			err  error
			want error
		}{
			{name: "success"},
			{name: "already closed", err: net.ErrClosed},
			{name: "wrapped already closed", err: fmt.Errorf("transport close: %w", net.ErrClosed)},
			{name: "real failure", err: closeFailure, want: closeFailure},
			{name: "same text is not the sentinel", err: lookalike, want: lookalike},
		} {
			t.Run(fmt.Sprintf("protocol_%d/%s", protocol, tt.name), func(t *testing.T) {
				transport := &closeResultTransport{TMemoryBuffer: thrift.NewTMemoryBuffer(), open: true, closeErr: tt.err}
				conn := newCloseTestTConn(protocol, transport)
				if err := conn.Close(); err != tt.want {
					t.Fatalf("Close() = %v, want %v", err, tt.want)
				}
				if conn.IsOpen() {
					t.Fatal("connection still reported open after closing")
				}
			})
		}
	}
}

func TestTConnRepeatedClose(t *testing.T) {
	for _, protocol := range []int{TH_PRO_FRAMED, TH_PRO_BUFFER} {
		t.Run(fmt.Sprintf("protocol_%d", protocol), func(t *testing.T) {
			client, peer := net.Pipe()
			t.Cleanup(func() { _ = client.Close(); _ = peer.Close() })
			socket := thrift.NewTSocketFromConnConf(client, &thrift.TConfiguration{})
			conn := newCloseTestTConn(protocol, socket)
			for i := 0; i < 3; i++ {
				if err := conn.Close(); err != nil {
					t.Fatalf("Close() attempt %d: %v", i+1, err)
				}
				if conn.IsOpen() {
					t.Fatal("closed socket reported open")
				}
			}
		})
	}
}

func TestGpoolShutdownPreservesRealCloseErrors(t *testing.T) {
	closeFailure := errors.New("physical close failed")
	for _, borrowed := range []bool{false, true} {
		for _, tt := range []struct {
			name string
			err  error
			want error
		}{
			{name: "already closed", err: net.ErrClosed},
			{name: "wrapped already closed", err: fmt.Errorf("close: %w", net.ErrClosed)},
			{name: "real failure", err: closeFailure, want: closeFailure},
		} {
			t.Run(fmt.Sprintf("borrowed_%t/%s", borrowed, tt.name), func(t *testing.T) {
				ctx := context.Background()
				pool := &Gpool[internalTestClient]{}
				pool.GpoolInit2(ctx, "test", 0, 1000, &GPoolConfig[internalTestClient]{
					MaxConns: 1, MaxIdleConns: 1,
					Cfunc: func(context.Context, string, int, int) (Conn[internalTestClient], error) {
						return newCloseTestTConn(TH_PRO_FRAMED, &closeResultTransport{
							TMemoryBuffer: thrift.NewTMemoryBuffer(), open: true, closeErr: tt.err,
						}), nil
					},
				})
				if borrowed {
					conn, err := pool.Get(ctx)
					if err != nil {
						t.Fatal(err)
					}
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					if err := pool.Close(cancelled); !errors.Is(err, context.Canceled) {
						t.Fatalf("Close() while borrowed = %v, want context.Canceled", err)
					}
					conn.Close(ctx)
				}
				for i := 0; i < 2; i++ {
					if err := pool.Close(ctx); !errors.Is(err, tt.want) {
						t.Fatalf("Close() = %v, want %v", err, tt.want)
					}
				}
				assertInternalPoolState(t, pool, 0, 0)
			})
		}
	}
}

func TestPoolConnConcurrentReleaseDuringShutdown(t *testing.T) {
	ctx := context.Background()
	pool := newInternalTestPool(1, 1)
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	physical := conn.Gc.(*internalTestConn)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				conn.Discard(ctx)
			} else {
				conn.Close(ctx)
			}
		}()
	}
	close(start)
	if err := pool.Close(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	assertInternalPoolState(t, pool, 0, 0)
	if got := physical.closeCalls.Load(); got != 1 {
		t.Fatalf("physical close calls = %d, want 1", got)
	}
}

func TestGpoolThriftCallReturnsBorrowError(t *testing.T) {
	pool := newInternalTestPool(1, 1)
	ctx := context.Background()
	if err := pool.Close(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := pool.ThriftCall(ctx, "unused")
	if result != nil || !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("ThriftCall() = (%v, %v), want (nil, ErrPoolClosed)", result, err)
	}
}
