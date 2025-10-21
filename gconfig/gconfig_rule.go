package gconfig

import (
	"context"
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
	if _, ok := ex[cr.Sever]; !ok {
		return errors.New("not found " + cr.Sever)
	}
	svrs := ex[cr.Sever]
	cnt := 0
	lcre := []GConfRuleEntry{}
	for k, v := range svrs {
		logger.Debugf(ctx, "k: %s", k)
		logger.Debugf(ctx, "v: %s", v)

		cre := GConfRuleEntry{}
		cre.Name = fmt.Sprintf("%s%d", cr.Sever, cnt)
		cnt++
		cre.Description = k
		cre.RuleWhen = v[0]
		s, err := strconv.Atoi(v[1])
		if err != nil {
			logger.Warnf(ctx, "err: %s", err.Error())
			panic(err)
		}
		cre.Salience = s
		setRet := fmt.Sprintf("R.Set('%s')", k)
		cre.RuleThen = []string{
			setRet,
			`Complete()`,
		}
		lcre = append(lcre, cre)
	}

	config := jsoniter.Config{
		SortMapKeys: true,
	}
	jRuleSet, _ := config.Froze().Marshal(lcre)
	logger.Debugf(ctx, "rule set: %s", jRuleSet)

	Rdata, errJ := pkg.ParseJSONRuleset([]byte(jRuleSet))
	if errJ != nil {
		logger.Warnf(ctx, "errJ: %s", errJ.Error())
		panic(errJ)
	}
	kl := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(kl)
	rFlag := "gconf_" + cr.Sever
	err := rb.BuildRuleFromResource(rFlag, "0.0.1", pkg.NewBytesResource([]byte(Rdata)))
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		panic(err)
	}
	cr.Kl = kl
	cr.GrulePool = &sync.Pool{
		New: func() interface{} {
			res, err := kl.NewKnowledgeBaseInstance(rFlag, "0.0.1")
			if err != nil {
				logger.Debugf(ctx, "err: %s", err.Error())
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
		logger.Debugf(ctx, "exec err %s", err.Error())
		return nil, err
	}
	logger.Debugf(ctx, "ret: %v", rRet)
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
		logger.Debugf(ctx, "exec err %s", err.Error())
		return nil, err
	}
	logger.Debugf(ctx, "ret: %v", rRet)
	return rRet, nil
}
