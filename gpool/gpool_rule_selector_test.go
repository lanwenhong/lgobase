package gpool

import (
	"context"
	"errors"
	"testing"

	"github.com/hyperjumptech/grule-rule-engine/ast"
	"github.com/lanwenhong/lgobase/gconfig_v2"
)

type ruleSelectorTestClient struct{}

func createFailedRuleSelectorTestConn(ctx context.Context, addr string, port int, timeout int) (Conn[ruleSelectorTestClient], error) {
	return nil, errors.New("connect failed")
}

func newRulePoolSelectorForTest(t *testing.T) *RpcPoolRuleSelector[ruleSelectorTestClient] {
	t.Helper()

	type Config struct {
		PayServers []RpcServerConf `yaml:"payservers"`
	}

	var cfg Config
	if err := gconfig_v2.UnmarshalFile(context.Background(), "rule_pool.yaml", &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.PayServers) != 3 {
		t.Fatalf("len(cfg.PayServers) = %d", len(cfg.PayServers))
	}

	selector := NewRpcRulePoolSelector[ruleSelectorTestClient]()
	for i := range cfg.PayServers {
		conf := cfg.PayServers[i]
		if err := selector.AddSvr(context.Background(), &conf, createFailedRuleSelectorTestConn, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := selector.ParseRule(context.Background()); err != nil {
		t.Fatal(err)
	}
	return selector
}

func requireRuleSelectedAddr(t *testing.T, pool *RpcPoolSelector[ruleSelectorTestClient], want string) {
	t.Helper()
	if pool == nil {
		t.Fatalf("selected pool is nil, want %s", want)
	}
	if pool.Gpconf == nil {
		t.Fatalf("selected pool has nil config, want %s", want)
	}
	if pool.Gpconf.Addrs != want {
		t.Fatalf("selected addr = %s, want %s", pool.Gpconf.Addrs, want)
	}
	if len(pool.Slist) == 0 {
		t.Fatalf("selected pool %s was not initialized", want)
	}
	rpcSvr, ok := pool.Slist[0].(*RpcSvr[ruleSelectorTestClient])
	if !ok {
		t.Fatalf("selected pool server type = %T", pool.Slist[0])
	}
	if rpcSvr.Gp == nil || rpcSvr.Gp.Gpconf == nil {
		t.Fatalf("selected pool %s has uninitialized gpool", want)
	}
}

func TestRpcPoolRuleSelectorSvrSelectFromJson(t *testing.T) {
	selector := newRulePoolSelectorForTest(t)

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "amount rule",
			json: `{"busicd":"1000","txamt":2000}`,
			want: "192.168.100.103:1000/1000",
		},
		{
			name: "business code rule",
			json: `{"busicd":"802801","txamt":1}`,
			want: "192.168.100.106:1000/1000",
		},
		{
			name: "fallback rule",
			json: `{"busicd":"other","txamt":1}`,
			want: "192.168.100.104:1000/1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := selector.SvrSelectFromJson(context.Background(), tt.json, "trade")
			if err != nil {
				t.Fatal(err)
			}
			requireRuleSelectedAddr(t, pool, tt.want)
		})
	}
}

func TestRpcPoolRuleSelectorSvrSelectFromDataCtx(t *testing.T) {
	selector := newRulePoolSelectorForTest(t)

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "amount rule",
			json: `{"busicd":"1000","txamt":2000}`,
			want: "192.168.100.103:1000/1000",
		},
		{
			name: "business code rule",
			json: `{"busicd":"802801","txamt":1}`,
			want: "192.168.100.106:1000/1000",
		},
		{
			name: "fallback rule",
			json: `{"busicd":"other","txamt":1}`,
			want: "192.168.100.104:1000/1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataCtx := ast.NewDataContext()
			if err := dataCtx.AddJSON("trade", []byte(tt.json)); err != nil {
				t.Fatal(err)
			}

			pool, err := selector.SvrSelectFromDataCtx(context.Background(), dataCtx)
			if err != nil {
				t.Fatal(err)
			}
			requireRuleSelectedAddr(t, pool, tt.want)
		})
	}
}
