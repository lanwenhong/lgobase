package network

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/lanwenhong/lgobase/logger"
)

type TcpConn struct {
	Conn          net.Conn
	ConnectTimout time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	Addr          string
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

func (conn *TcpConn) SetRTimeout(ctx context.Context, cTimeout time.Duration) {
	conn.ConnectTimout = cTimeout
}

func (conn *TcpConn) SetWTimeout(ctx context.Context, wTimeout time.Duration) {
	conn.ConnectTimout = wTimeout
}

func (conn *TcpConn) Open(ctx context.Context) error {
	c, err := net.DialTimeout("tcp", conn.Addr, conn.ConnectTimout)
	if err != nil {
		logger.Warnf(ctx, "connect err: %s", err.Error())
		return err
	}
	conn.Conn = c
	return nil
}

func (conn *TcpConn) Close(ctx context.Context) {
	conn.Conn.Close()
}

func (conn *TcpConn) Readn(ctx context.Context, n_byte int) ([]byte, error) {
	if int64(conn.ReadTimeout) > 0 {
		conn.Conn.SetDeadline(time.Now().Add(conn.ReadTimeout))
	}
	b := make([]byte, n_byte)
	n, err := io.ReadFull(conn.Conn, b)
	if err != nil {
		logger.Warnf(ctx, "read err: %s", err.Error())
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
