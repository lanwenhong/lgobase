package gpool

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

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/selector"
	"github.com/lanwenhong/lgobase/util"
)

const defaultRPCPingInterval = 5 * time.Second

// Keep the historical error text for callers that still compare Error(). New
// code can use errors.Is(err, ErrNoValidRPCServer).
var ErrNoValidRPCServer = errors.New("no server")

type PingSvr func(client interface{}) (string, error)

type GPoolConfig[T any] struct {
	Addrs           string
	MaxConns        int
	MaxIdleConns    int
	MaxWaiters      int
	MaxConnLife     int64
	MaxIdleConnLife int64
	// PurgeRate is retained for compatibility; Gpool's idle ring treats
	// MaxIdleConns as a hard limit.
	PurgeRate  float64
	Cfunc      CreateConn[T]
	Nc         NewThriftClient[T]
	Ping       PingSvr
	PingTicker int64
	TlsConf    *tls.Config
}

type RpcSvr[T any] struct {
	selector.BaseSvr
	Gp *Gpool[T]
}

type RpcPoolSelector[T any] struct {
	selector.Selector
	NotValid chan *RpcSvr[T]
	Gpconf   *GPoolConfig[T]

	closed       atomic.Bool
	initErr      error
	lifecycleMu  sync.Mutex
	healthCancel context.CancelFunc
	healthDone   chan struct{}
	ejectMu      sync.RWMutex
	ejectPolicy  func(error) bool
}

type rpcPoolEndpoint struct {
	addr    string
	port    int
	timeout int
}

// NewRpcPoolSelector retains the original constructor signature. New code that
// needs to handle invalid configuration immediately should use
// NewRpcPoolSelectorWithError.
func NewRpcPoolSelector[T any](ctx context.Context, conf *GPoolConfig[T]) *RpcPoolSelector[T] {
	if ctx == nil {
		ctx = context.Background()
	}
	rpcPool, err := NewRpcPoolSelectorWithError(ctx, conf)
	if err != nil {
		logger.Warn(ctx, "initialize RPC pool selector failed", "err", err)
	}
	return rpcPool
}

func NewRpcPoolSelectorWithError[T any](ctx context.Context, conf *GPoolConfig[T]) (*RpcPoolSelector[T], error) {
	rpcPool := &RpcPoolSelector[T]{}
	applyRPCPoolConfigDefaults(conf)
	if err := rpcPool.RpcPoolInit(ctx, conf); err != nil {
		return rpcPool, err
	}
	return rpcPool, nil
}

func applyRPCPoolConfigDefaults[T any](conf *GPoolConfig[T]) {
	if conf == nil {
		return
	}
	if conf.MaxConns == 0 {
		conf.MaxConns = 200
	}
	if conf.MaxIdleConns == 0 {
		// Preserve the constructor's historical visible config value. Each child
		// Gpool still clamps its effective idle capacity to MaxConns.
		conf.MaxIdleConns = 100
	}
}

// RoundRobin returns one currently valid endpoint pool. It performs no heap
// allocation and uses one shared atomic ticket for concurrent callers.
func (rps *RpcPoolSelector[T]) RoundRobin(_ context.Context) interface{} {
	if rps.closed.Load() || rps.initErr != nil {
		return nil
	}

	// A validity transition can occur between the two scans. Retry once rather
	// than allocating a candidate slice on every RPC.
	for retry := 0; retry < 2; retry++ {
		validCount := 0
		for _, item := range rps.Slist {
			server, ok := item.(*RpcSvr[T])
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
			server, ok := item.(*RpcSvr[T])
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

func (rps *RpcPoolSelector[T]) RpcPoolInit(ctx context.Context, conf *GPoolConfig[T]) error {
	if ctx == nil {
		ctx = context.Background()
	}

	endpoints, err := normalizeRPCPoolConfig(conf)
	if err != nil {
		rps.initErr = err
		return err
	}
	if len(rps.Slist) != 0 || rps.healthDone != nil {
		// RpcPoolInit historically allowed reusing the same selector. Preserve that
		// call pattern while closing the previously owned health loop and pools
		// instead of leaking them.
		if err := rps.Close(ctx); err != nil {
			return err
		}
		rps.lifecycleMu.Lock()
		rps.healthCancel = nil
		rps.healthDone = nil
		rps.lifecycleMu.Unlock()
		rps.Slist = nil
		rps.NotValid = nil
		rps.Gpconf = nil
	}

	rps.closed.Store(false)
	rps.initErr = nil
	atomic.StoreInt32(&rps.Pos, 0)
	rps.Slist = make([]selector.SvrAddr, len(endpoints))
	rps.Gpconf = conf
	rps.NotValid = make(chan *RpcSvr[T], len(endpoints))

	for i, endpoint := range endpoints {
		server := &RpcSvr[T]{}
		server.SetAddr(endpoint.addr)
		server.SetPort(endpoint.port)
		server.SetTimeOut(endpoint.timeout)
		server.SetStat(selector.SVR_VALID)
		server.Gp = &Gpool[T]{}
		server.Gp.GpoolInit2(ctx, endpoint.addr, endpoint.port, endpoint.timeout, conf)
		rps.Slist[i] = server
	}

	if conf.Ping != nil {
		rps.startHealthChecker(ctx)
	}
	return nil
}

func normalizeRPCPoolConfig[T any](conf *GPoolConfig[T]) ([]rpcPoolEndpoint, error) {
	if conf == nil {
		return nil, errors.New("RPC pool config is nil")
	}
	if conf.Cfunc == nil {
		return nil, errors.New("RPC connection factory is nil")
	}
	if conf.MaxConns < 0 {
		return nil, errors.New("RPC pool MaxConns must be positive")
	}
	if conf.MaxIdleConns < 0 {
		return nil, errors.New("RPC pool MaxIdleConns must not be negative")
	}
	if conf.MaxWaiters <= 0 {
		conf.MaxWaiters = max(1, conf.MaxConns)
	}
	if conf.Ping != nil && conf.PingTicker <= 0 {
		conf.PingTicker = int64(defaultRPCPingInterval / time.Second)
	}

	return parseRPCPoolEndpoints(conf.Addrs)
}

func parseRPCPoolEndpoints(addrs string) ([]rpcPoolEndpoint, error) {
	rawEndpoints := strings.Split(addrs, ",")
	endpoints := make([]rpcPoolEndpoint, 0, len(rawEndpoints))
	for _, rawEndpoint := range rawEndpoints {
		rawEndpoint = strings.TrimSpace(rawEndpoint)
		colon := strings.LastIndex(rawEndpoint, ":")
		if colon <= 0 || colon == len(rawEndpoint)-1 {
			return nil, fmt.Errorf("invalid RPC pool address %q: expected host:port/timeout_ms", rawEndpoint)
		}
		addr := strings.TrimSpace(rawEndpoint[:colon])
		portAndTimeout := strings.Split(rawEndpoint[colon+1:], "/")
		if addr == "" || len(portAndTimeout) != 2 {
			return nil, fmt.Errorf("invalid RPC pool address %q: expected host:port/timeout_ms", rawEndpoint)
		}
		port, err := strconv.Atoi(strings.TrimSpace(portAndTimeout[0]))
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid RPC pool port in %q", rawEndpoint)
		}
		timeout, err := strconv.Atoi(strings.TrimSpace(portAndTimeout[1]))
		if err != nil || timeout <= 0 {
			return nil, fmt.Errorf("invalid RPC pool timeout in %q", rawEndpoint)
		}
		endpoints = append(endpoints, rpcPoolEndpoint{addr: addr, port: port, timeout: timeout})
	}
	if len(endpoints) == 0 {
		return nil, errors.New("RPC pool has no endpoints")
	}
	return endpoints, nil
}

func (rps *RpcPoolSelector[T]) selectedServer(ctx context.Context) (*RpcSvr[T], error) {
	if rps.initErr != nil {
		return nil, rps.initErr
	}
	if rps.closed.Load() {
		return nil, ErrPoolClosed
	}
	selected := rps.RoundRobin(ctx)
	if selected == nil {
		logger.Warn(ctx, "select RPC server failed", "reason", "no_valid_server")
		return nil, ErrNoValidRPCServer
	}
	return selected.(*RpcSvr[T]), nil
}

func (rps *RpcPoolSelector[T]) handleCallError(ctx context.Context, server *RpcSvr[T], err error) error {
	if err == nil {
		return nil
	}
	logger.Warn(ctx, "RPC call failed", "addr", server.GetAddr(), "port", server.GetPort(), "err", err)
	if rps.Gpconf == nil || rps.Gpconf.Ping == nil || !rps.shouldEject(err) {
		return err
	}
	if !server.CompareAndSwapStat(selector.SVR_VALID, selector.SVR_NOTVALID) {
		return err
	}
	logger.Warn(ctx, "RPC server unavailable", "addr", server.GetAddr(), "port", server.GetPort(), "err", err)
	select {
	case rps.NotValid <- server:
	default:
		// The periodic scan observes the invalid state even when the immediate
		// notification buffer is full.
	}
	return err
}

func (rps *RpcPoolSelector[T]) shouldEject(err error) bool {
	rps.ejectMu.RLock()
	policy := rps.ejectPolicy
	rps.ejectMu.RUnlock()
	if policy != nil {
		return policy(err)
	}
	var transportErr thrift.TTransportException
	if errors.As(err, &transportErr) {
		return true
	}
	// Preserve the historical default: both transport and protocol exceptions
	// eject an endpoint when Ping-based recovery is configured.
	var protocolErr thrift.TProtocolException
	return errors.As(err, &protocolErr)
}

// SetShouldEject optionally overrides endpoint-ejection classification without
// changing the historical GPoolConfig struct layout. Passing nil restores the
// default transport-or-protocol policy.
func (rps *RpcPoolSelector[T]) SetShouldEject(policy func(error) bool) {
	rps.ejectMu.Lock()
	rps.ejectPolicy = policy
	rps.ejectMu.Unlock()
}

func (rps *RpcPoolSelector[T]) startHealthChecker(parent context.Context) {
	rps.lifecycleMu.Lock()
	defer rps.lifecycleMu.Unlock()
	healthCtx, cancel := context.WithCancel(context.WithoutCancel(parent))
	rps.healthCancel = cancel
	rps.healthDone = make(chan struct{})
	done := rps.healthDone
	go rps.runHealthChecker(healthCtx, done)
}

func (rps *RpcPoolSelector[T]) runHealthChecker(ctx context.Context, done chan struct{}) {
	defer close(done)
	interval := defaultRPCPingInterval
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
				server, ok := item.(*RpcSvr[T])
				if ok && server.GetValid() == selector.SVR_NOTVALID {
					rps.checkServerHealth(ctx, server)
				}
			}
		}
	}
}

func (rps *RpcPoolSelector[T]) checkServerHealth(ctx context.Context, server *RpcSvr[T]) {
	if server == nil || server.GetValid() == selector.SVR_VALID || rps.closed.Load() || rps.Gpconf == nil || rps.Gpconf.Ping == nil {
		return
	}
	timeout := time.Duration(server.GetTimeOut()) * time.Millisecond
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	err := server.Gp.ThriftCall2(pingCtx, rps.Gpconf.Ping)
	cancel()
	if err != nil {
		logger.Warn(ctx, "RPC server health check failed", "addr", server.GetAddr(), "port", server.GetPort(), "err", err)
		return
	}
	if rps.closed.Load() {
		return
	}
	server.SetStat(selector.SVR_VALID)
	logger.Info(ctx, "RPC server recovered", "addr", server.GetAddr(), "port", server.GetPort())
}

func (rps *RpcPoolSelector[T]) ThriftCall(ctx context.Context, process func(client interface{}) (string, error)) error {
	server, err := rps.selectedServer(ctx)
	if err != nil {
		return err
	}
	return rps.handleCallError(ctx, server, server.Gp.ThriftCall2(ctx, process))
}

func (rps *RpcPoolSelector[T]) ThriftWithTimeOutCall(ctx context.Context, timeout time.Duration, process func(client interface{}) (string, error)) error {
	server, err := rps.selectedServer(ctx)
	if err != nil {
		return err
	}
	return rps.handleCallError(ctx, server, server.Gp.ThriftWithTimeOutCall2(ctx, timeout, process))
}

func (rps *RpcPoolSelector[T]) ThriftExtCall(ctx context.Context, process func(ctx context.Context, client interface{}) (string, error)) error {
	nCtx, err := rpcExtensionContext(ctx)
	if err != nil {
		return err
	}
	server, err := rps.selectedServer(nCtx)
	if err != nil {
		return err
	}
	return rps.handleCallError(nCtx, server, server.Gp.ThriftExtCall2(nCtx, process))
}

func (rps *RpcPoolSelector[T]) ThriftWithTimeOutExtCall(ctx context.Context, timeout time.Duration, process func(ctx context.Context, client interface{}) (string, error)) error {
	nCtx, err := rpcExtensionContext(ctx)
	if err != nil {
		return err
	}
	server, err := rps.selectedServer(nCtx)
	if err != nil {
		return err
	}
	return rps.handleCallError(nCtx, server, server.Gp.ThriftWithTimeOutExtCall2(nCtx, timeout, process))
}

func rpcExtensionContext(ctx context.Context) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	eCtx, ok := ctx.(*ExtContext)
	if !ok {
		logger.Warn(ctx, "invalid thrift extension context", "context_type", fmt.Sprintf("%T", ctx))
		return nil, errors.New("ctx format error")
	}
	if eCtx.GetReqExtData("request_id") == "" {
		eCtx = eCtx.SetReqExtData(eCtx, "request_id", util.NewRequestID())
	}
	return eCtx, nil
}

// Close stops health checking, rejects new selection, and closes every
// endpoint's independent connection pool. It can be called again with a fresh
// context after an earlier timeout.
func (rps *RpcPoolSelector[T]) Close(ctx context.Context) error {
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
		server, ok := item.(*RpcSvr[T])
		if !ok || server.Gp == nil {
			continue
		}
		wg.Add(1)
		go func(pool *Gpool[T]) {
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

	// Initiate child-pool shutdown before waiting for a user Ping callback that
	// may not honor cancellation.
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
