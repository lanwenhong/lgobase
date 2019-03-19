package gpool

import (
	"container/list"
	"fmt"
	"github.com/lanwenhong/lgobase/logger"
	"sync"
)

type Conn interface {
	Init(host string, port int, timeout int) error
	Open() error
	Close() error
	IsClosed() bool
}

type CreateConn func(string, int, int) (Conn, error)

type PoolConn struct {
	Gc Conn
	gp *Gpool
	e  *list.Element
}

type Gpool struct {
	FreeList     *list.List
	UseList      *list.List
	MaxConns     int
	MaxIdleConns int
	hold         int
	mutex        sync.Mutex
	Cfunc        CreateConn
	//for thrift
	Nc      NewThriftClient
	Addr    string
	Port    int
	TimeOut int
}

func NewGpool(addr string, port int, timeout int, maxconns int, maxidleconns int, cfunc CreateConn) *Gpool {
	gp := new(Gpool)
	gp.FreeList = list.New()
	gp.UseList = list.New()
	gp.MaxConns = maxconns
	gp.MaxIdleConns = maxidleconns
	gp.Addr = addr
	gp.Port = port
	gp.TimeOut = timeout
	gp.Cfunc = cfunc
	gp.Nc = nil
	return gp
}

func (gp *Gpool) getConnFromFreeList() (*PoolConn, error) {
	e := gp.FreeList.Front()
	logger.Debugf("after get flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
	pc := e.Value.(*PoolConn)
	if pc.Gc.IsClosed() {
		err := pc.Gc.Open()
		if err != nil {
			logger.Warnf("open err %s", err.Error())
			return nil, err
		}
	}
	gp.FreeList.Remove(e)
	e.Value.(*PoolConn).e = gp.UseList.PushBack(e.Value)
	return e.Value.(*PoolConn), nil
}

func (gp *Gpool) getConnFromNew() (*PoolConn, error) {
	var err error
	pc := new(PoolConn)
	pc.gp = gp
	pc.Gc, err = gp.Cfunc(gp.Addr, gp.Port, gp.TimeOut)
	if err == nil {
		switch pc.Gc.(type) {
		case *TConn:
			if gp.Nc != nil {
				pc.Gc.(*TConn).NewThClient(gp.Nc)
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

func (gp *Gpool) Get() (*PoolConn, error) {
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
		len := gp.FreeList.Len()
		for i := 0; i < len-1; i++ {
			e := gp.FreeList.Front()
			pc := e.Value.(*PoolConn)
			pc.Gc.Close()
			gp.FreeList.Remove(e)
		}
		if len > 0 {
			return gp.getConnFromFreeList()
		}
	}
	return nil, fmt.Errorf("pool is full flist len %d ulist len %d", gp.FreeList.Len(), gp.UseList.Len())
}

func (pc *PoolConn) put() error {
	pc.gp.mutex.Lock()
	defer pc.gp.mutex.Unlock()
	pc.gp.UseList.Remove(pc.e)
	pc.gp.FreeList.PushBack(pc)
	if pc.gp.FreeList.Len() > pc.gp.MaxIdleConns {
		len := pc.gp.FreeList.Len() - pc.gp.MaxIdleConns
		for i := 0; i < len; i++ {
			e := pc.gp.FreeList.Front()
			pc := e.Value.(*PoolConn)
			pc.Gc.Close()
			pc.gp.FreeList.Remove(e)
		}
	}
	logger.Debugf("after put flist len %d ulist len %d", pc.gp.FreeList.Len(), pc.gp.UseList.Len())
	return nil
}

func (pc *PoolConn) Close() error {
	return pc.put()
}

//for thrift set newclient
func (gc *Gpool) SetNewThriftClent(nc NewThriftClient) {
	gc.Nc = nc
}
