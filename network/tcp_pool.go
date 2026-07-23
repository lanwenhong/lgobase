package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/lanwenhong/lgobase/gpool"
	"github.com/lanwenhong/lgobase/logger"
)

type TcpConnInter interface {
	Open(context.Context) error
	Close(context.Context)
	IsOpen(context.Context) bool
}

type CreateTcpConn[T TcpConnInter] func(context.Context, string, int, int, *tls.Config) TcpConnInter

var (
	ErrTcpPoolWaitQueueFull = gpool.ErrPoolWaitQueueFull
	ErrTcpPoolWaitTimeout   = gpool.ErrPoolWaitTimeout
	ErrTcpPoolClosed        = gpool.ErrPoolClosed
)

// tcpCoreConn adapts a raw TCP/TLS connection to gpool's protocol-neutral
// leasing lifecycle. Thrift-specific methods are deliberately no-ops here;
// network.Process uses the raw connection exposed by PoolTcpConn.
type tcpCoreConn struct {
	conn TcpConnInter
}

func (c *tcpCoreConn) Init(string, int, int) error { return nil }

func (c *tcpCoreConn) Open() error {
	return c.conn.Open(context.Background())
}

func (c *tcpCoreConn) Close() error {
	c.conn.Close(context.Background())
	return nil
}

func (c *tcpCoreConn) IsOpen() bool {
	return c.conn.IsOpen(context.Background())
}

func (c *tcpCoreConn) GetThrfitClient() *TcpConnInter {
	return &c.conn
}

func (c *tcpCoreConn) SetTimeOut(context.Context, time.Duration) {}

// PoolTcpConn is a borrowed TCP connection. Close returns the lease to the
// shared pool core and is safe to call more than once.
type PoolTcpConn[T TcpConnInter] struct {
	Gc       TcpConnInter
	gp       *GTcpPool[T]
	coreConn *gpool.PoolConn[TcpConnInter]
	Ctime    int64
	IdleTime int64
	Opened   bool
}

// GTcpPool keeps the existing network API while delegating connection storage,
// FIFO waiting, lifetime checks, idle eviction, and shutdown to gpool.Gpool.
// The only TCP-specific operation left here is connection construction.
type GTcpPool[T TcpConnInter] struct {
	MaxConns        int
	MaxIdleConns    int
	MaxWaiters      int
	MaxConnLife     int64
	MaxIdleConnLife int64
	PurgeRate       float64
	Cfunc           CreateTcpConn[T]
	Addr            string
	Port            int
	TimeOut         int
	Gpconf          *TcpPoolConfig[T]

	core *gpool.Gpool[TcpConnInter]
}

func NewSingleTcpConn[T TcpConnInter](_ context.Context, ip string, port int, timeout int, tlsConf *tls.Config) TcpConnInter {
	cTimeout := time.Duration(timeout) * time.Millisecond
	rTimeout := time.Duration(timeout) * time.Millisecond
	wTimeout := time.Duration(timeout) * time.Millisecond
	addr := fmt.Sprintf("%s:%d", ip, port)
	if tlsConf == nil {
		return NewTcpConn(addr, cTimeout, rTimeout, wTimeout)
	}
	return NewTcpSslConn(addr, cTimeout, rTimeout, wTimeout, tlsConf)
}

// GTcpPoolInit2 is retained for source compatibility. New code that has an
// initialization context should use GTcpPoolInitWithContext.
func (gp *GTcpPool[T]) GTcpPoolInit2(addr string, port int, timeout int, conf *TcpPoolConfig[T]) {
	gp.GTcpPoolInitWithContext(context.Background(), addr, port, timeout, conf)
}

func (gp *GTcpPool[T]) GTcpPoolInitWithContext(ctx context.Context, addr string, port int, timeout int, conf *TcpPoolConfig[T]) {
	if ctx == nil {
		ctx = context.Background()
	}
	gp.MaxConns = conf.MaxConns
	gp.MaxIdleConns = conf.MaxIdleConns
	gp.MaxWaiters = conf.MaxWaiters
	gp.MaxConnLife = conf.MaxConnLife
	gp.MaxIdleConnLife = conf.MaxIdleConnLife
	gp.PurgeRate = conf.PurgeRate
	gp.Cfunc = conf.Cfunc
	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.Gpconf = conf

	coreConf := &gpool.GPoolConfig[TcpConnInter]{
		MaxConns:        conf.MaxConns,
		MaxIdleConns:    conf.MaxIdleConns,
		MaxWaiters:      conf.MaxWaiters,
		MaxConnLife:     conf.MaxConnLife,
		MaxIdleConnLife: conf.MaxIdleConnLife,
		PurgeRate:       conf.PurgeRate,
		Cfunc:           gp.createCoreConn,
	}
	gp.core = &gpool.Gpool[TcpConnInter]{}
	gp.core.GpoolInit2(ctx, addr, port, timeout, coreConf)
	gp.MaxWaiters = coreConf.MaxWaiters
	conf.MaxWaiters = coreConf.MaxWaiters
}

func (gp *GTcpPool[T]) createCoreConn(ctx context.Context, addr string, port int, timeout int) (gpool.Conn[TcpConnInter], error) {
	if gp.Cfunc == nil {
		return nil, errors.New("TCP connection factory is nil")
	}
	conn := gp.Cfunc(ctx, addr, port, timeout, gp.Gpconf.TlsConf)
	if conn == nil {
		return nil, errors.New("TCP connection factory returned nil")
	}
	if err := conn.Open(ctx); err != nil {
		logger.Warn(ctx, "create TCP pool connection failed", "addr", addr, "port", port, "timeout_ms", timeout, "err", err)
		return nil, err
	}
	return &tcpCoreConn{conn: conn}, nil
}

func (gp *GTcpPool[T]) Get(ctx context.Context) (*PoolTcpConn[T], error) {
	if gp.core == nil {
		return nil, errors.New("TCP pool is not initialized")
	}
	coreConn, err := gp.core.Get(ctx)
	if err != nil {
		return nil, err
	}
	adapter, ok := coreConn.Gc.(*tcpCoreConn)
	if !ok || adapter.conn == nil {
		coreConn.Close(ctx)
		return nil, errors.New("TCP pool returned an invalid connection adapter")
	}
	return &PoolTcpConn[T]{
		Gc:       adapter.conn,
		gp:       gp,
		coreConn: coreConn,
		Ctime:    coreConn.Ctime,
		IdleTime: coreConn.IdleTime,
		Opened:   true,
	}, nil
}

func (pc *PoolTcpConn[T]) Close(ctx context.Context) {
	if pc == nil || pc.coreConn == nil {
		return
	}
	pc.Opened = false
	pc.coreConn.Close(ctx)
}

func (gp *GTcpPool[T]) GetFreeLen() int {
	if gp.core == nil {
		return 0
	}
	return gp.core.IdleCount()
}

func (gp *GTcpPool[T]) GetUseLen() int {
	if gp.core == nil {
		return 0
	}
	return gp.core.InUseCount()
}

func (gp *GTcpPool[T]) WaiterCount() int {
	if gp.core == nil {
		return 0
	}
	return gp.core.WaiterCount()
}

// Close rejects new borrows, wakes queued callers, and waits until borrowed
// connections are returned or ctx is canceled.
func (gp *GTcpPool[T]) Close(ctx context.Context) error {
	if gp.core == nil {
		return nil
	}
	return gp.core.Close(ctx)
}

func (gp *GTcpPool[T]) Process(ctx context.Context, process func(client interface{}) (string, error)) error {
	pc, err := gp.Get(ctx)
	if err != nil {
		logger.Warn(ctx, "get TCP pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		return err
	}
	defer pc.Close(ctx)

	var rpcErr error
	rpcName := ""
	started := time.Now()
	defer func() {
		logger.Info(ctx, "TCP RPC call completed",
			"method", rpcName,
			"addr", gp.Addr,
			"port", gp.Port,
			"timeout", time.Duration(gp.TimeOut)*time.Millisecond,
			"cost", time.Since(started),
			"err", rpcErr)
	}()

	rpcName, err = process(pc.Gc)
	if err != nil {
		rpcErr = err
		logger.Warn(ctx, "TCP RPC call failed", "method", rpcName, "addr", gp.Addr, "port", gp.Port, "err", err)
		pc.Gc.Close(ctx)
	}
	return err
}
