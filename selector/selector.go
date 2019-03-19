package selector

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

const (
	SVR_HTTP = iota
	SVR_THRIFT
	SVR_TCP
	SVR_REDIS
)

const (
	SVR_VALID    = 1
	SVR_NOTVALID = 0
)

type SvrAddr interface {
	GetAddr() string
	GetPort() int
	GetValid() int32
	GetTimeOut() int32
	SetStat(int32)
	SetAddr(string)
	SetTimeOut(int)
	SetPort(int)
}

type BaseSvr struct {
	Port    int
	Addr    string
	TimeOut int
	Valid   int32
	Index   int
}

type Selector struct {
	Pos   int32
	Slist []SvrAddr
}

func (bs *BaseSvr) GetPort() int {
	return bs.Port
}

func (bs *BaseSvr) GetAddr() string {
	return bs.Addr
}

func (bs *BaseSvr) GetTimeOut() int32 {
	return int32(bs.TimeOut)
}

func (bs *BaseSvr) SetStat(state int32) {
	atomic.StoreInt32(&bs.Valid, state)
}

func (bs *BaseSvr) SetAddr(addr string) {
	bs.Addr = addr
}

func (bs *BaseSvr) SetPort(port int) {
	bs.Port = port
}
func (bs *BaseSvr) SetTimeOut(timeout int) {
	bs.TimeOut = timeout
}
func (bs *BaseSvr) GetValid() int32 {
	return atomic.LoadInt32(&bs.Valid)
}

func NewSvr() *BaseSvr {
	bs := new(BaseSvr)
	bs.Port = 0
	bs.Addr = ""
	bs.Valid = 0
	return bs
}

func NewSelector() *Selector {
	s := new(Selector)
	s.Pos = 0
	return s
}

func getAddrPort(x string) (string, string) {
	ret := strings.Split(x, "/")
	return ret[0], ret[1]
}

func (s *Selector) SparseAddr(pstr string) error {
	x := strings.Split(pstr, ",")
	//fmt.Println(x)
	s.Slist = make([]SvrAddr, len(x))
	for i := 0; i < len(x); i++ {
		xx := strings.Split(x[i], ":")
		if len(xx) != 2 {
			return errors.New("addr format error!!!")
		}
		port, timeout := getAddrPort(xx[1])
		iport, _ := strconv.Atoi(port)
		itimeout, _ := strconv.Atoi(timeout)
		bs := NewSvr()
		s.Slist[i] = bs
		s.Slist[i].SetAddr(xx[0])
		s.Slist[i].SetPort(iport)
		s.Slist[i].SetTimeOut(itimeout)
		s.Slist[i].SetStat(1)
	}
	return nil
}

func (s *Selector) SparseRedisAddr(pstr []string) error {
	s.Slist = make([]SvrAddr, len(pstr))
	for i := 0; i < len(pstr); i++ {
		fmt.Printf("url: %s\n", pstr[i])
		bs := NewSvr()
		s.Slist[i] = bs
		s.Slist[i].SetAddr(pstr[i])
		s.Slist[i].SetStat(1)
	}
	return nil
}

func (s *Selector) RoundRobin() SvrAddr {
	var item []SvrAddr = make([]SvrAddr, len(s.Slist))
	var j int32 = 0
	for i := 0; i < len(s.Slist); i++ {
		if s.Slist[i].GetValid() == SVR_VALID {
			item[i] = s.Slist[i]
			j++
		}
	}
	if j == 0 {
		return nil
	}
	atomic_pos := atomic.LoadInt32(&s.Pos)
	addr := item[atomic_pos%j]
	atomic_pos = (atomic_pos + 1) % (int32(len(s.Slist)))
	atomic.StoreInt32(&s.Pos, atomic_pos)

	return addr
}

func (s *Selector) SetSvrStat(i int, stat int32) {
	svr := s.Slist[i]
	svr.SetStat(stat)
}
