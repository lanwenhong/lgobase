//go:build !windows && !wasm

package network

import (
	"errors"
	"net"
	"syscall"
	"time"
)

// isTCPConnOpen performs the same non-blocking peer connectivity check used by
// Apache Thrift's TSocket. MSG_PEEK preserves any unread application data.
func isTCPConnOpen(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	syscallConn, ok := conn.(syscall.Conn)
	if !ok {
		// The connection wrapper does not expose its socket. Keep the historical
		// behavior and let the next read or write determine its state.
		return true
	}

	// Clear a deadline left by a previous RPC. The probe itself is non-blocking,
	// and Readn sets the requested deadline again before reading.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return false
	}
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		return false
	}

	var (
		buffer  [1]byte
		n       int
		peekErr error
	)
	if err := rawConn.Read(func(fd uintptr) bool {
		n, _, peekErr = peekTCPNonblocking(int(fd), buffer[:])
		return true
	}); err != nil {
		return false
	}

	if n > 0 {
		return true
	}
	if errors.Is(peekErr, syscall.EAGAIN) || errors.Is(peekErr, syscall.EWOULDBLOCK) {
		return true
	}
	// n == 0 with no error means that the peer sent FIN. Any other error also
	// means the connection is no longer safe for another RPC.
	return false
}
