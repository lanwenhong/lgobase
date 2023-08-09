package gpool

import (
	"container/list"
	"fmt"
	"sync"

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

type CreateConn[T any] func(string, int, int) (Conn[T], error)

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

func (gp *Gpool[T]) getConnFromFreeList() (*PoolConn[T], error) {
	e := gp.FreeList.Front()
	logger.Debugf("after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	var reterr error
	pc := e.Value.(*PoolConn[T])
	if !pc.Gc.IsOpen() {
		reterr = pc.Gc.Open()
		if reterr != nil {
			logger.Warnf("open err %s", reterr.Error())
		}
	}
	gp.FreeList.Remove(e)
	e.Value.(*PoolConn[T]).e = gp.UseList.PushBack(e.Value)
	return e.Value.(*PoolConn[T]), reterr
}

func (gp *Gpool[T]) getConnFromNew() (*PoolConn[T], error) {
	var err error
	pc := new(PoolConn[T])
	pc.gp = gp
	logger.Debugf("----%v", gp.Cfunc)
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
			logger.Debugf("get conn from freelist")
			return gp.getConnFromFreeList()
		} else {
			logger.Debugf("get conn from new")
			return gp.getConnFromNew()
		}
	} else {
		logger.Debugf("pool full flist %d ulist %d", gp.FreeList.Len(), gp.UseList.Len())
		flen := gp.FreeList.Len()
		/*for i := 0; i < flen-1; i++ {
			e := gp.FreeList.Front()
			pc := e.Value.(*PoolConn[T])
			pc.Gc.Close()
			gp.FreeList.Remove(e)
		}*/
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

func (pc *PoolConn[T]) Close(err error) {
	switch err.(type) {
	case thrift.TTransportException:
		tte := err.(thrift.TTransportException)
		e_type_id := tte.TypeId()
		logger.Warnf("e id: %d", e_type_id)
	case thrift.TProtocolException:
		tpe := err.(thrift.TProtocolException)
		e_type_id := tpe.TypeId()
		logger.Warnf("e id: %d", e_type_id)
		pc.Gc.Close()
	}
	pc.put()
}

func (gp *Gpool[T]) GetFreeLen() int {
	return gp.FreeList.Len()
}

func (gp *Gpool[T]) GetUseLen() int {
	return gp.UseList.Len()
}
