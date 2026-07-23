package gpool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/hyperjumptech/grule-rule-engine/builder"
	"github.com/hyperjumptech/grule-rule-engine/engine"
	"github.com/hyperjumptech/grule-rule-engine/pkg"
	jsoniter "github.com/json-iterator/go"
	"github.com/lanwenhong/lgobase/logger"
)

type RuleItem struct {
	Name        string   `json:"name"`
	Description string   `json:"desc"`
	Salience    int      `json:"salience"`
	RuleWhen    string   `json:"when"`
	RuleThen    []string `json:"then"`
}

type RpcServerConf struct {
	Addr            string  `yaml:"addr"`
	Rule            string  `yaml:"rule"`
	Salience        int     `yaml:"Salience"`
	MaxConns        int     `yaml:"MaxConns"`
	MaxIdleConns    int     `yaml:"MaxIdleConns"`
	MaxWaiters      int     `yaml:"MaxWaiters"`
	MaxConnLife     int64   `yaml:"MaxConnLife"`
	MaxIdleConnLife int64   `yaml:"MaxIdleConnLife"`
	PurgeRate       float64 `yaml:"PurgeRate"`
	Proto           string  `yaml:"proto"`
	PingTicker      int64   `yaml:"PingTicker"`
}

type RuleRet struct {
	Svr string
}

func (r *RuleRet) Set(addr string) {
	r.Svr = addr
}

type RpcPoolRuleSelector[T any] struct {
	rlist     []RuleItem
	RulePools map[string]*RpcPoolSelector[T]
	Kl        *ast.KnowledgeLibrary
	GrulePool *sync.Pool

	mu     sync.RWMutex
	closed bool
}

func NewRpcRulePoolSelector[T any]() *RpcPoolRuleSelector[T] {
	return &RpcPoolRuleSelector[T]{
		RulePools: make(map[string]*RpcPoolSelector[T]),
	}
}

func (rrps *RpcPoolRuleSelector[T]) AddSvr(ctx context.Context, conf *RpcServerConf,
	cfunc CreateConn[T], nc NewThriftClient[T], ping PingSvr) error {
	rrps.mu.RLock()
	closed := rrps.closed
	rrps.mu.RUnlock()
	if closed {
		return ErrPoolClosed
	}
	if conf == nil {
		return errors.New("server config must not be nil")
	}
	if strings.TrimSpace(conf.Rule) == "" {
		return errors.New("server rule must not be empty")
	}
	if cfunc == nil {
		return errors.New("create conn func must not be nil")
	}
	if err := rrps.validateRpcServerAddrs(conf.Addr); err != nil {
		return err
	}
	gpConf := &GPoolConfig[T]{
		Addrs:           conf.Addr,
		MaxConns:        conf.MaxConns,
		MaxIdleConns:    conf.MaxIdleConns,
		MaxWaiters:      conf.MaxWaiters,
		MaxConnLife:     conf.MaxConnLife,
		MaxIdleConnLife: conf.MaxIdleConnLife,
		PurgeRate:       conf.PurgeRate,
		PingTicker:      conf.PingTicker,
		Cfunc:           cfunc,
		Nc:              nc,
		Ping:            ping,
	}
	applyRPCPoolConfigDefaults(gpConf)
	rps := &RpcPoolSelector[T]{}
	if err := rps.RpcPoolInit(ctx, gpConf); err != nil {
		return err
	}
	rItem := RuleItem{
		Description: conf.Addr,
		RuleWhen:    conf.Rule,
		Salience:    conf.Salience,
	}
	setRet := fmt.Sprintf("R.Set('%s')", conf.Addr)
	rItem.RuleThen = []string{
		setRet,
		`Complete()`,
	}

	rrps.mu.Lock()
	if rrps.closed {
		rrps.mu.Unlock()
		_ = rps.Close(context.Background())
		return ErrPoolClosed
	}
	if rrps.RulePools == nil {
		rrps.RulePools = make(map[string]*RpcPoolSelector[T])
	}
	previous := rrps.RulePools[conf.Addr]
	rrps.RulePools[conf.Addr] = rps
	rItem.Name = fmt.Sprintf("rpc_pool_rule_%d", len(rrps.rlist))
	rrps.rlist = append(rrps.rlist, rItem)
	rrps.mu.Unlock()

	if previous != nil {
		if err := previous.Close(ctx); err != nil {
			logger.Warn(ctx, "close replaced RPC rule pool failed", "addr", conf.Addr, "err", err)
		}
	}
	return nil
}

func (rrps *RpcPoolRuleSelector[T]) validateRpcServerAddrs(addrs string) error {
	_, err := parseRPCPoolEndpoints(addrs)
	return err
}

func (rrps *RpcPoolRuleSelector[T]) ParseRule(ctx context.Context) error {
	rrps.mu.RLock()
	if rrps.closed {
		rrps.mu.RUnlock()
		return ErrPoolClosed
	}
	rules := append([]RuleItem(nil), rrps.rlist...)
	rrps.mu.RUnlock()
	if len(rules) == 0 {
		return errors.New("no rule configured")
	}
	config := jsoniter.Config{
		SortMapKeys: true,
	}
	jRuleSet, _ := config.Froze().Marshal(rules)
	logger.Debug(ctx, "built RPC selector rule set", "rule_set", json.RawMessage((string(jRuleSet))))

	Rdata, errJ := pkg.ParseJSONRuleset([]byte(jRuleSet))
	if errJ != nil {
		logger.Warn(ctx, "parse rule set failed", "err", errJ)
		return errJ
	}
	kl := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(kl)
	rFlag := "gconf_" + "servers"
	err := rb.BuildRuleFromResource(rFlag, "0.0.1", pkg.NewBytesResource([]byte(Rdata)))
	if err != nil {
		logger.Warn(ctx, "build rule set failed", "rule", rFlag, "err", err)
		return err
	}
	grulePool := &sync.Pool{
		New: func() interface{} {
			res, err := kl.NewKnowledgeBaseInstance(rFlag, "0.0.1")
			if err != nil {
				logger.Debug(ctx, "create rule knowledge base failed", "rule", rFlag, "err", err)
				panic(err)
			}
			return res
		},
	}
	rrps.mu.Lock()
	if rrps.closed {
		rrps.mu.Unlock()
		return ErrPoolClosed
	}
	rrps.Kl = kl
	rrps.GrulePool = grulePool
	rrps.mu.Unlock()
	return nil
}

func (rrps *RpcPoolRuleSelector[T]) SvrSelectFromJson(ctx context.Context, jData string, jDataKey string) (*RpcPoolSelector[T], error) {
	rrps.mu.RLock()
	if rrps.closed {
		rrps.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	grulePool := rrps.GrulePool
	rrps.mu.RUnlock()
	if grulePool == nil {
		return nil, errors.New("rule selector is not initialized")
	}
	dataContext := ast.NewDataContext()
	if err := dataContext.AddJSON(jDataKey, []byte(jData)); err != nil {
		return nil, err
	}
	rRet := &RuleRet{}
	if err := dataContext.Add("R", rRet); err != nil {
		return nil, err
	}

	kb := grulePool.Get().(*ast.KnowledgeBase)
	defer grulePool.Put(kb)

	eng := engine.NewGruleEngine()
	err := eng.Execute(dataContext, kb)
	if err != nil {
		logger.Debug(ctx, "execute rule failed", "err", err)
		return nil, err
	}
	logger.Debug(ctx, "execute rule completed", "result", rRet)
	return rrps.rulePoolByRet(rRet)
}

func (cr *RpcPoolRuleSelector[T]) SvrSelectFromDataCtx(ctx context.Context, dataContext ast.IDataContext) (*RpcPoolSelector[T], error) {
	cr.mu.RLock()
	if cr.closed {
		cr.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	grulePool := cr.GrulePool
	cr.mu.RUnlock()
	if grulePool == nil {
		return nil, errors.New("rule selector is not initialized")
	}
	rRet := &RuleRet{}
	if err := dataContext.Add("R", rRet); err != nil {
		return nil, err
	}
	kb := grulePool.Get().(*ast.KnowledgeBase)
	defer grulePool.Put(kb)

	eng := engine.NewGruleEngine()
	err := eng.Execute(dataContext, kb)
	if err != nil {
		logger.Debug(ctx, "execute rule failed", "err", err)
		return nil, err
	}
	logger.Debug(ctx, "execute rule completed", "result", rRet)
	return cr.rulePoolByRet(rRet)
}

func (rrps *RpcPoolRuleSelector[T]) rulePoolByRet(rRet *RuleRet) (*RpcPoolSelector[T], error) {
	if rRet.Svr == "" {
		return nil, errors.New("no rule matched")
	}
	rrps.mu.RLock()
	defer rrps.mu.RUnlock()
	if rrps.closed {
		return nil, ErrPoolClosed
	}
	svr, ok := rrps.RulePools[rRet.Svr]
	if !ok {
		return nil, fmt.Errorf("rule matched unknown server %q", rRet.Svr)
	}
	return svr, nil
}

// Close prevents further rule selection and closes every selector and endpoint
// pool owned by this rule selector.
func (rrps *RpcPoolRuleSelector[T]) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	rrps.mu.Lock()
	if !rrps.closed {
		rrps.closed = true
		rrps.GrulePool = nil
	}
	pools := make([]*RpcPoolSelector[T], 0, len(rrps.RulePools))
	for _, pool := range rrps.RulePools {
		pools = append(pools, pool)
	}
	rrps.mu.Unlock()

	errCh := make(chan error, len(pools))
	var wg sync.WaitGroup
	for _, pool := range pools {
		if pool == nil {
			continue
		}
		wg.Add(1)
		go func(pool *RpcPoolSelector[T]) {
			defer wg.Done()
			if err := pool.Close(ctx); err != nil {
				errCh <- err
			}
		}(pool)
	}
	wg.Wait()
	close(errCh)
	var closeErr error
	for err := range errCh {
		closeErr = errors.Join(closeErr, err)
	}
	return closeErr
}
