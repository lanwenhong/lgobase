package network

import (
	"container/list"
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/lanwenhong/lgobase/cas"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

/*type TcpConnInter interface {
	TcpSslConn | TcpConn
}*/

type TcpConnInter interface {
	Open(context.Context) error
	Close(context.Context)
	IsOpen(context.Context) bool
}

type CreateTcpConn[T TcpConnInter] func(context.Context, string, int, int, *tls.Config) TcpConnInter

type PoolTcpConn[T TcpConnInter] struct {
	Gc     TcpConnInter
	gp     *GTcpPool[T]
	e      *list.Element
	Ctime  int64
	Opened bool
}

type GTcpPool[T TcpConnInter] struct {
	FreeList     *list.List
	UseList      *list.List
	PurgeQueue   cas.Queue
	MaxConns     int
	MaxIdleConns int
	MaxConnLife  int64
	PurgeRate    float64
	hold         int
	mutex        sync.Mutex
	Cfunc        CreateTcpConn[T]
	Addr         string
	Port         int
	TimeOut      int
	Waits        uint
	WaitNotify   chan struct{}
	PurgeNotify  chan struct{}
	Gpconf       *TcpPoolConfig[T]
}

func NewSingleTcpConn[T TcpConnInter](ctx context.Context, ip string, port int, timeout int, tsl_conf *tls.Config) TcpConnInter {
	cTimeout := time.Duration(timeout) * time.Millisecond
	rTimeout := time.Duration(timeout) * time.Millisecond
	wTimeout := time.Duration(timeout) * time.Millisecond

	addr := fmt.Sprintf("%s:%d", ip, port)
	if tsl_conf == nil {
		c := NewTcpConn(addr, cTimeout, rTimeout, wTimeout)
		return c
	} else {
		c := NewTcpSslConn(addr, cTimeout, rTimeout, wTimeout, tsl_conf)
		return c

	}
	return nil
}

func (gp *GTcpPool[T]) GTcpPoolInit2(addr string, port int, timeout int, gp_conf *TcpPoolConfig[T]) {
	gp.FreeList = list.New()
	gp.UseList = list.New()
	gp.PurgeQueue = cas.CreateCasQueue()
	gp.MaxConns = gp_conf.MaxConns
	gp.MaxIdleConns = gp_conf.MaxIdleConns
	gp.MaxConnLife = gp_conf.MaxConnLife
	gp.PurgeRate = gp_conf.PurgeRate
	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.Cfunc = gp_conf.Cfunc
	gp.WaitNotify = make(chan struct{})
	gp.PurgeNotify = make(chan struct{}, 1)
	gp.Gpconf = gp_conf

	go func() {
		ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
		for {
			select {
			case <-gp.PurgeNotify:
				for {
					e, _ := gp.PurgeQueue.PopFront(ctx)
					if e != nil {
						logger.Debugf(ctx, "host %s:%d purge conn: %v", addr, port, e)
						pc := e.(*PoolTcpConn[T])
						pc.Gc.Close(ctx)
					} else {
						break
					}
				}
			}
		}
	}()
}

func (gp *GTcpPool[T]) getConnFromFreeList(ctx context.Context) (*PoolTcpConn[T], error) {
	e := gp.FreeList.Front()
	var reterr error
	pc := e.Value.(*PoolTcpConn[T])
	var isMaxConnLife bool = false
	connNow := time.Now().Unix()
	logger.Debugf(ctx, "connNow: %d pc.Ctime: %d MaxConnLife: %d", connNow, pc.Ctime, gp.MaxConnLife)
	if gp.MaxConnLife > 0 && connNow-pc.Ctime > gp.MaxConnLife {
		isMaxConnLife = true
	}

	if !pc.Gc.IsOpen(ctx) || isMaxConnLife {
		//logger.Warnf(ctx, "reopen conn")
		// reterr = pc.Gc.Open()
		// if reterr != nil {
		// 	logger.Warnf(ctx, "open err %s", reterr.Error())
		// }

		logger.Infof(ctx, "func=reopen|pc=%p|ctime=%d|connNow=%d|MaxConnLife=%d", pc, pc.Ctime, connNow, gp.MaxConnLife)
		pc.Gc.Close(ctx)
		gp.FreeList.Remove(e)
		connNew, err := gp.getConnFromNew(ctx)
		return connNew, err
	}
	gp.FreeList.Remove(e)
	e.Value.(*PoolTcpConn[T]).e = gp.UseList.PushBack(e.Value)
	logger.Debugf(ctx, "after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	logger.Infof(ctx, "func=getConnFromFreeList|pc=%p|ctime=%d|usedTime=%d", pc, pc.Ctime, int64(connNow)-int64(pc.Ctime))
	//return e.Value.(*PoolTcpConn[T]), reterr
	return pc, reterr
}

func (gp *GTcpPool[T]) getConnFromNew(ctx context.Context) (*PoolTcpConn[T], error) {
	var err error
	pc := new(PoolTcpConn[T])
	pc.gp = gp
	pc.Ctime = time.Now().Unix()
	logger.Debugf(ctx, "----%v", gp.Cfunc)
	//pc.Gc, err = gp.Cfunc(ctx, gp.Addr, gp.Port, gp.TimeOut)
	pc.Gc = gp.Cfunc(ctx, gp.Addr, gp.Port, gp.TimeOut, gp.Gpconf.TlsConf)
	err = pc.Gc.Open(ctx)
	if err == nil {
		pc.e = gp.UseList.PushBack(pc)
	} else {
		logger.Warnf(ctx, "new conn %s", err.Error())
		pc = nil
	}
	logger.Debugf(ctx, "after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	if pc != nil {
		logger.Infof(ctx, "func=getConnFromNew|pc=%p|ctime=%d|usedTime=0", pc, pc.Ctime)
	}
	return pc, err
}

func (gp *GTcpPool[T]) getConnFromWait(ctx context.Context) (*PoolTcpConn[T], error) {
	gp.Waits += 1
	for {
		gp.mutex.Unlock()
		logger.Debugf(ctx, "--------------------wait")
		<-gp.WaitNotify
		logger.Debugf(ctx, "--------------------try lock")
		gp.mutex.Lock()
		logger.Debugf(ctx, "--------------------locked")
		//gp.Waits -= 1
		logger.Debugf(ctx, "pool  flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
		flen := gp.FreeList.Len()
		if gp.FreeList.Len()+gp.UseList.Len() >= gp.MaxConns && flen <= 0 {
			logger.Debugf(ctx, "wait again")
			gp.Waits += 1
			continue
		}
		if gp.FreeList.Len()+gp.UseList.Len() < gp.MaxConns {
			if gp.FreeList.Len() > 0 {
				return gp.getConnFromFreeList(ctx)
			} else {
				return gp.getConnFromNew(ctx)
			}
		} else {
			if gp.FreeList.Len() > 0 {
				return gp.getConnFromFreeList(ctx)
			}
		}
	}
}

func (gp *GTcpPool[T]) Get(ctx context.Context) (*PoolTcpConn[T], error) {
	logger.Debugf(ctx, "pool full flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
	gp.mutex.Lock()
	logger.Debugf(ctx, "pool full flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
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
		if flen > 0 {
			return gp.getConnFromFreeList(ctx)
		} else {
			logger.Debugf(ctx, "wait conn")
			return gp.getConnFromWait(ctx)
		}
	}
	return nil, fmt.Errorf("pool is full flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
}

func (pc *PoolTcpConn[T]) put(ctx context.Context) error {
	pc.gp.mutex.Lock()
	//defer pc.gp.mutex.Unlock()
	pc.gp.UseList.Remove(pc.e)
	pc.gp.FreeList.PushBack(pc)
	flen := pc.gp.FreeList.Len() - pc.gp.MaxIdleConns
	if flen < 0 {
		flen = 0
	}
	fMaxConns := float64(pc.gp.MaxConns)
	fflen := float64(pc.gp.FreeList.Len())
	rate := fflen / fMaxConns

	logger.Debugf(ctx, "need purge len: %d waits: %d purge_rate: %f", flen, pc.gp.Waits, rate)
	if pc.gp.FreeList.Len() > pc.gp.MaxIdleConns && pc.gp.Waits == 0 && rate >= pc.gp.PurgeRate {
		//flen := pc.gp.FreeList.Len() - pc.gp.MaxIdleConns
		//logger.Debugf(ctx, "purge len: %d", flen)
		for i := 0; i < flen; i++ {
			e := pc.gp.FreeList.Front()
			pc := e.Value.(*PoolTcpConn[T])
			pc.Gc.Close(ctx)
			pc.gp.FreeList.Remove(e)
			pc.gp.PurgeQueue.PushBack(ctx, pc)
			//notify
			select {
			case pc.gp.PurgeNotify <- struct{}{}:
			default:
				continue
			}
		}
	}
	logger.Debugf(ctx, "after put flist len %d ulist len %d", pc.gp.FreeList.Len(), pc.gp.UseList.Len())
	if pc.gp.Waits > 0 {
		logger.Debugf(ctx, "notify waits: %d", pc.gp.Waits)
		pc.gp.Waits -= 1
		logger.Debugf(ctx, "notified waits: %d", pc.gp.Waits)
		pc.gp.mutex.Unlock()
		pc.gp.WaitNotify <- struct{}{}
	} else {
		pc.gp.mutex.Unlock()
	}
	return nil
}

func (pc *PoolTcpConn[T]) Close(ctx context.Context) {
	pc.put(ctx)
}

func (gp *GTcpPool[T]) GetFreeLen() int {
	return gp.FreeList.Len()
}

func (gp *GTcpPool[T]) GetUseLen() int {
	return gp.UseList.Len()
}

func (gp *GTcpPool[T]) Process(ctx context.Context, process func(client interface{}) (string, error)) error {
	var rpc_err error
	var rpc_name string = ""
	pc, err := gp.Get(ctx)
	if err != nil {
		logger.Warnf(ctx, "get conn err: %s", err.Error())
		return err
	}
	defer pc.Close(ctx)

	starttime := time.Now()
	defer func() {
		//endTime := time.Now().UnixNano()
		//endTime := time.Now()
		errStr := ""
		if rpc_err != nil {
			errStr = rpc_err.Error()
		}
		address := fmt.Sprintf("%s:%d", gp.Addr, gp.Port)
		logger.Infof(ctx, "func=TcpProcess|msg=%v|addr=%s:%d|time=%v|err=%s",
			rpc_name, address, time.Duration(gp.TimeOut), time.Since(starttime), errStr)
	}()
	rpc_name, err = process(pc.Gc)
	if err != nil {
		rpc_err = err
		logger.Warnf(ctx, "err: %s", err.Error())
		pc.Gc.Close(ctx)
	}
	return err
}
