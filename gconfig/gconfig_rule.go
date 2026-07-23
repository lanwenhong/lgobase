package gconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/hyperjumptech/grule-rule-engine/builder"
	"github.com/hyperjumptech/grule-rule-engine/engine"
	"github.com/hyperjumptech/grule-rule-engine/pkg"
	jsoniter "github.com/json-iterator/go"
	"github.com/lanwenhong/lgobase/logger"
)

type GConfRuleEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"desc"`
	Salience    int      `json:"salience"`
	RuleWhen    string   `json:"when"`
	RuleThen    []string `json:"then"`
}

type GConfRule struct {
	Sever     string
	Kl        *ast.KnowledgeLibrary
	GrulePool *sync.Pool
}

type RuleRet struct {
	Svr string
}

func (r *RuleRet) Set(addr string) {
	r.Svr = addr
}

func NewGConfRule(svr string) *GConfRule {
	cr := &GConfRule{
		Sever: svr,
	}
	return cr
}

func (cr *GConfRule) AddRule(ctx context.Context, g_conf *Gconf) error {
	ex := g_conf.GlineExtend
	logger.Debug(ctx, "load rule config", "extensions", ex)
	if _, ok := ex[cr.Sever]; !ok {
		return errors.New("not found " + cr.Sever)
	}
	svrs := ex[cr.Sever]
	cnt := 0
	lcre := []GConfRuleEntry{}
	for k, v := range svrs {
		logger.Debug(ctx, "load rule config entry", "server", cr.Sever, "rule", k, "value", v)

		for _, iv := range v {
			cre := GConfRuleEntry{}
			cre.Name = fmt.Sprintf("%s%d", cr.Sever, cnt)
			cnt++
			cre.Description = k
			if when, ok := iv["rule"]; ok {
				cre.RuleWhen = when
			} else {
				continue
			}
			if salience, ok2 := iv["Salience"]; ok2 {
				s, err := strconv.Atoi(salience)
				if err != nil {
					panic(err)
				}
				cre.Salience = s
			} else {
				continue
			}
			setRet := fmt.Sprintf("R.Set('%s')", k)
			cre.RuleThen = []string{
				setRet,
				`Complete()`,
			}
			lcre = append(lcre, cre)
		}
	}
	config := jsoniter.Config{
		SortMapKeys: true,
	}
	jRuleSet, _ := config.Froze().Marshal(lcre)
	//logger.Debugf(ctx, "rule set: %s", jRuleSet)
	logger.Debug(ctx, "built rule set", "server", cr.Sever, "rule_set", json.RawMessage((string(jRuleSet))))

	Rdata, errJ := pkg.ParseJSONRuleset([]byte(jRuleSet))
	if errJ != nil {
		logger.Warn(ctx, "parse rule set failed", "server", cr.Sever, "err", errJ)
		panic(errJ)
	}
	kl := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(kl)
	rFlag := "gconf_" + cr.Sever
	err := rb.BuildRuleFromResource(rFlag, "0.0.1", pkg.NewBytesResource([]byte(Rdata)))
	if err != nil {
		logger.Warn(ctx, "build rule set failed", "server", cr.Sever, "rule", rFlag, "err", err)
		panic(err)
	}
	cr.Kl = kl
	cr.GrulePool = &sync.Pool{
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

func (cr *GConfRule) SvrSelectFromJson(ctx context.Context, jData string, jDataKey string) (*RuleRet, error) {
	dataContext := ast.NewDataContext()
	dataContext.AddJSON(jDataKey, []byte(jData))
	rRet := &RuleRet{}
	dataContext.Add("R", rRet)

	kb := cr.GrulePool.Get().(*ast.KnowledgeBase)
	defer cr.GrulePool.Put(kb)

	eng := engine.NewGruleEngine()
	err := eng.Execute(dataContext, kb)
	if err != nil {
		logger.Debug(ctx, "execute rule failed", "server", cr.Sever, "err", err)
		return nil, err
	}
	logger.Debug(ctx, "execute rule completed", "server", cr.Sever, "result", rRet)
	return rRet, nil
}

func (cr *GConfRule) SvrSelectFromDataCtx(ctx context.Context, dataContext ast.IDataContext) (*RuleRet, error) {
	rRet := &RuleRet{}
	dataContext.Add("R", rRet)
	kb := cr.GrulePool.Get().(*ast.KnowledgeBase)
	defer cr.GrulePool.Put(kb)

	eng := engine.NewGruleEngine()
	err := eng.Execute(dataContext, kb)
	if err != nil {
		logger.Debug(ctx, "execute rule failed", "server", cr.Sever, "err", err)
		return nil, err
	}
	logger.Debug(ctx, "execute rule completed", "server", cr.Sever, "result", rRet)
	return rRet, nil
}
