package network

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/selector"
)

const defaultTcpPingInterval = 5 * time.Second

type TcpPingSvr func(client interface{}) (string, error)

type TcpPoolConfig[T TcpConnInter] struct {
	Addrs           string
	MaxConns        int
	MaxIdleConns    int
	MaxWaiters      int
	MaxConnLife     int64
	MaxIdleConnLife int64
	PurgeRate       float64
	Cfunc           CreateTcpConn[T]
	Ping            TcpPingSvr
	PingTicker      int64
	TlsConf         *tls.Config
}

type TcpRpcSvr[T TcpConnInter] struct {
	selector.BaseSvr
	Gp *GTcpPool[T]
}

type TcpPoolSelector[T TcpConnInter] struct {
	selector.Selector
	NotValid chan *TcpRpcSvr[T]
	Gpconf   *TcpPoolConfig[T]

	closed       atomic.Bool
	lifecycleMu  sync.Mutex
	healthCancel context.CancelFunc
	healthDone   chan struct{}
}

type tcpPoolEndpoint struct {
	addr    string
	port    int
	timeout int
}

func NewTcpPoolSelector[T TcpConnInter](ctx context.Context, conf *TcpPoolConfig[T]) (*TcpPoolSelector[T], error) {
	poolSelector := &TcpPoolSelector[T]{}
	if err := poolSelector.RpcPoolInit(ctx, conf); err != nil {
		return nil, err
	}
	return poolSelector, nil
}

// RoundRobin returns one valid server pool. A single atomic ticket is shared by
// concurrent callers; selection is performed against the current valid-node
// set, so disabled nodes neither receive traffic nor bias the distribution.
func (rps *TcpPoolSelector[T]) RoundRobin(ctx context.Context) interface{} {
	if rps.closed.Load() {
		return nil
	}

	// A validity transition may occur between the counting and selection passes.
	// Retry once in that rare case instead of allocating a candidates slice on
	// every request.
	for retry := 0; retry < 2; retry++ {
		validCount := 0
		for _, item := range rps.Slist {
			server, ok := item.(*TcpRpcSvr[T])
			if ok && server.GetValid() == selector.SVR_VALID {
				validCount++
			}
		}
		if validCount == 0 {
			return nil
		}

		ticket := atomic.AddInt32(&rps.Pos, 1) - 1
		target := int(uint32(ticket) % uint32(validCount))
		for _, item := range rps.Slist {
			server, ok := item.(*TcpRpcSvr[T])
			if !ok || server.GetValid() != selector.SVR_VALID {
				continue
			}
			if target == 0 {
				return server
			}
			target--
		}
	}
	return nil
}

func (rps *TcpPoolSelector[T]) RpcPoolInit(ctx context.Context, conf *TcpPoolConfig[T]) error {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoints, err := parseTcpPoolEndpoints(conf)
	if err != nil {
		return err
	}

	rps.closed.Store(false)
	rps.Pos = 0
	rps.Slist = make([]selector.SvrAddr, len(endpoints))
	rps.Gpconf = conf
	rps.NotValid = make(chan *TcpRpcSvr[T], len(endpoints))

	for i, endpoint := range endpoints {
		server := &TcpRpcSvr[T]{}
		server.SetAddr(endpoint.addr)
		server.SetPort(endpoint.port)
		server.SetTimeOut(endpoint.timeout)
		server.SetStat(selector.SVR_VALID)
		server.Gp = &GTcpPool[T]{}
		server.Gp.GTcpPoolInitWithContext(ctx, endpoint.addr, endpoint.port, endpoint.timeout, conf)
		rps.Slist[i] = server
	}

	if conf.Ping != nil {
		rps.startHealthChecker(ctx)
	}
	return nil
}

func parseTcpPoolEndpoints[T TcpConnInter](conf *TcpPoolConfig[T]) ([]tcpPoolEndpoint, error) {
	if conf == nil {
		return nil, errors.New("TCP pool config is nil")
	}
	if conf.Cfunc == nil {
		return nil, errors.New("TCP connection factory is nil")
	}
	if conf.MaxConns <= 0 {
		return nil, errors.New("TCP pool MaxConns must be positive")
	}
	if conf.MaxIdleConns < 0 || conf.MaxIdleConns > conf.MaxConns {
		return nil, errors.New("TCP pool MaxIdleConns must be between zero and MaxConns")
	}

	rawEndpoints := strings.Split(conf.Addrs, ",")
	endpoints := make([]tcpPoolEndpoint, 0, len(rawEndpoints))
	for _, rawEndpoint := range rawEndpoints {
		rawEndpoint = strings.TrimSpace(rawEndpoint)
		colon := strings.LastIndex(rawEndpoint, ":")
		if colon <= 0 || colon == len(rawEndpoint)-1 {
			return nil, fmt.Errorf("invalid TCP pool address %q: expected host:port/timeout_ms", rawEndpoint)
		}
		addr := strings.TrimSpace(rawEndpoint[:colon])
		portAndTimeout := strings.Split(rawEndpoint[colon+1:], "/")
		if addr == "" || len(portAndTimeout) != 2 {
			return nil, fmt.Errorf("invalid TCP pool address %q: expected host:port/timeout_ms", rawEndpoint)
		}
		port, err := strconv.Atoi(strings.TrimSpace(portAndTimeout[0]))
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid TCP pool port in %q", rawEndpoint)
		}
		timeout, err := strconv.Atoi(strings.TrimSpace(portAndTimeout[1]))
		if err != nil || timeout <= 0 {
			return nil, fmt.Errorf("invalid TCP pool timeout in %q", rawEndpoint)
		}
		endpoints = append(endpoints, tcpPoolEndpoint{addr: addr, port: port, timeout: timeout})
	}
	if len(endpoints) == 0 {
		return nil, errors.New("TCP pool has no endpoints")
	}
	return endpoints, nil
}

func (rps *TcpPoolSelector[T]) Process(ctx context.Context, process func(client interface{}) (string, error)) error {
	selected := rps.RoundRobin(ctx)
	if selected == nil {
		if rps.closed.Load() {
			return ErrTcpPoolClosed
		}
		logger.Warn(ctx, "select TCP server failed", "reason", "no_valid_server")
		return errors.New("no valid TCP server")
	}
	server := selected.(*TcpRpcSvr[T])
	err := server.Gp.Process(ctx, process)
	if err != nil {
		logger.Warn(ctx, "TCP RPC call failed", "addr", server.GetAddr(), "port", server.GetPort(), "err", err)
		// Without a health callback there is no safe recovery mechanism, so keep
		// the node selectable. When Ping is configured, quarantine it immediately
		// and let the health checker decide when it can receive traffic again.
		if rps.Gpconf != nil && rps.Gpconf.Ping != nil {
			server.SetStat(selector.SVR_NOTVALID)
			select {
			case rps.NotValid <- server:
			default:
			}
		}
	}
	return err
}

func (rps *TcpPoolSelector[T]) startHealthChecker(parent context.Context) {
	rps.lifecycleMu.Lock()
	defer rps.lifecycleMu.Unlock()
	healthCtx, cancel := context.WithCancel(context.WithoutCancel(parent))
	rps.healthCancel = cancel
	rps.healthDone = make(chan struct{})
	done := rps.healthDone
	go rps.runHealthChecker(healthCtx, done)
}

func (rps *TcpPoolSelector[T]) runHealthChecker(ctx context.Context, done chan struct{}) {
	defer close(done)
	interval := defaultTcpPingInterval
	if rps.Gpconf.PingTicker > 0 {
		interval = time.Duration(rps.Gpconf.PingTicker) * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case server := <-rps.NotValid:
			rps.checkServerHealth(ctx, server)
		case <-ticker.C:
			for _, item := range rps.Slist {
				server := item.(*TcpRpcSvr[T])
				if server.GetValid() == selector.SVR_NOTVALID {
					rps.checkServerHealth(ctx, server)
				}
			}
		}
	}
}

func (rps *TcpPoolSelector[T]) checkServerHealth(ctx context.Context, server *TcpRpcSvr[T]) {
	if server == nil || server.GetValid() == selector.SVR_VALID || rps.closed.Load() {
		return
	}
	timeout := time.Duration(server.GetTimeOut()) * time.Millisecond
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	err := server.Gp.Process(pingCtx, rps.Gpconf.Ping)
	cancel()
	if err != nil {
		logger.Warn(ctx, "TCP server health check failed", "addr", server.GetAddr(), "port", server.GetPort(), "err", err)
		return
	}
	server.SetStat(selector.SVR_VALID)
	logger.Info(ctx, "TCP server recovered", "addr", server.GetAddr(), "port", server.GetPort())
}

// Close stops health checks and closes every per-endpoint connection pool. It
// may be called again with a fresh context after an earlier timeout.
func (rps *TcpPoolSelector[T]) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	rps.closed.Store(true)

	rps.lifecycleMu.Lock()
	cancel := rps.healthCancel
	done := rps.healthDone
	rps.healthCancel = nil
	rps.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}

	errCh := make(chan error, len(rps.Slist))
	var wg sync.WaitGroup
	for _, item := range rps.Slist {
		server, ok := item.(*TcpRpcSvr[T])
		if !ok || server.Gp == nil {
			continue
		}
		wg.Add(1)
		go func(pool *GTcpPool[T]) {
			defer wg.Done()
			if err := pool.Close(ctx); err != nil {
				errCh <- err
			}
		}(server.Gp)
	}
	poolsDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(poolsDone)
	}()

	// Start child-pool shutdown before waiting for the health loop. A user Ping
	// callback may ignore cancellation; that must not prevent pool shutdown from
	// being initiated.
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case <-poolsDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	close(errCh)
	var closeErr error
	for err := range errCh {
		closeErr = errors.Join(closeErr, err)
	}
	return closeErr
}
