//go:build windows || wasm

package network

import "net"

// Unix sockets support the non-consuming MSG_PEEK probe. Other platforms keep
// the former local-state behavior in TcpConn.IsOpen and TcpSslConn.IsOpen.
func isTCPConnOpen(conn net.Conn) bool {
	return conn != nil
}
