package gpool

import (
	"context"
	"crypto/tls"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/selector"
	"github.com/lanwenhong/lgobase/util"
)

type PingSvr func(client interface{}) (string, error)

type GPoolConfig[T any] struct {
	Addrs        string
	MaxConns     int
	MaxIdleConns int
	MaxConnLife  int64
	PurgeRate    float64
	Cfunc        CreateConn[T]
	Nc           NewThriftClient[T]
	Ping         PingSvr
	PingTicker   int64
	TlsConf      *tls.Config
}

type RpcSvr[T any] struct {
	selector.BaseSvr
	Gp *Gpool[T]
}

type RpcPoolSelector[T any] struct {
	selector.Selector
	NotValid chan *RpcSvr[T]
	Gpconf   *GPoolConfig[T]
}

func NewRpcPoolSelector[T any](ctx context.Context, conf *GPoolConfig[T]) *RpcPoolSelector[T] {
	if conf.MaxConns == 0 {
		conf.MaxConns = 200
	}
	if conf.MaxIdleConns == 0 {
		conf.MaxIdleConns = 100
	}
	rpcPool := &RpcPoolSelector[T]{}
	rpcPool.RpcPoolInit(ctx, conf)
	return rpcPool
}

func (rps *RpcPoolSelector[T]) RoundRobin(ctx context.Context) interface{} {
	//var item []interface{} = make([]interface{}, len(rps.Slist))
	var item []interface{}
	var j int32 = 0
	for i := 0; i < len(rps.Slist); i++ {
		bs := rps.Slist[i].(*RpcSvr[T])
		if bs.GetValid() == selector.SVR_VALID {
			//item[i] = rps.Slist[i]
			item = append(item, rps.Slist[i])
			j++
		}
	}
	if j == 0 {
		return nil
	}
	logger.Debugf(ctx, "item: %v j: %d itemn_len: %d", item, j, len(item))
	atomic_pos := atomic.LoadInt32(&rps.Pos)
	logger.Debugf(ctx, "=========atomic_pos: %d", atomic_pos)
	addr := item[atomic_pos%j]
	atomic_pos = (atomic_pos + 1) % (int32(len(rps.Slist)))
	atomic.StoreInt32(&rps.Pos, atomic_pos)

	return addr
}

func (rps *RpcPoolSelector[T]) RpcPoolInit(ctx context.Context, g_conf *GPoolConfig[T]) error {
	x := strings.Split(g_conf.Addrs, ",")
	rps.Slist = make([]selector.SvrAddr, len(x))
	rps.Gpconf = g_conf
	for i := 0; i < len(x); i++ {
		xx := strings.Split(x[i], ":")
		if len(xx) != 2 {
			return errors.New("addr format error!!!")
		}
		port, timeout := rps.GetAddrPort(xx[1])
		iport, _ := strconv.Atoi(port)
		itimeout, _ := strconv.Atoi(timeout)
		//bs := NewSvr()
		rs := &RpcSvr[T]{}
		rs.SetAddr(xx[0])
		rs.SetPort(iport)
		rs.SetTimeOut(itimeout)
		rs.SetStat(selector.SVR_VALID)

		rs.Gp = &Gpool[T]{}
		//rs.Gp.GpoolInit(xx[0], iport, itimeout, g_conf.MaxConns, g_conf.MaxIdleConns, g_conf.MaxConnLife,
		//g_conf.Cfunc, g_conf.Nc)
		rs.Gp.GpoolInit2(ctx, xx[0], iport, itimeout, g_conf)
		rps.Slist[i] = rs
	}
	rps.NotValid = make(chan *RpcSvr[T])
	if g_conf.Ping != nil {
		go func() {
			ticker := time.NewTicker(time.Duration(g_conf.PingTicker) * time.Second)
			//var ticker *time.Ticker = nil
			for {
				select {
				case rps_pool := <-rps.NotValid:
					err := rps_pool.Gp.ThriftCall2(ctx, g_conf.Ping)
					if err == nil {
						rps_pool.SetStat(selector.SVR_VALID)
						logger.Infof(ctx, "addr=%s:%d|state=recover", rps_pool.GetAddr(), rps_pool.GetPort())
					} else {
						/*if ticker == nil {
							ticker = time.NewTicker(time.Duration(g_conf.PingTicker) * time.Second)
						}*/
						ticker.Reset(time.Duration(g_conf.PingTicker) * time.Second)
						logger.Warnf(ctx, "ping err: %s", err.Error())
					}
				case <-ticker.C:
					logger.Debugf(ctx, "ping process")
					valids := 0
					for i := 0; i < len(rps.Slist); i++ {
						rps_pool := rps.Slist[i].(*RpcSvr[T])
						if rps_pool.GetValid() == selector.SVR_NOTVALID {
							err := rps_pool.Gp.ThriftCall2(ctx, g_conf.Ping)
							if err == nil {
								rps_pool.SetStat(selector.SVR_VALID)
								logger.Infof(ctx, "addr=%s:%d|state=recover", rps_pool.GetAddr(), rps_pool.GetPort())
							} else {
								logger.Warnf(ctx, "ping err: %s", err.Error())
							}

						} else {
							valids++
						}
					}
					if valids == len(rps.Slist) {
						ticker.Stop()
						//ticker = nil
					}
				}
			}
		}()
	}
	return nil
}

func (rps *RpcPoolSelector[T]) ThriftCall(ctx context.Context, process func(client interface{}) (string, error)) error {
	isvr := rps.RoundRobin(ctx)
	if isvr == nil {
		logger.Warnf(ctx, "no server select")
		return errors.New("no server")
	}
	rps_pool := isvr.(*RpcSvr[T])
	logger.Debugf(ctx, "select adds: %s port %d", rps_pool.GetAddr(), rps_pool.GetPort())
	//return rps_pool.Gp.ThriftCall2(ctx, process)
	err := rps_pool.Gp.ThriftCall2(ctx, process)
	if rps.Gpconf.Ping != nil && err != nil {

		switch err.(type) {
		case thrift.TTransportException, thrift.TProtocolException:
			logger.Warnf(ctx, "%s:%d down", rps_pool.GetAddr(), rps_pool.GetPort())
			rps_pool.SetStat(selector.SVR_NOTVALID)
			rps.NotValid <- rps_pool
		default:
			logger.Warnf(ctx, "rpc: %s", err.Error())
		}
	}
	return err
}

func (rps *RpcPoolSelector[T]) ThriftExtCall(ctx context.Context, process func(ctx context.Context, client interface{}) (string, error)) error {
	var nCtx context.Context
	if eCtx, ok := ctx.(*ExtContext); ok {
		rid := eCtx.GetReqExtData("request_id")
		if rid == "" {
			eCtx = eCtx.SetReqExtData(eCtx, "request_id", util.NewRequestID())
		}
		nCtx = eCtx
	} else {
		logger.Warnf(ctx, "ctx format error")
		return errors.New("ctx format error")
	}
	isvr := rps.RoundRobin(nCtx)
	if isvr == nil {
		logger.Warnf(nCtx, "no server select")
		return errors.New("no server")
	}
	rps_pool := isvr.(*RpcSvr[T])
	logger.Debugf(nCtx, "select adds: %s port %d", rps_pool.GetAddr(), rps_pool.GetPort())
	err := rps_pool.Gp.ThriftExtCall2(nCtx, process)
	if rps.Gpconf.Ping != nil && err != nil {

		switch err.(type) {
		case thrift.TTransportException, thrift.TProtocolException:
			logger.Warnf(nCtx, "%s:%d down", rps_pool.GetAddr(), rps_pool.GetPort())
			rps_pool.SetStat(selector.SVR_NOTVALID)
			rps.NotValid <- rps_pool
		default:
			logger.Warnf(nCtx, "rpc: %s", err.Error())
		}
	}
	return err

}
