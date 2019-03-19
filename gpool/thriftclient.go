package gpool

import (
	"errors"
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lgobase/logger"
	"reflect"
	"time"
)

type NewThriftClient func(thrift.TTransport, thrift.TProtocolFactory) interface{}

const (
	TH_PRO_FRAMED = iota
	TH_PRO_BUFFER
)

type TConn struct {
	Addr     string
	Port     int
	TimeOut  int
	Protocol int
	Tft      thrift.TTransport
	Tbt      *thrift.TBufferedTransport
	Tbp      *thrift.TBinaryProtocolFactory
	isClose  bool
	Nc       NewThriftClient
	Client   interface{}
}

func (tc *TConn) Init(addr string, port int, timeout int) error {
	tc.Addr = addr
	tc.Port = port
	tc.TimeOut = timeout

	if tc.Protocol == TH_PRO_FRAMED {
		transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
		tc.Tbp = thrift.NewTBinaryProtocolFactoryDefault()
		addr := fmt.Sprintf("%s:%d", tc.Addr, tc.Port)
		transport, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second)
		tc.Tft = transportFactory.GetTransport(transport)
		//tc.Client = tc.Nc(tc.Tft, tc.Tbp)
	} else if tc.Protocol == TH_PRO_BUFFER {
		addr := fmt.Sprintf("%s:%d", tc.Addr, tc.Port)
		socket, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second)
		tc.Tbt = thrift.NewTBufferedTransport(socket, 8192)
		tc.Tbp = thrift.NewTBinaryProtocolFactoryDefault()
		//tc.Client = tc.Nc(tc.Tbt, tc.Tbp)
	}
	return nil
}

func (tc *TConn) Open() error {
	if tc.Protocol == TH_PRO_FRAMED {
		//tc.Client = tc.Nc(tc.Tft, tc.Tbp)
		err := tc.Tft.Open()
		if err == nil {
			tc.isClose = false
		}
		return err
	} else if tc.Protocol == TH_PRO_BUFFER {
		//tc.Client = tc.Nc(tc.Tbt, tc.Tbp)
		err := tc.Tbt.Open()
		if err == nil {
			tc.isClose = false
		}
		return err
	}
	return errors.New("not support")
}

func (tc *TConn) NewThClient(nc NewThriftClient) {
	tc.Nc = nc
	if tc.Protocol == TH_PRO_FRAMED {
		tc.Client = tc.Nc(tc.Tft, tc.Tbp)
	} else if tc.Protocol == TH_PRO_BUFFER {
		tc.Client = tc.Nc(tc.Tbt, tc.Tbp)
	}
}

func (tc *TConn) Close() error {
	if tc.Protocol == TH_PRO_FRAMED {
		tc.Tft.Close()
		tc.isClose = true
	} else if tc.Protocol == TH_PRO_BUFFER {
		tc.Tbt.Close()
		tc.isClose = true
	}
	return errors.New("not support")
}

func (tc *TConn) IsClosed() bool {
	return tc.isClose
}

func NewTConn(addr string, port int, timeout int, protocol int) *TConn {
	tc := &TConn{}
	tc.Protocol = protocol
	tc.isClose = true
	tc.Init(addr, port, timeout)
	return tc
}

func CreateThriftFramedConn(addr string, port int, timeout int) (c Conn, err error) {
	conn := NewTConn(addr, port, timeout, TH_PRO_FRAMED)
	err = conn.Open()
	c = conn
	return c, err
}

func CreateThriftBufferConn(addr string, port int, timeout int) (c Conn, err error) {
	conn := NewTConn(addr, port, timeout, TH_PRO_BUFFER)
	err = conn.Open()
	c = conn
	return c, err
}

//func ThriftCall(pc *PoolConn, client interface{}, method string, arguments ...interface{}) (interface{}, error) {
func ThriftCall(pc *PoolConn, method string, arguments ...interface{}) (interface{}, error) {
	var err error
	err = nil
	//c := reflect.ValueOf(client)
	tconn := pc.Gc.(*TConn)
	c := reflect.ValueOf(tconn.Client)
	starttime := time.Now().UnixNano()
	defer func() {
		endTime := time.Now().UnixNano()
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		address := fmt.Sprintf("%s:%d", tconn.Addr, tconn.Port)
		logger.Infof("func=ThriftCallFramed|module=%s|method=%s|addr=%s/%d|time=%d|err=%s",
			c.Elem().Type().Name(), method, address, time.Duration(tconn.TimeOut)/time.Millisecond, (endTime-starttime)/1000, errStr)
	}()

	if tconn.Client == nil {
		if tconn.Protocol == TH_PRO_FRAMED {
			c.Elem().FieldByName("Transport").Set(reflect.ValueOf(tconn.Tft))
			c.Elem().FieldByName("ProtocolFactory").Set(reflect.ValueOf(tconn.Tbp))
			c.Elem().FieldByName("InputProtocol").Set(reflect.ValueOf(tconn.Tbp.GetProtocol(tconn.Tft)))
			c.Elem().FieldByName("OutputProtocol").Set(reflect.ValueOf(tconn.Tbp.GetProtocol(tconn.Tft)))
			c.Elem().FieldByName("SeqId").SetInt(0)
		} else {
			c.Elem().FieldByName("Transport").Set(reflect.ValueOf(tconn.Tbt))
			c.Elem().FieldByName("ProtocolFactory").Set(reflect.ValueOf(tconn.Tbp))
			c.Elem().FieldByName("InputProtocol").Set(reflect.ValueOf(tconn.Tbp.GetProtocol(tconn.Tbt)))
			c.Elem().FieldByName("OutputProtocol").Set(reflect.ValueOf(tconn.Tbp.GetProtocol(tconn.Tbt)))
			c.Elem().FieldByName("SeqId").SetInt(0)
		}
	}

	function := c.MethodByName(method)
	if !function.IsValid() || function.IsNil() {
		return nil, errors.New("method not found")
	}

	if need := function.Type().NumIn(); need != len(arguments) {
		return nil, errors.New(fmt.Sprintf("arguments number not match, need %d but got %d", need, len(arguments)))
	}

	callArgs := make([]reflect.Value, len(arguments))
	for i, arg := range arguments {
		callArgs[i] = reflect.ValueOf(arg)
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
	if err != nil {
		logger.Warnf("call %s %s", method, err.Error())
		switch err.(type) {
		case thrift.TTransportException:
		case thrift.TProtocolException:
			pc.Gc.Close()
		}
	}
	if retlen == 1 {
		return nil, err
	} else {
		return rets[0], err
	}
}

func ThriftCall2(pc *PoolConn, fn interface{}, params ...interface{}) (interface{}, error) {
	var err error
	err = nil

	tconn := pc.Gc.(*TConn)
	starttime := time.Now().UnixNano()
	defer func() {
		endTime := time.Now().UnixNano()
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		address := fmt.Sprintf("%s:%d", tconn.Addr, tconn.Port)
		logger.Infof("func=ThriftCall|method=%v|addr=%s/%d|time=%d|err=%s",
			fn, address, time.Duration(tconn.TimeOut)/time.Millisecond, (endTime-starttime)/1000, errStr)
	}()

	v := reflect.ValueOf(fn)
	if len(params) != v.Type().NumIn() {
		return nil, errors.New("The number of params is not adapted.")
		//return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}

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
	if err != nil {
		logger.Warnf("call %v %s", fn, err.Error())
		switch err.(type) {
		case thrift.TTransportException:
		case thrift.TProtocolException:
			pc.Gc.Close()
		}
	}
	if retlen == 1 {
		return nil, err
	} else {
		return rets[0], err
	}
}
