package network

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/lanwenhong/lgobase/logger"
)

type TcpSslConn struct {
	Conn          *tls.Conn
	ConnectTimout time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	Addr          string
	TlsConf       *tls.Config
	Opened        bool
}

func NewTcpSslConn(addr string, cTimeout time.Duration, rTimeout time.Duration, wTimeout time.Duration, tslConf *tls.Config) *TcpSslConn {
	conn := &TcpSslConn{
		Addr:          addr,
		ConnectTimout: cTimeout,
		ReadTimeout:   rTimeout,
		WriteTimeout:  wTimeout,
		TlsConf:       tslConf,
	}
	return conn
}

func NewTcpSslFromConn(c *tls.Conn) *TcpSslConn {
	conn := &TcpSslConn{
		Conn: c,
	}
	return conn
}

func (conn *TcpSslConn) SetRTimeout(ctx context.Context, rTimeout time.Duration) {
	conn.ReadTimeout = rTimeout
}

func (conn *TcpSslConn) SetWTimeout(ctx context.Context, wTimeout time.Duration) {
	conn.WriteTimeout = wTimeout
}

func (conn *TcpSslConn) IsOpen(ctx context.Context) bool {
	return conn.Opened
}

func (conn *TcpSslConn) Open(ctx context.Context) error {
	//c, err := net.DialTimeout("tcp", conn.Addr, conn.ConnectTimout)
	logger.Debugf(ctx, "Timeout: %d", conn.ConnectTimout)
	dialer := &net.Dialer{
		Timeout: conn.ConnectTimout, // 连接超时（包括 TCP 握手 + TLS 握手）
	}
	c, err := tls.DialWithDialer(dialer, "tcp", conn.Addr, conn.TlsConf)
	if err != nil {
		logger.Warnf(ctx, "connect err: %s", err.Error())
		return err
	}
	conn.Conn = c
	return nil
}

func (conn *TcpSslConn) Close(ctx context.Context) {
	logger.Debugf(ctx, "conn close")
	conn.Conn.Close()
}

func (conn *TcpSslConn) Readn(ctx context.Context, n_byte int) ([]byte, error) {
	logger.Debugf(ctx, "conn.ReadTimeout: %d", conn.ReadTimeout)
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

func (conn *TcpSslConn) Writen(ctx context.Context, b []byte) error {
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

func (conn *TcpSslConn) SetOptLinger(ctx context.Context, sec int) error {
	tcpConn := conn.Conn.NetConn().(*net.TCPConn)
	err := tcpConn.SetLinger(sec)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
	}
	return err
}
