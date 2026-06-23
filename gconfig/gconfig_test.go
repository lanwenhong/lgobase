package gconfig_test

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
	STxamt string `json:"s_txamt"`
	Chnlid int    `json:"chnlid"`
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

func buildRule(ctx context.Context, ex map[string]map[string][]map[string]string) error {
	logger.Debugf(ctx, "ex: %v", ex)

	server := "test1"
	c_test1 := ex[server]
	lcr := []GConfRule{}
	i := 0
	for k, v := range c_test1 {
		logger.Debugf(ctx, "k: %s", k)
		logger.Debugf(ctx, "v: %s", v)
		for _, iv := range v {
			cr := GConfRule{}
			cr.Name = fmt.Sprintf("%s%d", server, i)
			i++
			cr.Description = k
			cr.RuleWhen = iv["rule"]
			s, _ := strconv.Atoi(iv["Salience"])
			cr.Salience = s
			setRet := fmt.Sprintf("R.Set('%s')", k)
			logger.Debugf(ctx, "setRet: %s", setRet)
			cr.RuleThen = []string{
				setRet,
				`Complete()`,
			}
			lcr = append(lcr, cr)
		}
		/*cr := GConfRule{}
		cr.Name = fmt.Sprintf("%s%d", server, i)
		i++
		cr.Description = k
		//cr.RuleWhen = v[0]
		cr.RuleWhen = v[0][0]
		s, _ := strconv.Atoi(v[0][1])
		cr.Salience = s
		setRet := fmt.Sprintf("R.Set('%s')", k)
		logger.Debugf(ctx, "setRet: %s", setRet)
		cr.RuleThen = []string{
			setRet,
			`Complete()`,
		}
		lcr = append(lcr, cr)*/
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

func TestGConfRule(t *testing.T) {
	xlogger := logrus.New()
	xlogger.Level = logrus.InfoLevel

	antlr.SetLogger(xlogger)
	ctx := context.Background()
	//t.Log("start")
	g_cf := gconfig.NewGconf("test_rule.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}

	gcr := gconfig.NewGConfRule("test1")
	gcr.AddRule(ctx, g_cf)
	trade := Trade{
		Busicd: "7000",
		Txamt:  3000,
	}
	trade.STxamt = strconv.Itoa(trade.Txamt)

	pTrade, _ := json.Marshal(trade)
	logger.Debugf(ctx, "pTrade: %s", string(pTrade))
	gcr.SvrSelectFromJson(ctx, string(pTrade), "trade")
}

func TestWithDataContext(t *testing.T) {
	xlogger := logrus.New()
	xlogger.Level = logrus.DebugLevel

	/*glog.Log = noop.WithFields(glog.Fields{"lib": "grule-rule-engine"})
	glog.Log = glog.LogEntry{
		Logger: glog.NewLogrus(xlogger).WithFields(glog.Fields{"lib": "grule-rule-engine"}),
		Level:  glog.DebugLevel,
	}*/

	antlr.SetLogger(xlogger)
	ctx := context.Background()
	//t.Log("start")
	g_cf := gconfig.NewGconf("test_rule.ini")
	err := g_cf.GconfParse()
	if err != nil {
		t.Errorf("err: %s", err.Error())
	}

	gcr := gconfig.NewGConfRule("test1")
	gcr.AddRule(ctx, g_cf)
	trade := Trade{
		Busicd: "802801",
		Chnlid: 1132,
		Txamt:  3000,
	}
	trade.STxamt = strconv.Itoa(trade.Txamt)
	/*pTrade := `{
	    "cardaid": "A0000000031010",
	    "cardbin": "433668",
	    "cardscheme": "VISA",
	    "entry_mode": "tap",
	    "extend_info": "TfSjLPt7l0aJQx3kYB/POxtD0T/+C4cLY/Sizy5tOYGzdReMNQp2PpDADO99TuieH1wGq118lxHNQ6LDUvsxZ5MvxbvGd/rnSvosJC6dkrRSF1gNPLxZj9frj4ku4M2IbVeN7hx2qDyXxDJoArPbfmERViboyDLUc32EEifherB3kRlNGNlCO3gCsOcWaFbgXN5y/krmkjBllUOZtWGFQn0d+oovbWH1VBzbQrfTkvBXTAda2hAGtsLLokoL2ZsLUD6MzV28McUhlj/DH7SZ/SWmOyg2nvhgODE+AxCf3uKBVyUnwUrbNQE3ZLY/pDG35Y899DF+IDB6k+BX9qT8w/pDuGSwqmqksrA0CBgaDCQa+nKH/rcfCrlUQDyI7a4VMSybC0z2hy7wFsoZ50AJXuKFGaXmCFYCjZkX6Jj0mHqYipu8//vRzDzZGtQuHBBA3IVIxXr9rJN7NA4SAQNCZ77ZLSYEoMHrGDpq2eEbqnGi4C2d7BagXbIy+lHXILVWceX/As2phFswCdkaCazGu8Vz7S64sKAqoAZbJn/dPR6ejKkTGf8W7u5tTcU4ew3LV/Md5e/buHRD8i1/SWuu6z2YAkgT8om6HmHUxDPANqDjsUnimGSLxdlNfGSdXcHcj2gal1ZvXbCVdYxm2vcoSQ\u003d\u003d",
	    "tip_amt": "",
	    "account_device_id": "24BGCASW8320",
	    "app_name": "hjsh",
	    "appver": "4.34.16.1",
	    "busicd": "802801",
	    "clisn": "035402",
	    "clitm": "2025-09-19 09:50:02",
	    "contact": "911300004230002",
	    "lnglat": [
	        "0",
	        "0"
	    ],
	    "cardNo":"2321312312321312",
	    "iccdata":"21312312312sadasdasdsad",
	    "network": "wifi",
	    "os": "Android",
	    "osver": "10",
	    "phonemodel": "A8S",
	    "txamt": "100",
	    "txcurrcd": "344",
	    "txdtm": "2025-09-19 09:50:02",
	    "txzone": "+0800",
	    "udid": "8690710558929631",
	    "userid": "1130081629",
	    "opuid": null
	}`*/
	//pTrade := `{"busicd": "802801"}`
	pTrade, _ := json.Marshal(trade)
	logger.Debugf(ctx, "pTrade: %s", string(pTrade))
	dataContext := ast.NewDataContext()
	dataContext.AddJSON("trade", []byte(pTrade))
	gcr.SvrSelectFromDataCtx(ctx, dataContext)
}
