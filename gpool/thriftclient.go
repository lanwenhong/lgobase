package gpool

import (
	"errors"
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
    //"github.com/lanwenhong/lgobase/logger"
	"time"
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
	isOpen   bool
    
	Nc       NewThriftClient[T]
    Client   *T
}

func (tc *TConn[T])GetThrfitClient() *T {
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
		transport, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second, time.Duration(tc.TimeOut)*time.Second)
		tc.Tft, _ = transportFactory.GetTransport(transport)
	} else if tc.Protocol == TH_PRO_BUFFER {
		addr := fmt.Sprintf("%s:%d", tc.Addr, tc.Port)
		socket, _ := thrift.NewTSocketTimeout(addr, time.Duration(tc.TimeOut)*time.Second, time.Duration(tc.TimeOut)*time.Second)
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
			tc.isOpen = tc.Tft.IsOpen()
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
		tc.isOpen = tc.Tft.IsOpen()
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

func CreateThriftFramedConn[T any](addr string, port int, timeout int) (c Conn[T], err error) {
	conn := NewTConn[T](addr, port, timeout, TH_PRO_FRAMED)
	err = conn.Open()
	c = conn
	return c, err
}

func CreateThriftBufferConn[T any](addr string, port int, timeout int) (c Conn[T], err error) {
	conn := NewTConn[T](addr, port, timeout, TH_PRO_BUFFER)
	err = conn.Open()
	c = conn
	return c, err
}
