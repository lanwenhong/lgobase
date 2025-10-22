package network

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lanwenhong/lgobase/logger"
)

type TcpConn struct {
	Conn          net.Conn
	ConnectTimout time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	Addr          string
	Opened        bool
	once          sync.Once
}

func NewTcpConn(addr string, cTimeout time.Duration, rTimeout time.Duration, wTimeout time.Duration) *TcpConn {
	conn := &TcpConn{
		Addr:          addr,
		ConnectTimout: cTimeout,
		ReadTimeout:   rTimeout,
		WriteTimeout:  wTimeout,
	}
	return conn
}

func NewTcpFromConn(c net.Conn) *TcpConn {
	conn := &TcpConn{
		Conn: c,
	}
	return conn
}

func (conn *TcpConn) IsOpen(ctx context.Context) bool {
	return conn.Opened
}
func (conn *TcpConn) SetRTimeout(ctx context.Context, rTimeout time.Duration) {
	conn.ReadTimeout = rTimeout
}

func (conn *TcpConn) SetWTimeout(ctx context.Context, wTimeout time.Duration) {
	conn.WriteTimeout = wTimeout
}

func (conn *TcpConn) Open(ctx context.Context) error {
	c, err := net.DialTimeout("tcp", conn.Addr, conn.ConnectTimout)
	if err != nil {
		logger.Warnf(ctx, "connect err: %s", err.Error())
		return err
	}
	conn.Conn = c
	conn.Opened = true
	return nil
}

func (conn *TcpConn) Close(ctx context.Context) {
	logger.Debugf(ctx, "conn close")
	conn.Opened = false
	conn.once.Do(func() {
		if err := conn.Conn.Close(); err != nil {
			logger.Warnf(ctx, "close err: %s", err.Error())
		}
	})
}

func (conn *TcpConn) Readn(ctx context.Context, n_byte int) ([]byte, error) {
	logger.Debugf(ctx, "conn.ReadTimeout: %d", conn.ReadTimeout)
	if int64(conn.ReadTimeout) > 0 {
		conn.Conn.SetDeadline(time.Now().Add(conn.ReadTimeout))
	}
	b := make([]byte, n_byte)
	n, err := io.ReadFull(conn.Conn, b)
	if err != nil {
		logger.Warnf(ctx, "read err: %s", err.Error())
		conn.Opened = false
	}
	logger.Debugf(ctx, "read: %d", n)
	return b, err

}

func (conn *TcpConn) Writen(ctx context.Context, b []byte) error {
	if int64(conn.WriteTimeout) > 0 {
		conn.Conn.SetDeadline(time.Now().Add(conn.WriteTimeout))
	}
	/*n, err := io.WriteFull(conn.Conn, b)
	if err != nil {
		logger.Warnf(ctx, "write err: %s", err.Error())
	}
	logger.Debugf(ctx, "write: %d", n)
	*/
	var start int = 0
	for {
		n, err := conn.Conn.Write(b[start:])
		if err != nil {
			logger.Warnf(ctx, "write error: %s", err.Error())
			conn.Opened = false
			return err
		}
		logger.Debugf(ctx, "write: %d", n)
		start += n
		if start == len(b) {
			break
		}
	}
	return nil
}

func (conn *TcpConn) SetOptLinger(ctx context.Context, sec int) error {
	tcpConn := conn.Conn.(*net.TCPConn)
	err := tcpConn.SetLinger(sec)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
	}
	return err
}
