package gpool

import (
	"container/list"
	"fmt"
	"github.com/lanwenhong/lgobase/logger"
	"sync"
)

type Conn[T any] interface {
	Init(host string, port int, timeout int) error
	Open() error
	Close() error
	IsClosed() bool
    GetThrfitClient() *T
}

type CreateConn[T any] func(string, int, int) (Conn[T], error)

type PoolConn[T any] struct {
	Gc Conn[T]
    //Tc *TConn[T]
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

func (gp *Gpool[T])GpoolInit(addr string, port int, timeout int, maxconns int, maxidleconns int, 
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


func (gp *Gpool[T]) getConnFromFreeList() (*PoolConn[T], error) {
	e := gp.FreeList.Front()
	logger.Debugf("after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	pc := e.Value.(*PoolConn[T])
	if pc.Gc.IsClosed() {
		err := pc.Gc.Open()
		if err != nil {
			logger.Warnf("open err %s", err.Error())
			return nil, err
		}
	}
	gp.FreeList.Remove(e)
	e.Value.(*PoolConn[T]).e = gp.UseList.PushBack(e.Value)
	return e.Value.(*PoolConn[T]), nil
}

func (gp *Gpool[T]) getConnFromNew() (*PoolConn[T], error) {
	var err error
	pc := new(PoolConn[T])
	pc.gp = gp
	pc.Gc, err = gp.Cfunc(gp.Addr, gp.Port, gp.TimeOut)
	if err == nil {
		switch pc.Gc.(type) {
		case *TConn[T]:
			if gp.Nc != nil {
				pc.Gc.(*TConn[T]).NewThClient(gp.Nc)
			}
		}
		pc.e = gp.UseList.PushBack(pc)
	} else {
		logger.Warnf("new conn %s", err.Error())
		pc = nil
	}
	logger.Debugf("after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	return pc, err
}

func (gp *Gpool[T]) Get() (*PoolConn[T], error) {
	gp.mutex.Lock()
	defer gp.mutex.Unlock()
	if gp.FreeList.Len()+gp.UseList.Len() < gp.MaxConns {
		if gp.FreeList.Len() > 0 {
			return gp.getConnFromFreeList()
		} else {
			return gp.getConnFromNew()
		}
	} else {
		logger.Debugf("pool full flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
		flen := gp.FreeList.Len()
		for i := 0; i < flen-1; i++ {
			e := gp.FreeList.Front()
			pc := e.Value.(*PoolConn[T])
			pc.Gc.Close()
			gp.FreeList.Remove(e)
		}
		if flen > 0 {
			return gp.getConnFromFreeList()
		}
	}
	return nil, fmt.Errorf("pool is full flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
}

func (pc *PoolConn[T]) put() error {
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
	logger.Debugf("after put flist len %d ulist len %d", pc.gp.FreeList.Len(), pc.gp.UseList.Len())
	return nil
}

func (pc *PoolConn[T]) Close() error {
	return pc.put()
}
