package gconfig_v2_test

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestLoadConfFile(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	gconfig_v2.Unmarshal(ctx, "test_rule.yaml", &m)
}

func TestLoadConfFile2(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	gconfig_v2.Unmarshal(ctx, "config1.yaml", &m)
}

func TestLoadConfFile3(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	gconfig_v2.Unmarshal(ctx, "config5.yaml", &m)
}
