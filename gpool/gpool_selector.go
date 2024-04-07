package gpool

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/selector"
)

type GPoolConfig[T any] struct {
	Addrs        string
	MaxConns     int
	MaxIdleConns int
	Cfunc        CreateConn[T]
	Nc           NewThriftClient[T]
}

type RpcSvr[T any] struct {
	selector.BaseSvr
	Gp *Gpool[T]
}

type RpcPoolSelector[T any] struct {
	selector.Selector
}

func (rps *RpcPoolSelector[T]) RoundRobin() interface{} {
	var item []interface{} = make([]interface{}, len(rps.Slist))
	var j int32 = 0
	for i := 0; i < len(rps.Slist); i++ {
		bs := rps.Slist[i].(RpcSvr[T])
		if bs.GetValid() == selector.SVR_VALID {
			item[i] = rps.Slist[i]
			j++
		}
	}
	if j == 0 {
		return nil
	}
	atomic_pos := atomic.LoadInt32(&rps.Pos)
	addr := item[atomic_pos%j]
	atomic_pos = (atomic_pos + 1) % (int32(len(rps.Slist)))
	atomic.StoreInt32(&rps.Pos, atomic_pos)

	return addr
}

func (rps *RpcPoolSelector[T]) RpcPoolInit(ctx context.Context, g_conf *GPoolConfig[T]) error {
	x := strings.Split(g_conf.Addrs, ",")
	rps.Slist = make([]selector.SvrAddr, len(x))
	for i := 0; i < len(x); i++ {
		xx := strings.Split(x[i], ":")
		if len(xx) != 2 {
			return errors.New("addr format error!!!")
		}
		port, timeout := rps.GetAddrPort(xx[1])
		iport, _ := strconv.Atoi(port)
		itimeout, _ := strconv.Atoi(timeout)
		//bs := NewSvr()
		rs := RpcSvr[T]{}
		rs.SetAddr(xx[0])
		rs.SetPort(iport)
		rs.SetTimeOut(itimeout)
		rs.SetStat(1)

		rs.Gp = &Gpool[T]{}
		rs.Gp.GpoolInit(xx[0], iport, itimeout, g_conf.MaxConns, g_conf.MaxIdleConns,
			g_conf.Cfunc, g_conf.Nc)
		rps.Slist[i] = rs
	}
	return nil
}

func (rps *RpcPoolSelector[T]) ThriftCall(ctx context.Context, process func(client interface{}) (string, error)) error {
	isvr := rps.RoundRobin()
	if isvr == nil {
		logger.Warnf(ctx, "no server select")
		return errors.New("no server")
	}
	rps_pool := isvr.(RpcSvr[T])
	logger.Debugf(ctx, "select add: %s port %d", rps_pool.GetAddr(), rps_pool.GetPort())
	return rps_pool.Gp.ThriftCall2(ctx, process)
}
