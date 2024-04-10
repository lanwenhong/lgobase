package gpool

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
)

type NewThriftClient[T any] func(thrift.TTransport, thrift.TProtocolFactory) *T

const (
	TH_PRO_FRAMED = iota
	TH_PRO_BUFFER
)

type TConn[T any] struct {
	Addr     string
	Port     int
	TimeOut  int
	Protocol int
	Tft      thrift.TTransport
	Tbt      *thrift.TBufferedTransport
	Tbp      *thrift.TBinaryProtocolFactory
	//isClose  bool
	isOpen bool

	Nc     NewThriftClient[T]
	Client *T
}

func (tc *TConn[T]) GetThrfitClient() *T {
	return tc.Client
}

func (tc *TConn[T]) Init(addr string, port int, timeout int) error {
	tc.Addr = addr
	tc.Port = port
	tc.TimeOut = timeout

	if tc.Protocol == TH_PRO_FRAMED {
		transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
		tc.Tbp = thrift.NewTBinaryProtocolFactoryDefault()
		addr := fmt.Sprintf("%s:%d", tc.Addr, tc.Port)
		//transport, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second, time.Duration(tc.TimeOut)*time.Second)
		transport, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second, time.Duration(tc.TimeOut)*time.Second)
		tc.Tft, _ = transportFactory.GetTransport(transport)
	} else if tc.Protocol == TH_PRO_BUFFER {
		addr := fmt.Sprintf("%s:%d", tc.Addr, tc.Port)
		socket, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Millisecond, time.Duration(tc.TimeOut)*time.Millisecond)
		tc.Tbt = thrift.NewTBufferedTransport(socket, 8192)
		tc.Tbp = thrift.NewTBinaryProtocolFactoryDefault()
	}
	return nil
}

func (tc *TConn[T]) Open() error {
	if tc.Protocol == TH_PRO_FRAMED {
		err := tc.Tft.Open()
		if err == nil {
			tc.isOpen = tc.Tft.IsOpen()
		}
		return err
	} else if tc.Protocol == TH_PRO_BUFFER {
		err := tc.Tbt.Open()
		if err == nil {
			tc.isOpen = tc.Tbt.IsOpen()
		}
		return err
	}
	return errors.New("not support")
}

func (tc *TConn[T]) NewThClient(nc NewThriftClient[T]) {
	tc.Nc = nc
	if tc.Protocol == TH_PRO_FRAMED {
		tc.Client = tc.Nc(tc.Tft, tc.Tbp)
	} else if tc.Protocol == TH_PRO_BUFFER {
		tc.Client = tc.Nc(tc.Tbt, tc.Tbp)
	}
}

func (tc *TConn[T]) Close() error {
	if tc.Protocol == TH_PRO_FRAMED {
		tc.Tft.Close()
		tc.isOpen = tc.Tft.IsOpen()
	} else if tc.Protocol == TH_PRO_BUFFER {
		tc.Tbt.Close()
		tc.isOpen = tc.Tbt.IsOpen()
	}
	return errors.New("not support")
}

func (tc *TConn[T]) IsOpen() bool {
	if tc.Protocol == TH_PRO_FRAMED {
		tc.isOpen = tc.Tft.IsOpen()
	} else if tc.Protocol == TH_PRO_BUFFER {
		tc.isOpen = tc.Tbt.IsOpen()
	}
	return tc.isOpen
}

func NewTConn[T any](addr string, port int, timeout int, protocol int) *TConn[T] {
	tc := &TConn[T]{}
	tc.Protocol = protocol
	tc.isOpen = false
	tc.Init(addr, port, timeout)
	return tc
}

func CreateThriftFramedConn[T any](ctx context.Context, addr string, port int, timeout int) (c Conn[T], err error) {
	conn := NewTConn[T](addr, port, timeout, TH_PRO_FRAMED)
	err = conn.Open()
	c = conn
	return c, err
}

func CreateThriftBufferConn[T any](ctx context.Context, addr string, port int, timeout int) (c Conn[T], err error) {
	logger.Debugf(ctx, "in CreateThriftBufferConn")
	conn := NewTConn[T](addr, port, timeout, TH_PRO_BUFFER)
	logger.Debugf(ctx, "conn created: %v", conn)
	err = conn.Open()
	logger.Debugf(ctx, "conn opened")
	c = conn
	return c, err
}

func ThriftCall[T any](ctx context.Context, pc *PoolConn[T], method string, arguments ...interface{}) (interface{}, error) {
	var err error
	err = nil
	//c := reflect.ValueOf(client)
	tconn := pc.Gc.(*TConn[T])
	c := reflect.ValueOf(tconn.Client)
	starttime := time.Now().UnixNano()
	//defer pc.Close(ctx, err)
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
		err = rets[0].(error)
	} else if rets[1] != nil {
		err = rets[1].(error)
	}
	if retlen == 1 {
		return nil, err
	} else {
		return rets[0], err
	}
}

func ThriftCall2[T any](ctx context.Context, pc *PoolConn[T], fn interface{}, params ...interface{}) (interface{}, error) {
	var err error
	err = nil

	tconn := pc.Gc.(*TConn[T])
	starttime := time.Now().UnixNano()
	//defer pc.Close(ctx, err)
	defer func() {
		endTime := time.Now().UnixNano()
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		address := fmt.Sprintf("%s:%d", tconn.Addr, tconn.Port)
		logger.Infof(ctx, "func=ThriftCall|method=%v|addr=%s:%d|time=%d|err=%s",
			fn, address, time.Duration(tconn.TimeOut)*1000, (endTime-starttime)/1000, errStr)
	}()

	v := reflect.ValueOf(fn)
	if len(params) != v.Type().NumIn()-1 {
		return nil, errors.New("The number of params is not adapted.")
		//return
	}
	//in := make([]reflect.Value, len(params))
	in := make([]reflect.Value, 0)
	//in := make([]reflect.Value, len(params)+1)
	in = append(in, reflect.ValueOf(ctx))
	for _, param := range params {
		//in[k] = reflect.ValueOf(param)
		in = append(in, reflect.ValueOf(param))
	}

	logger.Debug(ctx, "====%d", len(in))
	var rets []interface{}
	for _, arg := range v.Call(in) {
		rets = append(rets, arg.Interface())
	}

	retlen := len(rets)
	if retlen == 1 && rets[0] != nil {
		err = rets[0].(error)
	} else if rets[1] != nil {
		err = rets[1].(error)
	}
	if retlen == 1 {
		return nil, err
	} else {
		return rets[0], err
	}
}
