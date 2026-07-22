package gpool

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

type Conn[T any] interface {
	Init(host string, port int, timeout int) error
	Open() error
	Close() error
	IsOpen() bool
	GetThrfitClient() *T
	SetTimeOut(ctx context.Context, timeout time.Duration)
}

type CreateConn[T any] func(context.Context, string, int, int) (Conn[T], error)

type poolConnState uint32

const (
	poolConnIdle poolConnState = iota
	poolConnBorrowed
	poolConnReturning
	poolConnClosing
	poolConnClosed
)

type PoolConn[T any] struct {
	Gc       Conn[T]
	gp       *Gpool[T]
	Ctime    int64
	IdleTime int64
	state    atomic.Uint32
}

type Gpool[T any] struct {
	idle            idleQueue[T]
	MaxConns        int
	MaxIdleConns    int
	MaxConnLife     int64
	MaxIdleConnLife int64
	// PurgeRate is retained for configuration compatibility. The fixed-capacity
	// idle ring now enforces MaxIdleConns as a hard limit.
	PurgeRate  float64
	hold       int
	inUse      int
	creating   int
	closing    int
	mutex      sync.Mutex
	Cfunc      CreateConn[T]
	Nc         NewThriftClient[T]
	Addr       string
	Port       int
	TimeOut    int
	MaxWaiters int
	Waits      uint
	waiters    waiterQueue[T]
	Gpconf     *GPoolConfig[T]
	closed     bool
	closeDone  chan struct{}
	closeEnded bool
	closeErr   error
}

func (gp *Gpool[T]) GpoolInit(addr string, port int, timeout int,
	maxconns int, maxidleconns int, maxconnlife int64,
	cfunc CreateConn[T], clfunc NewThriftClient[T]) {
	if maxidleconns > maxconns {
		maxidleconns = maxconns
	}
	gp.inUse = 0
	gp.creating = 0
	gp.closing = 0
	gp.waiters = waiterQueue[T]{}
	gp.Waits = 0
	gp.resetCloseState()
	gp.MaxConns = maxconns
	gp.MaxIdleConns = maxidleconns
	gp.idle = newIdleQueue[T](gp.MaxIdleConns)
	gp.MaxConnLife = maxconnlife

	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.MaxWaiters = max(1, maxconns)
	gp.Cfunc = cfunc
	gp.Nc = clfunc

	ctx := context.Background()
	gp.CreateIdleConn(ctx)
}

func (gp *Gpool[T]) GpoolInit2(ctx context.Context, addr string, port int, timeout int, gp_conf *GPoolConfig[T]) {
	gp.inUse = 0
	gp.creating = 0
	gp.closing = 0
	gp.waiters = waiterQueue[T]{}
	gp.Waits = 0
	gp.resetCloseState()
	gp.MaxConns = gp_conf.MaxConns
	gp.MaxIdleConns = gp_conf.MaxIdleConns
	if gp.MaxIdleConns > gp.MaxConns {
		gp.MaxIdleConns = gp.MaxConns
	}
	gp.idle = newIdleQueue[T](gp.MaxIdleConns)
	gp.MaxConnLife = gp_conf.MaxConnLife
	gp.MaxIdleConnLife = gp_conf.MaxIdleConnLife
	gp.PurgeRate = gp_conf.PurgeRate
	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.MaxWaiters = gp_conf.MaxWaiters
	if gp.MaxWaiters <= 0 {
		gp.MaxWaiters = max(1, gp.MaxConns)
		gp_conf.MaxWaiters = gp.MaxWaiters
	}
	gp.Cfunc = gp_conf.Cfunc
	gp.Nc = gp_conf.Nc
	gp.Gpconf = gp_conf

	gp.CreateIdleConn(ctx)
}

func (gp *Gpool[T]) CreateIdleConn(ctx context.Context) {
	for i := 0; i < gp.MaxIdleConns; i++ {
		pc, err := gp.getConnFromNew(ctx)
		if err == nil {
			//c.Close(ctx)
			pc.IdleTime = time.Now().Unix()
			pc.state.Store(uint32(poolConnIdle))
			if !gp.idle.Push(pc) {
				if closeErr := pc.Gc.Close(); closeErr != nil {
					logger.Warn(ctx, "close idle connection after pool initialization failed", "addr", gp.Addr, "port", gp.Port, "err", closeErr)
				}
			}
		}

	}
}

// capacityUsedLocked returns every connection that currently occupies a pool
// slot. Callers must hold gp.mutex.
func (gp *Gpool[T]) capacityUsedLocked() int {
	return gp.idle.Len() + gp.inUse + gp.creating + gp.closing
}

func (gp *Gpool[T]) resetCloseState() {
	gp.closed = false
	gp.closeDone = make(chan struct{})
	gp.closeEnded = false
	gp.closeErr = nil
}

// finishCloseLocked completes pool shutdown after every connection has left
// idle, borrowed, creating, and closing states. Callers must hold gp.mutex.
func (gp *Gpool[T]) finishCloseLocked() {
	if !gp.closed || gp.closeEnded || gp.capacityUsedLocked() != 0 || gp.waiters.Len() != 0 {
		return
	}
	gp.closeEnded = true
	close(gp.closeDone)
}

// IdleCount returns the number of connections currently available for reuse.
func (gp *Gpool[T]) IdleCount() int {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	return gp.idle.Len()
}

// InUseCount returns the number of connections currently borrowed by callers.
func (gp *Gpool[T]) InUseCount() int {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	return gp.inUse
}

// WaiterCount returns the number of callers queued for a connection.
func (gp *Gpool[T]) WaiterCount() int {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	return gp.waiters.Len()
}

// closeConnectionLocked synchronously closes a connection without holding the
// pool mutex. The connection keeps occupying capacity until Close returns. The
// function returns with gp.mutex locked.
func (gp *Gpool[T]) closeConnectionLocked(ctx context.Context, pc *PoolConn[T]) {
	if pc == nil {
		return
	}

	pc.state.Store(uint32(poolConnClosing))
	gp.closing++
	gp.mutex.Unlock()
	err := pc.Gc.Close()
	if err != nil {
		logger.Warn(ctx, "close pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
	}
	gp.mutex.Lock()
	pc.state.Store(uint32(poolConnClosed))
	gp.closing--
	if gp.closed {
		gp.closeErr = errors.Join(gp.closeErr, err)
		gp.finishCloseLocked()
	} else {
		gp.dispatchCapacityWaitersLocked()
	}
}

func (gp *Gpool[T]) getConnFromIdle(ctx context.Context) (*PoolConn[T], error) {
	var reterr error
	pc := gp.idle.Pop()
	pc.state.Store(uint32(poolConnBorrowed))
	var isMaxConnLife bool = false
	var isMaxIdleConnLife bool = false
	connNow := time.Now().Unix()
	if gp.MaxConnLife > 0 && connNow-pc.Ctime > gp.MaxConnLife {
		isMaxConnLife = true
	}

	if gp.MaxIdleConnLife > 0 && connNow-pc.IdleTime > gp.MaxIdleConnLife {
		isMaxIdleConnLife = true
	}

	if !pc.Gc.IsOpen() || isMaxConnLife || isMaxIdleConnLife {
		// reterr = pc.Gc.Open()
		logger.Info(ctx, "reopen RPC pool connection", "created_at", pc.Ctime, "idle_since", pc.IdleTime, "now", connNow, "max_lifetime", gp.MaxConnLife, "max_idle_lifetime", gp.MaxIdleConnLife)
		gp.closeConnectionLocked(ctx, pc)
		//connNew, err := gp.getConnFromNew(ctx)
		connNew, err := gp.getConnFromNewForUse(ctx)
		return connNew, err
	}
	gp.inUse++
	return pc, reterr
}

func (gp *Gpool[T]) getConnFromNew(ctx context.Context) (*PoolConn[T], error) {
	var err error
	pc := new(PoolConn[T])
	pc.gp = gp
	pc.Ctime = time.Now().Unix()
	pc.Gc, err = gp.Cfunc(ctx, gp.Addr, gp.Port, gp.TimeOut)
	if err == nil {
		switch pc.Gc.(type) {
		case *TConn[T]:
			if gp.Nc != nil {
				pc.Gc.(*TConn[T]).NewThClient(gp.Nc)
			}
		}
	} else {
		logger.Warn(ctx, "create pool connection failed", "addr", gp.Addr, "port", gp.Port, "timeout_ms", gp.TimeOut, "err", err)
		pc = nil
	}
	return pc, err
}

func (gp *Gpool[T]) getConnFromNewForUse(ctx context.Context) (*PoolConn[T], error) {
	gp.creating++
	gp.mutex.Unlock()
	pc, err := gp.getConnFromNew(ctx)
	gp.mutex.Lock()
	gp.creating--
	if gp.closed {
		if pc != nil {
			gp.closeConnectionLocked(ctx, pc)
		} else {
			gp.finishCloseLocked()
		}
		return nil, ErrPoolClosed
	}
	if pc != nil {
		pc.state.Store(uint32(poolConnBorrowed))
		gp.inUse++
		return pc, err
	}
	gp.dispatchCapacityWaitersLocked()
	return pc, err

}

func (gp *Gpool[T]) Get(ctx context.Context) (*PoolConn[T], error) {
	gp.mutex.Lock()
	if gp.closed {
		gp.mutex.Unlock()
		return nil, ErrPoolClosed
	}
	if gp.waiters.Len() > 0 {
		return gp.waitForConnectionLocked(ctx)
	}
	if gp.idle.Len() > 0 {
		pc, err := gp.getConnFromIdle(ctx)
		gp.mutex.Unlock()
		return pc, err
	}
	if gp.capacityUsedLocked() < gp.MaxConns {
		pc, err := gp.getConnFromNewForUse(ctx)
		gp.mutex.Unlock()
		return pc, err
	}
	return gp.waitForConnectionLocked(ctx)
}

func (pc *PoolConn[T]) put(ctx context.Context) error {
	gp := pc.gp
	now := time.Now().Unix()
	gp.mutex.Lock()
	if gp.closed {
		gp.inUse--
		pc.IdleTime = now
		gp.closeConnectionLocked(ctx, pc)
		gp.mutex.Unlock()
		return nil
	}

	if gp.waiters.Len() > 0 && gp.connectionReusableForHandoff(pc, now) {
		waiter := gp.waiters.PopFront()
		pc.state.Store(uint32(poolConnBorrowed))
		gp.deliverWaiterLocked(waiter, waitResult[T]{
			kind: waitResultConn,
			conn: pc,
		})
		// Ownership moves directly to the waiter, so inUse is unchanged.
		gp.mutex.Unlock()
		return nil
	}

	gp.inUse--
	pc.IdleTime = now
	pc.state.Store(uint32(poolConnIdle))

	var closeConn *PoolConn[T]
	if gp.waiters.Len() > 0 {
		// The connection was not reusable for direct handoff.
		closeConn = pc
	} else if !gp.idle.Push(pc) {
		if gp.idle.Cap() == 0 {
			closeConn = pc
		} else {
			// Preserve FIFO cleanup semantics: retain the connection that just
			// completed a successful borrow and close the oldest idle one.
			closeConn = gp.idle.Pop()
			_ = gp.idle.Push(pc)
		}
	}
	if closeConn != nil {
		gp.closeConnectionLocked(ctx, closeConn)
	}
	gp.mutex.Unlock()
	return nil
}

// connectionReusableForHandoff is called with gp.mutex held and only on the
// saturated path where a returned connection may bypass the idle ring.
func (gp *Gpool[T]) connectionReusableForHandoff(pc *PoolConn[T], now int64) bool {
	if !pc.Gc.IsOpen() {
		return false
	}
	return gp.MaxConnLife <= 0 || now-pc.Ctime <= gp.MaxConnLife
}

func (pc *PoolConn[T]) Close(ctx context.Context) {
	if !pc.state.CompareAndSwap(uint32(poolConnBorrowed), uint32(poolConnReturning)) {
		return
	}
	_ = pc.put(ctx)
}

// Close stops the pool, rejects new Get calls, wakes queued waiters, and waits
// until idle, borrowed, creating, and closing connections have all been
// physically closed. If ctx expires, shutdown continues in the background and
// a later Close call can wait for the same shutdown to finish.
func (gp *Gpool[T]) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	gp.mutex.Lock()
	if gp.closeDone == nil {
		gp.closeDone = make(chan struct{})
	}

	var closeList []*PoolConn[T]
	if !gp.closed {
		gp.closed = true

		for gp.waiters.Len() > 0 {
			waiter := gp.waiters.PopFront()
			gp.deliverWaiterLocked(waiter, waitResult[T]{err: ErrPoolClosed})
		}

		closeList = make([]*PoolConn[T], 0, gp.idle.Len())
		for gp.idle.Len() > 0 {
			pc := gp.idle.Pop()
			pc.state.Store(uint32(poolConnClosing))
			gp.closing++
			closeList = append(closeList, pc)
		}
		gp.finishCloseLocked()
	}

	done := gp.closeDone
	gp.mutex.Unlock()

	if len(closeList) > 0 {
		go gp.closeIdleConnections(closeList)
	}

	select {
	case <-done:
		gp.mutex.Lock()
		err := gp.closeErr
		gp.mutex.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (gp *Gpool[T]) closeIdleConnections(connections []*PoolConn[T]) {
	ctx := context.Background()
	for _, pc := range connections {
		err := pc.Gc.Close()
		if err != nil {
			logger.Warn(ctx, "close idle connection during pool shutdown failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		}

		gp.mutex.Lock()
		pc.state.Store(uint32(poolConnClosed))
		gp.closing--
		gp.closeErr = errors.Join(gp.closeErr, err)
		gp.finishCloseLocked()
		gp.mutex.Unlock()
	}
}

func (gp *Gpool[T]) ThriftCall(ctx context.Context, method string, arguments ...interface{}) (interface{}, error) {
	var rpc_err error = nil
	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	tconn := pc.Gc.(*TConn[T])

	c := reflect.ValueOf(tconn.Client)
	starttime := time.Now().UnixNano()
	defer func() {
		endTime := time.Now().UnixNano()
		logger.Info(ctx, "thrift rpc call",
			"func", "ThriftCallFramed",
			"module", c.Elem().Type().Name(),
			"method", method,
			"addr", tconn.Addr,
			"port", tconn.Port,
			"timeout_ms", tconn.TimeOut,
			"cost_us", (endTime-starttime)/1000,
			"err", err)
	}()

	function := c.MethodByName(method)
	if !function.IsValid() || function.IsNil() {
		return nil, errors.New("method not found")
	}

	if need := function.Type().NumIn(); need != len(arguments)+1 {
		return nil, errors.New(fmt.Sprintf("arguments number not match, need %d but got %d", need, len(arguments)))
	}

	//callArgs := make([]reflect.Value, len(arguments))
	callArgs := make([]reflect.Value, 0)
	callArgs = append(callArgs, reflect.ValueOf(ctx))
	for _, arg := range arguments {
		//callArgs[i] = reflect.ValueOf(arg)
		callArgs = append(callArgs, reflect.ValueOf(arg))
	}

	var rets []interface{}
	for _, arg := range function.Call(callArgs) {
		rets = append(rets, arg.Interface())
	}
	retlen := len(rets)
	if retlen == 1 && rets[0] != nil {
		rpc_err = rets[0].(error)
	} else if rets[1] != nil {
		rpc_err = rets[1].(error)
	}
	if rpc_err != nil {
		logger.Warn(ctx, "thrift rpc call failed", "method", method, "addr", gp.Addr, "port", gp.Port, "err", rpc_err)
		switch rpc_err.(type) {
		case thrift.TTransportException:
		case thrift.TProtocolException:
			pc.Gc.Close()
		}
	}
	if retlen == 1 {
		return nil, rpc_err
	} else {
		return rets[0], rpc_err
	}
}

func (gp *Gpool[T]) GetCaller(skip int) (fileName string, line string, funcName string) {
	pc, file, iline, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "0", "unknown"
	}

	// 提取函数名
	fn := runtime.FuncForPC(pc)
	if fn != nil {
		funcName = fn.Name()
	}

	// 只取文件名，不要全路径
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	fileName = short

	return fileName, ":" + strconv.Itoa(iline), funcName
}

func (gp *Gpool[T]) ThriftCall2(ctx context.Context, process func(client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	//file, line, fn := gp.GetCaller(2)

	starttime := time.Now()
	defer func() {
		logger.Info(ctx, "gpool rpc call", "func", "ThriftCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "timeout_ms", gp.TimeOut, "cost", time.Since(starttime), "err", rpc_err)
	}()

	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	if err != nil {
		logger.Warn(ctx, "get pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		return err
	}
	client := pc.Gc.GetThrfitClient()
	rpc_name, err = process(client)
	if err != nil {
		rpc_err = err
		logger.Warn(ctx, "gpool rpc call failed", "func", "ThriftCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "err", err)
		switch err.(type) {
		case thrift.TTransportException:
			tte := err.(thrift.TTransportException)
			e_type_id := tte.TypeId()
			logger.Warn(ctx, "thrift transport exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		case thrift.TProtocolException:
			tpe := err.(thrift.TProtocolException)
			e_type_id := tpe.TypeId()
			logger.Warn(ctx, "thrift protocol exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		default:
			logger.Warn(ctx, "gpool rpc error", "err", err)
		}
		return err
	}
	return nil
}

func (gp *Gpool[T]) ThriftWithTimeOutCall2(ctx context.Context, timeout time.Duration, process func(client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	//file, line, fn := gp.GetCaller(2)
	starttime := time.Now()
	defer func() {
		logger.Info(ctx, "gpool rpc call", "func", "ThriftWithTimeOutCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "timeout_ms", gp.TimeOut, "call_timeout", timeout, "cost", time.Since(starttime), "err", rpc_err)
	}()

	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	if err != nil {
		logger.Warn(ctx, "get pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		return err
	}
	pc.Gc.SetTimeOut(ctx, timeout)
	client := pc.Gc.GetThrfitClient()
	//rpc_name, err = process(client)
	_, err = process(client)
	pc.Gc.SetTimeOut(ctx, time.Duration(gp.TimeOut)*time.Millisecond)
	if err != nil {
		rpc_err = err
		logger.Warn(ctx, "gpool rpc call failed", "func", "ThriftWithTimeOutCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "err", err)
		switch err.(type) {
		case thrift.TTransportException:
			tte := err.(thrift.TTransportException)
			e_type_id := tte.TypeId()
			logger.Warn(ctx, "thrift transport exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		case thrift.TProtocolException:
			tpe := err.(thrift.TProtocolException)
			e_type_id := tpe.TypeId()
			logger.Warn(ctx, "thrift protocol exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		default:
			logger.Warn(ctx, "gpool rpc error", "err", err)
		}
		return err
	}
	return nil
}

func (gp *Gpool[T]) ThriftExtCall2(ctx context.Context, process func(ctx context.Context, client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	//file, line, fn := gp.GetCaller(3)
	if eCtx, ok := ctx.(*ExtContext); ok {
		clientService := eCtx.GetReqExtData(THRIFT_EXT_CALL_CLIENT_SERVICE)
		if clientService == "" {
			eCtx = eCtx.SetReqExtCallClientService(eCtx, util.GetEnv("CLIENT_SERVICE", "-"))
			ctx = eCtx
		}
	}
	starttime := time.Now()
	defer func() {
		//nCtx := ctx.(*ExtContext)
		//rid := nCtx.GetReqExtData("request_id")
		logger.Info(ctx, "gpool rpc call", "func", "ThriftExtCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "timeout_ms", gp.TimeOut, "cost", time.Since(starttime), "err", rpc_err)

	}()

	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	if err != nil {
		logger.Warn(ctx, "get pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		rpc_err = err
		return err
	}
	client := pc.Gc.GetThrfitClient()
	//rpc_name, err = process(ctx, client)
	_, err = process(ctx, client)
	if err != nil {
		rpc_err = err
		logger.Warn(ctx, "gpool rpc call failed", "func", "ThriftExtCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "err", err)
		switch err.(type) {
		case thrift.TTransportException:
			tte := err.(thrift.TTransportException)
			e_type_id := tte.TypeId()
			logger.Warn(ctx, "thrift transport exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		case thrift.TProtocolException:
			tpe := err.(thrift.TProtocolException)
			e_type_id := tpe.TypeId()
			logger.Warn(ctx, "thrift protocol exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		default:
			logger.Warn(ctx, "gpool rpc error", "err", err)
		}
		return err
	}
	return nil
}

func (gp *Gpool[T]) ThriftWithTimeOutExtCall2(ctx context.Context, timeout time.Duration, process func(ctx context.Context, client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	//file, line, fn := gp.GetCaller(3)
	if eCtx, ok := ctx.(*ExtContext); ok {
		clientService := eCtx.GetReqExtData(THRIFT_EXT_CALL_CLIENT_SERVICE)
		if clientService == "" {
			eCtx = eCtx.SetReqExtCallClientService(eCtx, util.GetEnv("CLIENT_SERVICE", "-"))
			ctx = eCtx
		}
	}
	starttime := time.Now()
	defer func() {
		//nCtx := ctx.(*ExtContext)
		//rid := nCtx.GetReqExtData("request_id")
		logger.Info(ctx, "gpool rpc call", "func", "ThriftWithTimeOutExtCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "timeout_ms", gp.TimeOut, "call_timeout", timeout, "cost", time.Since(starttime), "err", rpc_err)

	}()

	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	if err != nil {
		logger.Warn(ctx, "get pool connection failed", "addr", gp.Addr, "port", gp.Port, "err", err)
		rpc_err = err
		return err
	}
	pc.Gc.SetTimeOut(ctx, timeout)
	client := pc.Gc.GetThrfitClient()
	//rpc_name, err = process(ctx, client)
	_, err = process(ctx, client)
	pc.Gc.SetTimeOut(ctx, time.Duration(gp.TimeOut)*time.Millisecond)
	if err != nil {
		rpc_err = err
		logger.Warn(ctx, "gpool rpc call failed", "func", "ThriftWithTimeOutExtCall2", "method", rpc_name, "addr", gp.Addr, "port", gp.Port, "err", err)
		switch err.(type) {
		case thrift.TTransportException:
			tte := err.(thrift.TTransportException)
			e_type_id := tte.TypeId()
			logger.Warn(ctx, "thrift transport exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		case thrift.TProtocolException:
			tpe := err.(thrift.TProtocolException)
			e_type_id := tpe.TypeId()
			logger.Warn(ctx, "thrift protocol exception", "type_id", e_type_id, "err", err)
			pc.Gc.Close()
		default:
			logger.Warn(ctx, "gpool rpc error", "err", err)
		}
		return err
	}
	return nil
}
