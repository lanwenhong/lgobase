//go:build performance

package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	selectorpkg "github.com/lanwenhong/lgobase/selector"
)

const (
	benchmarkConnectTimeoutMS  = 5_000
	latencyProfileWorkers      = 16
	latencyProfileOpsPerWorker = 500
)

type tcpPoolBenchmarkProtocol struct {
	name      string
	serverTLS *tls.Config
	clientTLS *tls.Config
}

type framedEchoServer struct {
	listener net.Listener

	mu    sync.Mutex
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
}

var nextTCPPoolBenchmarkPort atomic.Uint32

func startFramedEchoServer(tb testing.TB, tlsConf *tls.Config) *framedEchoServer {
	tb.Helper()
	rawListener, err := listenOnUniqueBenchmarkPort()
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	listener := rawListener
	if tlsConf != nil {
		listener = tls.NewListener(rawListener, tlsConf)
	}
	server := &framedEchoServer{
		listener: listener,
		conns:    make(map[net.Conn]struct{}),
	}
	server.wg.Add(1)
	go server.accept()
	return server
}

func listenOnUniqueBenchmarkPort() (net.Listener, error) {
	var lastErr error
	for attempt := 0; attempt < 1_000; attempt++ {
		// Use a new non-ephemeral destination port for every server created by the
		// process. This avoids colliding with TIME_WAIT tuples across -count runs.
		port := 20_000 + int(nextTCPPoolBenchmarkPort.Add(1)%20_000)
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			return listener, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("no benchmark port available: %w", lastErr)
}

func (s *framedEchoServer) address() string {
	return s.listener.Addr().String()
}

func (s *framedEchoServer) accept() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serve(conn)
	}
}

func (s *framedEchoServer) serve(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = conn.Close()
	}()

	header := make([]byte, 4)
	payload := make([]byte, 0, 64*1024)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		length := int(binary.BigEndian.Uint32(header))
		if length < 0 || length > 1<<20 {
			return
		}
		if cap(payload) < length {
			payload = make([]byte, length)
		} else {
			payload = payload[:length]
		}
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		if err := writeAll(conn, header); err != nil {
			return
		}
		if err := writeAll(conn, payload); err != nil {
			return
		}
	}
}

func (s *framedEchoServer) close() {
	_ = s.listener.Close()
	s.mu.Lock()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func benchmarkFrame(payloadSize int) ([]byte, []byte) {
	request := make([]byte, 4+payloadSize)
	binary.BigEndian.PutUint32(request[:4], uint32(payloadSize))
	for i := 4; i < len(request); i++ {
		request[i] = byte(i)
	}
	return request, make([]byte, len(request))
}

func benchmarkRoundTrip(conn net.Conn, request, response []byte) error {
	if err := writeAll(conn, request); err != nil {
		return err
	}
	_, err := io.ReadFull(conn, response)
	return err
}

func benchmarkRawConn(conn TcpConnInter) (net.Conn, error) {
	switch raw := conn.(type) {
	case *TcpConn:
		return raw.Conn, nil
	case *TcpSslConn:
		return raw.Conn, nil
	default:
		return nil, fmt.Errorf("unsupported benchmark connection type %T", conn)
	}
}

func benchmarkProtocols(tb testing.TB) []tcpPoolBenchmarkProtocol {
	tb.Helper()
	serverTLS, clientTLS := benchmarkTLSConfigs(tb)
	return []tcpPoolBenchmarkProtocol{
		{name: "TCP"},
		{name: "TLS", serverTLS: serverTLS, clientTLS: clientTLS},
	}
}

func benchmarkTLSConfigs(tb testing.TB) (*tls.Config, *tls.Config) {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatalf("generate TLS key: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		tb.Fatalf("create TLS certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		tb.Fatalf("marshal TLS key: %v", err)
	}
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		tb.Fatalf("load TLS key pair: %v", err)
	}
	rootCAs := x509.NewCertPool()
	parsedCertificate, err := x509.ParseCertificate(der)
	if err != nil {
		tb.Fatalf("parse TLS certificate: %v", err)
	}
	rootCAs.AddCert(parsedCertificate)
	return &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		}, &tls.Config{
			RootCAs:    rootCAs,
			ServerName: "127.0.0.1",
			MinVersion: tls.VersionTLS12,
		}
}

func benchmarkEndpoint(tb testing.TB, address string) (string, int) {
	tb.Helper()
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		tb.Fatalf("split server address %q: %v", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		tb.Fatalf("parse server port %q: %v", portText, err)
	}
	return host, port
}

func newBenchmarkTCPPool(tb testing.TB, address string, tlsConf *tls.Config, maxConns, maxIdle int) *GTcpPool[TcpConnInter] {
	tb.Helper()
	host, port := benchmarkEndpoint(tb, address)
	conf := &TcpPoolConfig[TcpConnInter]{
		MaxConns:     maxConns,
		MaxIdleConns: maxIdle,
		MaxWaiters:   4096,
		Cfunc:        NewSingleTcpConn[TcpConnInter],
		TlsConf:      tlsConf,
	}
	pool := &GTcpPool[TcpConnInter]{}
	pool.GTcpPoolInitWithContext(context.Background(), host, port, benchmarkConnectTimeoutMS, conf)
	return pool
}

func dialBenchmarkConn(address string, tlsConf *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: benchmarkConnectTimeoutMS * time.Millisecond}
	if tlsConf == nil {
		return dialer.Dial("tcp", address)
	}
	return tls.DialWithDialer(dialer, "tcp", address, tlsConf)
}

func BenchmarkTCPPool(b *testing.B) {
	disableTCPPoolBenchmarkLogging(b)
	payloads := []struct {
		name string
		size int
	}{
		{name: "64B", size: 64},
		{name: "1KiB", size: 1024},
		{name: "64KiB", size: 64 * 1024},
	}
	ampleConnections := max(32, runtime.GOMAXPROCS(0)*4)

	for _, protocol := range benchmarkProtocols(b) {
		protocol := protocol
		b.Run(protocol.name, func(b *testing.B) {
			// Keep each case's listener alive until every sibling case finishes so
			// macOS cannot immediately recycle a destination port that still has a
			// large number of client-side TIME_WAIT tuples from reconnect tests.
			newServer := func(tb testing.TB) *framedEchoServer {
				server := startFramedEchoServer(tb, protocol.serverTLS)
				b.Cleanup(server.close)
				return server
			}
			for _, payload := range payloads {
				payload := payload
				b.Run("Payload_"+payload.name, func(b *testing.B) {
					b.Run("DedicatedSerial", func(b *testing.B) {
						server := newServer(b)
						benchmarkDedicatedSerial(b, server.address(), protocol.clientTLS, payload.size)
					})
					b.Run("PoolSerial", func(b *testing.B) {
						server := newServer(b)
						benchmarkPoolSerial(b, server.address(), protocol.clientTLS, payload.size, 1, 1)
					})
					b.Run("DedicatedParallel", func(b *testing.B) {
						server := newServer(b)
						benchmarkDedicatedParallel(b, server.address(), protocol.clientTLS, payload.size)
					})
					b.Run("PoolParallelAmple", func(b *testing.B) {
						server := newServer(b)
						benchmarkPoolParallel(b, server.address(), protocol.clientTLS, payload.size, ampleConnections)
					})
					b.Run("PoolParallelMax8", func(b *testing.B) {
						server := newServer(b)
						benchmarkPoolParallel(b, server.address(), protocol.clientTLS, payload.size, 8)
					})
					b.Run("PoolParallelMax1", func(b *testing.B) {
						server := newServer(b)
						benchmarkPoolParallel(b, server.address(), protocol.clientTLS, payload.size, 1)
					})
					if payload.size == 64 {
						b.Run("DialEachSerial", func(b *testing.B) {
							server := newServer(b)
							benchmarkDialEachSerial(b, server.address(), protocol.clientTLS, payload.size)
						})
						b.Run("PoolChurnSerial", func(b *testing.B) {
							server := newServer(b)
							benchmarkPoolSerial(b, server.address(), protocol.clientTLS, payload.size, 1, 0)
						})
					}
				})
			}
		})
	}
}

func BenchmarkTCPPoolSelectorRoundRobin(b *testing.B) {
	disableTCPPoolBenchmarkLogging(b)
	cases := []struct {
		name         string
		nodes        int
		invalidEvery int
	}{
		{name: "Nodes1", nodes: 1},
		{name: "Nodes3", nodes: 3},
		{name: "Nodes16", nodes: 16},
		{name: "Nodes16HalfInvalid", nodes: 16, invalidEvery: 2},
	}
	for _, benchmarkCase := range cases {
		benchmarkCase := benchmarkCase
		poolSelector := &TcpPoolSelector[TcpConnInter]{}
		poolSelector.Slist = make([]selectorpkg.SvrAddr, benchmarkCase.nodes)
		for i := range poolSelector.Slist {
			server := &TcpRpcSvr[TcpConnInter]{}
			server.SetAddr("benchmark-node")
			server.SetPort(20_000 + i)
			server.SetStat(selectorpkg.SVR_VALID)
			if benchmarkCase.invalidEvery > 0 && i%benchmarkCase.invalidEvery == 0 {
				server.SetStat(selectorpkg.SVR_NOTVALID)
			}
			poolSelector.Slist[i] = server
		}
		b.Run(benchmarkCase.name+"/Serial", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if poolSelector.RoundRobin(context.Background()) == nil {
					b.Fatal("RoundRobin returned nil")
				}
			}
		})
		b.Run(benchmarkCase.name+"/Parallel", func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if poolSelector.RoundRobin(context.Background()) == nil {
						b.Error("RoundRobin returned nil")
					}
				}
			})
		})
	}
}

func benchmarkDedicatedSerial(b *testing.B, address string, tlsConf *tls.Config, payloadSize int) {
	b.StopTimer()
	conn, err := dialBenchmarkConn(address, tlsConf)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	request, response := benchmarkFrame(payloadSize)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := benchmarkRoundTrip(conn, request, response); err != nil {
			b.Fatalf("round trip: %v", err)
		}
	}
	b.StopTimer()
	_ = conn.Close()
}

func benchmarkPoolSerial(b *testing.B, address string, tlsConf *tls.Config, payloadSize, maxConns, maxIdle int) {
	b.StopTimer()
	pool := newBenchmarkTCPPool(b, address, tlsConf, maxConns, maxIdle)
	request, response := benchmarkFrame(payloadSize)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		borrowed, err := pool.Get(context.Background())
		if err != nil {
			b.Fatalf("pool Get: %v", err)
		}
		conn, err := benchmarkRawConn(borrowed.Gc)
		if err == nil {
			err = benchmarkRoundTrip(conn, request, response)
		}
		if maxIdle == 0 {
			setBenchmarkLingerZero(conn)
		}
		borrowed.Close(context.Background())
		if err != nil {
			b.Fatalf("round trip: %v", err)
		}
	}
	b.StopTimer()
	if err := pool.Close(context.Background()); err != nil {
		b.Fatalf("pool Close: %v", err)
	}
}

func benchmarkDedicatedParallel(b *testing.B, address string, tlsConf *tls.Config, payloadSize int) {
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	var firstErr error
	var errorOnce sync.Once
	b.RunParallel(func(pb *testing.PB) {
		conn, err := dialBenchmarkConn(address, tlsConf)
		if err != nil {
			errorOnce.Do(func() { firstErr = err })
			for pb.Next() {
			}
			return
		}
		defer conn.Close()
		request, response := benchmarkFrame(payloadSize)
		for pb.Next() {
			if err := benchmarkRoundTrip(conn, request, response); err != nil {
				errorOnce.Do(func() { firstErr = err })
				return
			}
		}
	})
	if firstErr != nil {
		b.Fatalf("parallel direct connection: %v", firstErr)
	}
}

func benchmarkPoolParallel(b *testing.B, address string, tlsConf *tls.Config, payloadSize, maxConns int) {
	b.StopTimer()
	pool := newBenchmarkTCPPool(b, address, tlsConf, maxConns, maxConns)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	var firstErr error
	var errorOnce sync.Once
	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		request, response := benchmarkFrame(payloadSize)
		for pb.Next() {
			borrowed, err := pool.Get(context.Background())
			if err != nil {
				errorOnce.Do(func() { firstErr = err })
				for pb.Next() {
				}
				return
			}
			conn, rawErr := benchmarkRawConn(borrowed.Gc)
			if rawErr == nil {
				rawErr = benchmarkRoundTrip(conn, request, response)
			}
			borrowed.Close(context.Background())
			if rawErr != nil {
				errorOnce.Do(func() { firstErr = rawErr })
				for pb.Next() {
				}
				return
			}
		}
	})
	b.StopTimer()
	if err := pool.Close(context.Background()); err != nil {
		b.Fatalf("pool Close: %v", err)
	}
	if firstErr != nil {
		b.Fatalf("parallel pool: %v", firstErr)
	}
}

func benchmarkDialEachSerial(b *testing.B, address string, tlsConf *tls.Config, payloadSize int) {
	request, response := benchmarkFrame(payloadSize)
	b.SetBytes(int64(payloadSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := dialBenchmarkConn(address, tlsConf)
		if err == nil {
			err = benchmarkRoundTrip(conn, request, response)
		}
		if conn != nil {
			setBenchmarkLingerZero(conn)
			_ = conn.Close()
		}
		if err != nil {
			b.Fatalf("dial and round trip: %v", err)
		}
	}
}

func setBenchmarkLingerZero(conn net.Conn) {
	var tcpConn *net.TCPConn
	switch typed := conn.(type) {
	case *net.TCPConn:
		tcpConn = typed
	case *tls.Conn:
		tcpConn, _ = typed.NetConn().(*net.TCPConn)
	}
	if tcpConn != nil {
		_ = tcpConn.SetLinger(0)
	}
}

type latencyProfileResult struct {
	get   time.Duration
	rpc   time.Duration
	total time.Duration
}

func TestTCPPoolLatencyProfile(t *testing.T) {
	disableTCPPoolBenchmarkLogging(t)
	scenarios := []struct {
		name     string
		maxConns int
	}{
		{name: "Ample", maxConns: latencyProfileWorkers},
		{name: "Max4", maxConns: 4},
		{name: "Max1", maxConns: 1},
	}
	for _, protocol := range benchmarkProtocols(t) {
		server := startFramedEchoServer(t, protocol.serverTLS)
		for _, scenario := range scenarios {
			pool := newBenchmarkTCPPool(t, server.address(), protocol.clientTLS, scenario.maxConns, scenario.maxConns)
			results := make([]latencyProfileResult, latencyProfileWorkers*latencyProfileOpsPerWorker)
			errCh := make(chan error, latencyProfileWorkers)
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(latencyProfileWorkers)
			started := time.Now()
			for worker := 0; worker < latencyProfileWorkers; worker++ {
				worker := worker
				go func() {
					defer wg.Done()
					request, response := benchmarkFrame(1024)
					<-start
					for operation := 0; operation < latencyProfileOpsPerWorker; operation++ {
						index := worker*latencyProfileOpsPerWorker + operation
						totalStarted := time.Now()
						getStarted := totalStarted
						borrowed, err := pool.Get(context.Background())
						getFinished := time.Now()
						if err != nil {
							errCh <- fmt.Errorf("pool Get: %w", err)
							return
						}
						conn, err := benchmarkRawConn(borrowed.Gc)
						if err == nil {
							err = benchmarkRoundTrip(conn, request, response)
						}
						rpcFinished := time.Now()
						borrowed.Close(context.Background())
						if err != nil {
							errCh <- fmt.Errorf("round trip: %w", err)
							return
						}
						results[index] = latencyProfileResult{
							get:   getFinished.Sub(getStarted),
							rpc:   rpcFinished.Sub(getFinished),
							total: time.Since(totalStarted),
						}
					}
				}()
			}
			started = time.Now()
			close(start)
			wg.Wait()
			elapsed := time.Since(started)
			close(errCh)
			for err := range errCh {
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := pool.Close(context.Background()); err != nil {
				t.Fatalf("pool Close: %v", err)
			}
			getP50, getP95, getP99 := latencyPercentiles(results, func(result latencyProfileResult) time.Duration { return result.get })
			rpcP50, rpcP95, rpcP99 := latencyPercentiles(results, func(result latencyProfileResult) time.Duration { return result.rpc })
			totalP50, totalP95, totalP99 := latencyPercentiles(results, func(result latencyProfileResult) time.Duration { return result.total })
			qps := float64(len(results)) / elapsed.Seconds()
			t.Logf("TCP_POOL_PERF protocol=%s scenario=%s workers=%d max_conns=%d payload_bytes=1024 operations=%d elapsed=%s qps=%.0f get_p50=%s get_p95=%s get_p99=%s rpc_p50=%s rpc_p95=%s rpc_p99=%s total_p50=%s total_p95=%s total_p99=%s",
				protocol.name, scenario.name, latencyProfileWorkers, scenario.maxConns, len(results), elapsed, qps,
				getP50, getP95, getP99, rpcP50, rpcP95, rpcP99, totalP50, totalP95, totalP99)
		}
		server.close()
	}
}

func disableTCPPoolBenchmarkLogging(tb testing.TB) {
	tb.Helper()
	previous := logger.Gfilelog
	logger.Gfilelog = nil
	tb.Cleanup(func() { logger.Gfilelog = previous })
}

func latencyPercentiles(results []latencyProfileResult, value func(latencyProfileResult) time.Duration) (time.Duration, time.Duration, time.Duration) {
	values := make([]time.Duration, len(results))
	for i, result := range results {
		values[i] = value(result)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	percentile := func(p float64) time.Duration {
		index := int(float64(len(values)-1) * p)
		return values[index]
	}
	return percentile(0.50), percentile(0.95), percentile(0.99)
}
