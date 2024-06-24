package gpool

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
)

type Conn[T any] interface {
	Init(host string, port int, timeout int) error
	Open() error
	Close() error
	IsOpen() bool
	GetThrfitClient() *T
}

type CreateConn[T any] func(context.Context, string, int, int) (Conn[T], error)

type PoolConn[T any] struct {
	Gc Conn[T]
	gp *Gpool[T]
	e  *list.Element
}

type Gpool[T any] struct {
	FreeList     *list.List
	UseList      *list.List
	MaxConns     int
	MaxIdleConns int
	hold         int
	mutex        sync.Mutex
	Cfunc        CreateConn[T]
	//for thrift
	Nc      NewThriftClient[T]
	Addr    string
	Port    int
	TimeOut int
}

func (gp *Gpool[T]) GpoolInit(addr string, port int, timeout int, maxconns int, maxidleconns int,
	cfunc CreateConn[T], clfunc NewThriftClient[T]) {
	gp.FreeList = list.New()
	gp.UseList = list.New()
	gp.MaxConns = maxconns
	gp.MaxIdleConns = maxidleconns
	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.Cfunc = cfunc
	gp.Nc = clfunc
}

func (gp *Gpool[T]) getConnFromFreeList(ctx context.Context) (*PoolConn[T], error) {
	e := gp.FreeList.Front()
	logger.Debugf(ctx, "after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	var reterr error
	pc := e.Value.(*PoolConn[T])
	if !pc.Gc.IsOpen() {
		logger.Debugf(ctx, "reopen conn")
		reterr = pc.Gc.Open()
		if reterr != nil {
			logger.Warnf(ctx, "open err %s", reterr.Error())
		}
	}
	gp.FreeList.Remove(e)
	e.Value.(*PoolConn[T]).e = gp.UseList.PushBack(e.Value)
	return e.Value.(*PoolConn[T]), reterr
}

func (gp *Gpool[T]) getConnFromNew(ctx context.Context) (*PoolConn[T], error) {
	var err error
	pc := new(PoolConn[T])
	pc.gp = gp
	logger.Debugf(ctx, "----%v", gp.Cfunc)
	pc.Gc, err = gp.Cfunc(ctx, gp.Addr, gp.Port, gp.TimeOut)
	if err == nil {
		switch pc.Gc.(type) {
		case *TConn[T]:
			if gp.Nc != nil {
				pc.Gc.(*TConn[T]).NewThClient(gp.Nc)
			}
		}
		pc.e = gp.UseList.PushBack(pc)
	} else {
		logger.Warnf(ctx, "new conn %s", err.Error())
		pc = nil
	}
	logger.Debugf(ctx, "after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	return pc, err
}

func (gp *Gpool[T]) Get(ctx context.Context) (*PoolConn[T], error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if gp.FreeList.Len()+gp.UseList.Len() < gp.MaxConns {
		if gp.FreeList.Len() > 0 {
			logger.Debugf(ctx, "get conn from freelist")
			return gp.getConnFromFreeList(ctx)
		} else {
			logger.Debugf(ctx, "get conn from new")
			return gp.getConnFromNew(ctx)
		}
	} else {
		logger.Debugf(ctx, "pool full flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
		flen := gp.FreeList.Len()
		/*for i := 0; i < flen-1; i++ {
			e := gp.FreeList.Front()
			pc := e.Value.(*PoolConn[T])
			pc.Gc.Close()
			gp.FreeList.Remove(e)
		}*/
		if flen > 0 {
			return gp.getConnFromFreeList(ctx)
		}
	}
	return nil, fmt.Errorf("pool is full flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
}

func (pc *PoolConn[T]) put(ctx context.Context) error {
	pc.gp.mutex.Lock()
	defer pc.gp.mutex.Unlock()
	pc.gp.UseList.Remove(pc.e)
	pc.gp.FreeList.PushBack(pc)
	if pc.gp.FreeList.Len() > pc.gp.MaxIdleConns {
		flen := pc.gp.FreeList.Len() - pc.gp.MaxIdleConns
		for i := 0; i < flen; i++ {
			e := pc.gp.FreeList.Front()
			pc := e.Value.(*PoolConn[T])
			pc.Gc.Close()
			pc.gp.FreeList.Remove(e)
		}
	}
	logger.Debugf(ctx, "after put flist len %d ulist len %d", pc.gp.FreeList.Len(), pc.gp.UseList.Len())
	return nil
}

func (pc *PoolConn[T]) CloseWithErr(ctx context.Context, err error) {
	switch err.(type) {
	case thrift.TTransportException:
		tte := err.(thrift.TTransportException)
		e_type_id := tte.TypeId()
		logger.Warnf(ctx, "e id: %d", e_type_id)
		pc.Gc.Close()
	case thrift.TProtocolException:
		tpe := err.(thrift.TProtocolException)
		e_type_id := tpe.TypeId()
		logger.Warnf(ctx, "e id: %d", e_type_id)
		pc.Gc.Close()
	default:
		logger.Warnf(ctx, "e: %v", err)
	}
	pc.put(ctx)
}

func (pc *PoolConn[T]) Close(ctx context.Context) {
	pc.put(ctx)
}

func (gp *Gpool[T]) GetFreeLen() int {
	return gp.FreeList.Len()
}

func (gp *Gpool[T]) GetUseLen() int {
	return gp.UseList.Len()
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
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		address := fmt.Sprintf("%s:%d", tconn.Addr, tconn.Port)
		logger.Infof(ctx, "func=ThriftCallFramed|module=%s|method=%s|addr=%s:%d|time=%d|err=%s",
			c.Elem().Type().Name(), method, address, time.Duration(tconn.TimeOut)*1000, (endTime-starttime)/1000, errStr)
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
		logger.Warnf(ctx, "call %s %s", method, rpc_err.Error())
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

func (gp *Gpool[T]) ThriftCall2(ctx context.Context, process func(client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	pc, err := gp.Get(ctx)
	if pc != nil {
		defer pc.Close(ctx)
	}
	if err != nil {
		logger.Warnf(ctx, "get conn err: %s", err.Error())
		return err
	}
	tconn := pc.Gc.(*TConn[T])

	starttime := time.Now().UnixNano()
	defer func() {
		endTime := time.Now().UnixNano()
		errStr := ""
		if rpc_err != nil {
			errStr = rpc_err.Error()
		}
		address := fmt.Sprintf("%s:%d", tconn.Addr, tconn.Port)
		logger.Infof(ctx, "func=ThriftCall2|method=%v|addr=%s:%d|time=%d|err=%s",
			rpc_name, address, time.Duration(tconn.TimeOut), (endTime-starttime)/1000, errStr)
	}()

	if err != nil {
		logger.Warnf(ctx, "pool get conn err: %s", err.Error())
		return err
	}
	client := pc.Gc.GetThrfitClient()
	rpc_name, err = process(client)
	if err != nil {
		rpc_err = err
		logger.Warnf(ctx, "rpc ret %s", err.Error())
		switch err.(type) {
		case thrift.TTransportException:
			tte := err.(thrift.TTransportException)
			e_type_id := tte.TypeId()
			logger.Warnf(ctx, "e id: %d", e_type_id)
			pc.Gc.Close()
		case thrift.TProtocolException:
			tpe := err.(thrift.TProtocolException)
			e_type_id := tpe.TypeId()
			logger.Warnf(ctx, "e id: %d", e_type_id)
			pc.Gc.Close()
		default:
			logger.Warnf(ctx, "e: %v", err)
		}
		return err
	}
	return nil
}
