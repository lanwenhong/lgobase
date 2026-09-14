//go:build !windows && !wasm

package gpool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
)

const closeRegressionTimeout = 2 * time.Second

type closeRegressionClient struct{ err error }

func (c *closeRegressionClient) Fail(context.Context) (int32, error) { return 0, c.err }

type closeRegressionFixture struct {
	t       *testing.T
	proto   int
	created []*TConn[closeRegressionClient]
	peers   []net.Conn
}

func newCloseRegressionSocketPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("Socketpair() error = %v", err)
	}
	files := []*os.File{os.NewFile(uintptr(fds[0]), "pool-client"), os.NewFile(uintptr(fds[1]), "pool-peer")}
	conns := make([]net.Conn, 2)
	for i, file := range files {
		conns[i], err = net.FileConn(file)
		_ = file.Close()
		if err != nil {
			for j := 0; j < i; j++ {
				_ = conns[j].Close()
			}
			for j := i + 1; j < len(files); j++ {
				_ = files[j].Close()
			}
			t.Fatalf("FileConn() error = %v", err)
		}
	}
	t.Cleanup(func() { _ = conns[0].Close(); _ = conns[1].Close() })
	return conns[0], conns[1]
}

func (f *closeRegressionFixture) create(_ context.Context, _ string, _ int, _ int) (Conn[closeRegressionClient], error) {
	client, peer := newCloseRegressionSocketPair(f.t)
	socket := thrift.NewTSocketFromConnConf(client, nil)
	tc := &TConn[closeRegressionClient]{Protocol: f.proto, TSock: socket, Tbp: thrift.NewTBinaryProtocolFactoryDefault(), isOpen: true}
	if f.proto == TH_PRO_FRAMED {
		tc.Tft = thrift.NewTFramedTransport(socket)
	} else {
		tc.Tbt = thrift.NewTBufferedTransport(socket, 8192)
	}
	f.created = append(f.created, tc)
	f.peers = append(f.peers, peer)
	return tc, nil
}

func newCloseRegressionPool(t *testing.T, proto int) (*Gpool[closeRegressionClient], *closeRegressionFixture) {
	t.Helper()
	f := &closeRegressionFixture{t: t, proto: proto}
	gp := new(Gpool[closeRegressionClient])
	gp.GpoolInit("socketpair", 0, 1000, 1, 1, 0, f.create,
		func(thrift.TTransport, thrift.TProtocolFactory) *closeRegressionClient {
			return &closeRegressionClient{}
		})
	if gp.IdleCount() != 1 {
		t.Fatalf("IdleCount() = %d, want 1", gp.IdleCount())
	}
	return gp, f
}

func captureCloseRegressionLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	previousGlog, previousSlog := logger.Gfilelog, slog.Default()
	logger.NewDefaultGLog()
	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { logger.Gfilelog = previousGlog; slog.SetDefault(previousSlog) })
	return &output
}

func requireNoClosedConnectionWarning(t *testing.T, output *bytes.Buffer) {
	t.Helper()
	got := output.String()
	if strings.Contains(got, "close pool connection failed") || strings.Contains(got, "close idle connection during pool shutdown failed") || strings.Contains(got, "use of closed network connection") {
		t.Fatalf("unexpected closed-connection warning:\n%s", got)
	}
}

func closePeerAndObserveEOF(t *testing.T, tc *TConn[closeRegressionClient], peer net.Conn) {
	t.Helper()
	if err := peer.Close(); err != nil {
		t.Fatalf("peer.Close() error = %v", err)
	}
	if err := tc.TSock.Conn().SetReadDeadline(time.Now().Add(closeRegressionTimeout)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	var b [1]byte
	if _, err := tc.TSock.Conn().Read(b[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("socket Read() error = %v, want EOF", err)
	}
}

func TestCloseRegressionReplacesPeerClosedIdleThriftConnection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proto int
	}{{"framed", TH_PRO_FRAMED}, {"buffered", TH_PRO_BUFFER}} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureCloseRegressionLogs(t)
			gp, fixture := newCloseRegressionPool(t, tc.proto)
			first, err := gp.Get(context.Background())
			if err != nil {
				t.Fatalf("first Get() error = %v", err)
			}
			first.Close(context.Background())
			closePeerAndObserveEOF(t, fixture.created[0], fixture.peers[0])
			replacement, err := gp.Get(context.Background())
			if err != nil {
				t.Fatalf("replacement Get() error = %v", err)
			}
			if replacement == first {
				t.Fatal("Get() reused peer-closed connection")
			}
			if len(fixture.created) != 2 {
				t.Fatalf("created %d connections, want 2", len(fixture.created))
			}
			if gp.InUseCount() != 1 || gp.IdleCount() != 0 {
				t.Fatalf("counts inUse=%d idle=%d, want 1/0", gp.InUseCount(), gp.IdleCount())
			}
			replacement.Close(context.Background())
			if err := gp.Close(context.Background()); err != nil {
				t.Fatalf("pool Close() error = %v", err)
			}
			requireNoClosedConnectionWarning(t, logs)
		})
	}
}

func TestCloseRegressionLegacyThriftCallDiscardsProtocolAndTransportErrors(t *testing.T) {
	transportErr := thrift.NewTTransportExceptionFromError(io.EOF)
	protocolErr := thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, errors.New("bad protocol"))
	tests := []struct {
		name string
		err  error
	}{
		{"transport", transportErr}, {"wrapped_transport", fmt.Errorf("wrapped: %w", transportErr)},
		{"protocol", protocolErr}, {"wrapped_protocol", fmt.Errorf("wrapped: %w", protocolErr)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureCloseRegressionLogs(t)
			gp, fixture := newCloseRegressionPool(t, TH_PRO_FRAMED)
			fixture.created[0].Client.err = tt.err
			if _, err := gp.ThriftCall(context.Background(), "Fail"); !errors.Is(err, tt.err) {
				t.Fatalf("ThriftCall() error = %v, want %v", err, tt.err)
			}
			if gp.IdleCount() != 0 || gp.InUseCount() != 0 {
				t.Fatalf("counts idle=%d inUse=%d, want 0/0", gp.IdleCount(), gp.InUseCount())
			}
			pc, err := gp.Get(context.Background())
			if err != nil {
				t.Fatalf("Get() after failure error = %v", err)
			}
			if pc.Gc == fixture.created[0] {
				t.Fatal("failed connection was reused")
			}
			pc.Close(context.Background())
			if err := gp.Close(context.Background()); err != nil {
				t.Fatalf("pool Close() error = %v", err)
			}
			requireNoClosedConnectionWarning(t, logs)
		})
	}
}

func TestCloseRegressionLegacyThriftCallKeepsConnectionForBusinessError(t *testing.T) {
	logs := captureCloseRegressionLogs(t)
	gp, fixture := newCloseRegressionPool(t, TH_PRO_FRAMED)
	businessErr := errors.New("business failure")
	fixture.created[0].Client.err = businessErr
	if _, err := gp.ThriftCall(context.Background(), "Fail"); !errors.Is(err, businessErr) {
		t.Fatalf("first ThriftCall() error = %v, want %v", err, businessErr)
	}
	if gp.IdleCount() != 1 || gp.InUseCount() != 0 {
		t.Fatalf("counts after business error idle=%d inUse=%d, want 1/0", gp.IdleCount(), gp.InUseCount())
	}
	fixture.created[0].Client.err = nil
	if _, err := gp.ThriftCall(context.Background(), "Fail"); err != nil {
		t.Fatalf("second ThriftCall() error = %v", err)
	}
	if len(fixture.created) != 1 {
		t.Fatalf("created %d connections, want the original connection reused", len(fixture.created))
	}
	if err := gp.Close(context.Background()); err != nil {
		t.Fatalf("pool Close() error = %v", err)
	}
	requireNoClosedConnectionWarning(t, logs)
}

func TestCloseRegressionCall2VariantsDiscardExceptionalConnections(t *testing.T) {
	protocolErr := thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, errors.New("bad protocol"))
	tests := []struct {
		name string
		call func(context.Context, *Gpool[closeRegressionClient]) error
	}{
		{"ThriftCall2", func(ctx context.Context, gp *Gpool[closeRegressionClient]) error {
			return gp.ThriftCall2(ctx, func(interface{}) (string, error) { return "Fail", protocolErr })
		}},
		{"ThriftWithTimeOutCall2", func(ctx context.Context, gp *Gpool[closeRegressionClient]) error {
			return gp.ThriftWithTimeOutCall2(ctx, time.Second, func(interface{}) (string, error) { return "Fail", protocolErr })
		}},
		{"ThriftExtCall2", func(ctx context.Context, gp *Gpool[closeRegressionClient]) error {
			return gp.ThriftExtCall2(ctx, func(context.Context, interface{}) (string, error) { return "Fail", protocolErr })
		}},
		{"ThriftWithTimeOutExtCall2", func(ctx context.Context, gp *Gpool[closeRegressionClient]) error {
			return gp.ThriftWithTimeOutExtCall2(ctx, time.Second, func(context.Context, interface{}) (string, error) { return "Fail", protocolErr })
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureCloseRegressionLogs(t)
			gp, fixture := newCloseRegressionPool(t, TH_PRO_FRAMED)
			if err := tt.call(context.Background(), gp); !errors.Is(err, protocolErr) {
				t.Fatalf("call error = %v, want %v", err, protocolErr)
			}
			if gp.IdleCount() != 0 || gp.InUseCount() != 0 {
				t.Fatalf("counts idle=%d inUse=%d, want 0/0", gp.IdleCount(), gp.InUseCount())
			}
			pc, err := gp.Get(context.Background())
			if err != nil {
				t.Fatalf("Get() after failure error = %v", err)
			}
			if pc.Gc == fixture.created[0] {
				t.Fatal("exceptional connection was reused")
			}
			pc.Close(context.Background())
			if err := gp.Close(context.Background()); err != nil {
				t.Fatalf("pool Close() error = %v", err)
			}
			requireNoClosedConnectionWarning(t, logs)
		})
	}
}

func TestCloseRegressionPoolShutdownIgnoresAlreadyClosedConnections(t *testing.T) {
	for _, state := range []string{"idle", "borrowed"} {
		t.Run(state, func(t *testing.T) {
			logs := captureCloseRegressionLogs(t)
			gp, fixture := newCloseRegressionPool(t, TH_PRO_FRAMED)
			var borrowed *PoolConn[closeRegressionClient]
			if state == "borrowed" {
				var err error
				borrowed, err = gp.Get(context.Background())
				if err != nil {
					t.Fatalf("Get() error = %v", err)
				}
			}
			if err := fixture.created[0].Close(); err != nil {
				t.Fatalf("initial connection Close() error = %v", err)
			}
			if state == "borrowed" {
				cancelled, cancel := context.WithCancel(context.Background())
				cancel()
				if err := gp.Close(cancelled); !errors.Is(err, context.Canceled) {
					t.Fatalf("initial pool Close() error = %v, want context.Canceled", err)
				}
				borrowed.Close(context.Background())
				if err := gp.Close(context.Background()); err != nil {
					t.Fatalf("final pool Close() error = %v", err)
				}
			} else if err := gp.Close(context.Background()); err != nil {
				t.Fatalf("pool Close() error = %v", err)
			}
			requireNoClosedConnectionWarning(t, logs)
		})
	}
}
