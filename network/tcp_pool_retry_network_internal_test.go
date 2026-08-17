package network

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestTCPPoolProcessRetriesAfterServerClosesConnection(t *testing.T) {
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
				// The first request reaches the server, but its connection is
				// closed before a response is sent.
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

	conf := &TcpPoolConfig[*TcpConn]{
		MaxConns:      1,
		MaxIdleConns:  1,
		MaxWaiters:    1,
		RPCMaxRetries: 1,
		Cfunc:         NewSingleTcpConn[*TcpConn],
	}
	pool := &GTcpPool[*TcpConn]{}
	pool.GTcpPoolInitWithContext(context.Background(), host, port, 1_000, conf)
	t.Cleanup(func() {
		if closeErr := pool.Close(context.Background()); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	calls := 0
	err = pool.Process(context.Background(), func(client interface{}) (string, error) {
		calls++
		conn := client.(*TcpConn)
		if writeErr := conn.Writen(context.Background(), []byte{1}); writeErr != nil {
			return "network-retry", writeErr
		}
		_, readErr := conn.Readn(context.Background(), 1)
		return "network-retry", readErr
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("process calls = %d, want 2", calls)
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
		t.Fatal("server did not finish")
	}
}
