//go:build aix

package network

import "syscall"

func peekTCPNonblocking(fd int, buffer []byte) (int, syscall.Sockaddr, error) {
	return syscall.Recvfrom(fd, buffer, syscall.MSG_PEEK|syscall.MSG_NONBLOCK)
}
