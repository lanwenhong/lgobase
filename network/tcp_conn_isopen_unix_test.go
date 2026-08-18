//go:build !windows && !wasm

package network

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

type tcpAcceptResult struct {
	conn *net.TCPConn
	err  error
}

type tlsAcceptResult struct {
	conn *tls.Conn
	err  error
}

func newTLSServerConfig(t *testing.T) *tls.Config {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certificateDER},
			PrivateKey:  privateKey,
		}},
	}
}

func newTCPTestPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()

	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenTCP() error = %v", err)
	}
	acceptResult := make(chan tcpAcceptResult, 1)
	go func() {
		conn, acceptErr := listener.AcceptTCP()
		acceptResult <- tcpAcceptResult{conn: conn, err: acceptErr}
	}()

	client, err := net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		_ = listener.Close()
		t.Fatalf("DialTCP() error = %v", err)
	}
	accepted := <-acceptResult
	_ = listener.Close()
	if accepted.err != nil {
		_ = client.Close()
		t.Fatalf("AcceptTCP() error = %v", accepted.err)
	}

	t.Cleanup(func() {
		_ = client.Close()
		_ = accepted.conn.Close()
	})
	return client, accepted.conn
}

func newTLSTestPair(t *testing.T) (*tls.Conn, *tls.Conn) {
	t.Helper()

	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenTCP() error = %v", err)
	}
	acceptResult := make(chan tlsAcceptResult, 1)
	serverConfig := newTLSServerConfig(t)
	go func() {
		rawConn, acceptErr := listener.AcceptTCP()
		if acceptErr != nil {
			acceptResult <- tlsAcceptResult{err: acceptErr}
			return
		}
		serverConn := tls.Server(rawConn, serverConfig)
		acceptErr = serverConn.Handshake()
		acceptResult <- tlsAcceptResult{conn: serverConn, err: acceptErr}
	}()

	rawClient, err := net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		_ = listener.Close()
		t.Fatalf("DialTCP() error = %v", err)
	}
	client := tls.Client(rawClient, &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	})
	if err := client.Handshake(); err != nil {
		_ = rawClient.Close()
		_ = listener.Close()
		t.Fatalf("client TLS Handshake() error = %v", err)
	}
	accepted := <-acceptResult
	_ = listener.Close()
	if accepted.err != nil {
		_ = client.Close()
		t.Fatalf("server TLS Handshake() error = %v", accepted.err)
	}

	t.Cleanup(func() {
		_ = client.NetConn().Close()
		_ = accepted.conn.NetConn().Close()
	})
	return client, accepted.conn
}

func requireEventuallyClosed(t *testing.T, isOpen func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !isOpen() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("IsOpen() remained true after the peer closed the connection")
}

func TestTcpConnIsOpenUsesUnixSocketState(t *testing.T) {
	ctx := context.Background()

	t.Run("live connection without data", func(t *testing.T) {
		client, _ := newTCPTestPair(t)
		conn := &TcpConn{Conn: client, Opened: true}
		if !conn.IsOpen(ctx) {
			t.Fatal("IsOpen() = false for a live connection")
		}
	})

	t.Run("FIN", func(t *testing.T) {
		client, server := newTCPTestPair(t)
		conn := &TcpConn{Conn: client, Opened: true}
		if err := server.Close(); err != nil {
			t.Fatalf("server Close() error = %v", err)
		}
		requireEventuallyClosed(t, func() bool { return conn.IsOpen(ctx) })
		if conn.Opened {
			t.Fatal("Opened remained true after FIN was detected")
		}
	})

	t.Run("pending data is not consumed", func(t *testing.T) {
		client, server := newTCPTestPair(t)
		conn := &TcpConn{Conn: client, Opened: true}
		if _, err := server.Write([]byte{0x7a}); err != nil {
			t.Fatalf("server Write() error = %v", err)
		}
		if !conn.IsOpen(ctx) {
			t.Fatal("IsOpen() = false while unread data is pending")
		}
		got, err := conn.Readn(ctx, 1)
		if err != nil {
			t.Fatalf("Readn() error = %v", err)
		}
		if len(got) != 1 || got[0] != 0x7a {
			t.Fatalf("Readn() = %v, want [122]", got)
		}
	})

	t.Run("RST", func(t *testing.T) {
		client, server := newTCPTestPair(t)
		conn := &TcpConn{Conn: client, Opened: true}
		if err := server.SetLinger(0); err != nil {
			t.Fatalf("SetLinger(0) error = %v", err)
		}
		if err := server.Close(); err != nil {
			t.Fatalf("server Close() error = %v", err)
		}
		requireEventuallyClosed(t, func() bool { return conn.IsOpen(ctx) })
	})
}

func TestTcpSslConnIsOpenChecksUnderlyingUnixSocket(t *testing.T) {
	ctx := context.Background()
	client, server := newTLSTestPair(t)
	conn := &TcpSslConn{Conn: client, Opened: true}

	if !conn.IsOpen(ctx) {
		t.Fatal("IsOpen() = false after a successful TLS handshake")
	}
	if err := server.NetConn().Close(); err != nil {
		t.Fatalf("server underlying TCP Close() error = %v", err)
	}
	requireEventuallyClosed(t, func() bool { return conn.IsOpen(ctx) })
}

func TestTcpSslConnIsOpenSeesTLSCloseNotifyAsPendingData(t *testing.T) {
	ctx := context.Background()
	client, server := newTLSTestPair(t)
	conn := &TcpSslConn{Conn: client, Opened: true}

	if err := server.Close(); err != nil {
		t.Fatalf("server TLS Close() error = %v", err)
	}
	if !conn.IsOpen(ctx) {
		t.Fatal("IsOpen() = false before the pending TLS close_notify was consumed")
	}
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	buffer := make([]byte, 1)
	if _, err := client.Read(buffer); !errors.Is(err, io.EOF) {
		t.Fatalf("TLS Read() error = %v, want EOF", err)
	}
	requireEventuallyClosed(t, func() bool { return conn.IsOpen(ctx) })
}

func TestTCPPoolReplacesFINClosedIdleConnectionBeforeBorrow(t *testing.T) {
	ctx := context.Background()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenTCP() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan tcpAcceptResult, 4)
	go func() {
		for range 4 {
			conn, acceptErr := listener.AcceptTCP()
			accepted <- tcpAcceptResult{conn: conn, err: acceptErr}
		}
	}()

	var created []*TcpConn
	factory := func(_ context.Context, _ string, _ int, timeout int, _ *tls.Config) TcpConnInter {
		conn := NewTcpConn(
			listener.Addr().String(),
			time.Duration(timeout)*time.Millisecond,
			time.Duration(timeout)*time.Millisecond,
			time.Duration(timeout)*time.Millisecond,
		)
		created = append(created, conn)
		return conn
	}
	conf := &TcpPoolConfig[*TcpConn]{
		MaxConns:     3,
		MaxIdleConns: 3,
		MaxWaiters:   3,
		Cfunc:        factory,
	}
	pool := &GTcpPool[*TcpConn]{}
	pool.GTcpPoolInitWithContext(ctx, "127.0.0.1", 1, 1_000, conf)
	t.Cleanup(func() {
		if closeErr := pool.Close(context.Background()); closeErr != nil {
			t.Errorf("pool Close() error = %v", closeErr)
		}
	})

	if len(created) != 3 {
		t.Fatalf("initial connections = %d, want 3", len(created))
	}
	initialPeers := make([]*net.TCPConn, 0, 3)
	for range 3 {
		result := <-accepted
		if result.err != nil {
			t.Fatalf("AcceptTCP() error = %v", result.err)
		}
		initialPeers = append(initialPeers, result.conn)
	}
	for _, peer := range initialPeers {
		if closeErr := peer.Close(); closeErr != nil {
			t.Fatalf("peer Close() error = %v", closeErr)
		}
	}
	for _, conn := range created {
		requireEventuallyClosed(t, func() bool { return isTCPConnOpen(conn.Conn) })
	}

	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer borrowed.Close(ctx)
	if len(created) != 4 {
		t.Fatalf("connections after Get() = %d, want 4", len(created))
	}
	if borrowed.Gc != created[3] {
		t.Fatal("Get() did not replace the FIN-closed idle connection")
	}
	newPeer := <-accepted
	if newPeer.err != nil {
		t.Fatalf("accept replacement connection error = %v", newPeer.err)
	}
	t.Cleanup(func() { _ = newPeer.conn.Close() })
}

func TestTCPPoolReplacesFINClosedTLSIdleConnectionBeforeBorrow(t *testing.T) {
	ctx := context.Background()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenTCP() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	serverConfig := newTLSServerConfig(t)
	accepted := make(chan tlsAcceptResult, 2)
	go func() {
		for range 2 {
			rawConn, acceptErr := listener.AcceptTCP()
			if acceptErr != nil {
				accepted <- tlsAcceptResult{err: acceptErr}
				continue
			}
			serverConn := tls.Server(rawConn, serverConfig)
			acceptErr = serverConn.Handshake()
			accepted <- tlsAcceptResult{conn: serverConn, err: acceptErr}
		}
	}()

	var created []*TcpSslConn
	clientConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}
	factory := func(_ context.Context, _ string, _ int, timeout int, tlsConf *tls.Config) TcpConnInter {
		conn := NewTcpSslConn(
			listener.Addr().String(),
			time.Duration(timeout)*time.Millisecond,
			time.Duration(timeout)*time.Millisecond,
			time.Duration(timeout)*time.Millisecond,
			tlsConf,
		)
		created = append(created, conn)
		return conn
	}
	conf := &TcpPoolConfig[*TcpSslConn]{
		MaxConns:     1,
		MaxIdleConns: 1,
		MaxWaiters:   1,
		Cfunc:        factory,
		TlsConf:      clientConfig,
	}
	pool := &GTcpPool[*TcpSslConn]{}
	pool.GTcpPoolInitWithContext(ctx, "127.0.0.1", 1, 1_000, conf)
	t.Cleanup(func() {
		if closeErr := pool.Close(context.Background()); closeErr != nil {
			t.Errorf("pool Close() error = %v", closeErr)
		}
	})

	firstPeer := <-accepted
	if firstPeer.err != nil {
		t.Fatalf("initial server TLS Handshake() error = %v", firstPeer.err)
	}
	if len(created) != 1 {
		t.Fatalf("initial TLS connections = %d, want 1", len(created))
	}
	if closeErr := firstPeer.conn.NetConn().Close(); closeErr != nil {
		t.Fatalf("initial server TCP Close() error = %v", closeErr)
	}
	requireEventuallyClosed(t, func() bool {
		return isTCPConnOpen(created[0].Conn.NetConn())
	})

	borrowed, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer borrowed.Close(ctx)
	if len(created) != 2 {
		t.Fatalf("TLS connections after Get() = %d, want 2", len(created))
	}
	if borrowed.Gc != created[1] {
		t.Fatal("Get() did not replace the FIN-closed TLS idle connection")
	}
	secondPeer := <-accepted
	if secondPeer.err != nil {
		t.Fatalf("replacement server TLS Handshake() error = %v", secondPeer.err)
	}
	t.Cleanup(func() { _ = secondPeer.conn.NetConn().Close() })
}
