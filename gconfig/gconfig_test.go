package gconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hyperjumptech/grule-rule-engine/antlr"
	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/hyperjumptech/grule-rule-engine/builder"
	"github.com/hyperjumptech/grule-rule-engine/engine"
	"github.com/hyperjumptech/grule-rule-engine/pkg"
	jsoniter "github.com/json-iterator/go"
	"github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/sirupsen/logrus"
)

type GConfRule struct {
	Name        string   `json:"name"`
	Description string   `json:"desc"`
	Salience    int      `json:"salience"`
	RuleWhen    string   `json:"when"`
	RuleThen    []string `json:"then"`
}

type Trade struct {
	Busicd string `json:"busicd"`
	Txamt  int    `json:"txamt"`
}

type Ret struct {
	Svr string
}

func (r *Ret) Set(addr string) {
	r.Svr = addr
}

func TestGparse(t *testing.T) {
	t.Log("start")
	g_cf := gconfig.NewGconf("test1.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}
	if se, ok := g_cf.Gcf["section1"]; ok {
		if test1, ok := se["test1"]; ok {
			t.Logf("get test1: %s", test1)
		} else {
			t.Errorf("test1 not found")
		}

		if test2, ok := se["test2"]; ok {
			t.Logf("get test2: %s", test2)
		} else {
			t.Errorf("test2 not found")
		}

		if test3, ok := se["test3"]; ok {
			t.Logf("get test3: %s", test3)
			if ex, ok := g_cf.GlineExtend["test3"]; ok {
				t.Logf("get test3 ex: %s", ex)
			} else {
				t.Errorf("test3 extend not found")
			}
		} else {
			t.Errorf("test3 not found")
		}

	} else {
		t.Errorf("section1 not found")
	}
}

func buildRule(ctx context.Context, ex map[string]map[string][]string) error {
	logger.Debugf(ctx, "ex: %v", ex)

	server := "test1"
	c_test1 := ex[server]
	lcr := []GConfRule{}
	i := 0
	for k, v := range c_test1 {
		logger.Debugf(ctx, "k: %s", k)
		logger.Debugf(ctx, "v: %s", v)
		cr := GConfRule{}
		//cr.Name = k
		//cr.Name = util.GenXid()
		//cr.Name = util.GenBetterGUID()
		cr.Name = fmt.Sprintf("%s%d", server, i)
		i++
		cr.Description = k
		//cr.Description = "cc"
		cr.RuleWhen = v[0]
		//cr.Salience = v[1]
		s, _ := strconv.Atoi(v[1])
		cr.Salience = s
		setRet := fmt.Sprintf("R.Set('%s')", k)
		logger.Debugf(ctx, "setRet: %s", setRet)
		cr.RuleThen = []string{
			setRet,
			`Complete()`,
		}
		lcr = append(lcr, cr)
	}
	logger.Debugf(ctx, "lcr: %v", lcr)

	config := jsoniter.Config{
		SortMapKeys: true,
	}
	jRuleSet, _ := config.Froze().Marshal(lcr)
	logger.Debugf(ctx, "rule set: %s", jRuleSet)

	trade := Trade{
		Busicd: "1000",
		Txamt:  2000,
	}

	pTrade, _ := json.Marshal(trade)
	logger.Debugf(ctx, "pTrade: %s", string(pTrade))

	dataContext := ast.NewDataContext()
	dataContext.AddJSON("trade", []byte(pTrade))
	rRet := &Ret{}
	dataContext.Add("R", rRet)

	Rdata, errJ := pkg.ParseJSONRuleset([]byte(jRuleSet))
	if errJ != nil {
		logger.Warnf(ctx, "errJ: %s", errJ.Error())
		return errJ
	}
	kl := ast.NewKnowledgeLibrary()
	rb := builder.NewRuleBuilder(kl)
	gFlag := "test_grule"
	//rb.MustBuildRuleFromResource(gFlag, "0.0.1", pkg.NewBytesResource([]byte(Rdata)))
	err := rb.BuildRuleFromResource(gFlag, "0.0.1", pkg.NewBytesResource([]byte(Rdata)))
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return err
	}
	kb, err := kl.NewKnowledgeBaseInstance(gFlag, "0.0.1")
	eng := engine.NewGruleEngine()
	//ruleEntries, err := eng.FetchMatchingRules(dataContext, kb)
	err = eng.Execute(dataContext, kb)
	if err != nil {
		return err
	}
	logger.Debugf(ctx, "ret: %v", rRet)
	/*for _, e := range ruleEntries {
		logger.Debugf(ctx, "server select: %s", e.RuleDescription)
	}*/

	//logger.Debugf(ctx, "when: %s", ex[0])
	//logger.Debugf(ctx, "Salience: %s", ex[1])
	return nil
}

func TestRule(t *testing.T) {
	//noop := &glog.NoopLogger{}
	logger := logrus.New()
	logger.Level = logrus.InfoLevel

	//glog.Log = noop.WithFields(glog.Fields{"lib": "grule-rule-engine"})
	/*glog.Log = glog.LogEntry{
		Logger: glog.NewLogrus(logger).WithFields(glog.Fields{"lib": "grule-rule-engine"}),
		Level:  glog.DebugLevel,
	}*/

	antlr.SetLogger(logger)
	ctx := context.Background()
	t.Log("start")
	g_cf := gconfig.NewGconf("test_rule.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}
	//g_cf.AddExd("test1", "test1 = 192.168.100.105/1000")
	if _, ok := g_cf.Gcf["section1"]; ok {
		err := buildRule(ctx, g_cf.GlineExtend)
		if err != nil {
			t.Fatal()
		}
	}
}
