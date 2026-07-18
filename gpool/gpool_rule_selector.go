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
}

func NewRpcRulePoolSelector[T any]() *RpcPoolRuleSelector[T] {
	return &RpcPoolRuleSelector[T]{
		RulePools: make(map[string]*RpcPoolSelector[T]),
	}
}

func (rrps *RpcPoolRuleSelector[T]) AddSvr(ctx context.Context, conf *RpcServerConf,
	cfunc CreateConn[T], nc NewThriftClient[T], ping PingSvr) error {
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
	if rrps.RulePools == nil {
		rrps.RulePools = make(map[string]*RpcPoolSelector[T])
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
	if gpConf.MaxConns == 0 {
		gpConf.MaxConns = 200
	}
	if gpConf.MaxIdleConns == 0 {
		gpConf.MaxIdleConns = 100
	}

	rps := &RpcPoolSelector[T]{}
	if err := rps.RpcPoolInit(ctx, gpConf); err != nil {
		return err
	}
	rrps.RulePools[conf.Addr] = rps
	rItem := RuleItem{
		Name:        fmt.Sprintf("rpc_pool_rule_%d", len(rrps.rlist)),
		Description: conf.Addr,
		RuleWhen:    conf.Rule,
		Salience:    conf.Salience,
	}
	setRet := fmt.Sprintf("R.Set('%s')", conf.Addr)
	rItem.RuleThen = []string{
		setRet,
		`Complete()`,
	}
	rrps.rlist = append(rrps.rlist, rItem)
	return nil
}

func (rrps *RpcPoolRuleSelector[T]) validateRpcServerAddrs(addrs string) error {
	if strings.TrimSpace(addrs) == "" {
		return errors.New("server addr must not be empty")
	}
	for _, addr := range strings.Split(addrs, ",") {
		addr = strings.TrimSpace(addr)
		parts := strings.Split(addr, ":")
		if len(parts) != 2 || parts[0] == "" {
			return fmt.Errorf("addr format error: %s", addr)
		}
		portTimeout := strings.Split(parts[1], "/")
		if len(portTimeout) != 2 || portTimeout[0] == "" || portTimeout[1] == "" {
			return fmt.Errorf("addr format error: %s", addr)
		}
	}
	return nil
}

func (rrps *RpcPoolRuleSelector[T]) ParseRule(ctx context.Context) error {
	if len(rrps.rlist) == 0 {
		return errors.New("no rule configured")
	}
	config := jsoniter.Config{
		SortMapKeys: true,
	}
	jRuleSet, _ := config.Froze().Marshal(rrps.rlist)
	logger.Debug(ctx, "rule config", "rule_set", json.RawMessage((string(jRuleSet))))

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
	rrps.Kl = kl
	rrps.GrulePool = &sync.Pool{
		New: func() interface{} {
			res, err := kl.NewKnowledgeBaseInstance(rFlag, "0.0.1")
			if err != nil {
				logger.Debug(ctx, "create rule knowledge base failed", "rule", rFlag, "err", err)
				panic(err)
			}
			return res
		},
	}
	return nil
}

func (rrps *RpcPoolRuleSelector[T]) SvrSelectFromJson(ctx context.Context, jData string, jDataKey string) (*RpcPoolSelector[T], error) {
	if rrps.GrulePool == nil {
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

	kb := rrps.GrulePool.Get().(*ast.KnowledgeBase)
	defer rrps.GrulePool.Put(kb)

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
	if cr.GrulePool == nil {
		return nil, errors.New("rule selector is not initialized")
	}
	rRet := &RuleRet{}
	if err := dataContext.Add("R", rRet); err != nil {
		return nil, err
	}
	kb := cr.GrulePool.Get().(*ast.KnowledgeBase)
	defer cr.GrulePool.Put(kb)

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
	svr, ok := rrps.RulePools[rRet.Svr]
	if !ok {
		return nil, fmt.Errorf("rule matched unknown server %q", rRet.Svr)
	}
	return svr, nil
}
