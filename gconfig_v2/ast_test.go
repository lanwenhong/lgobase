package gconfig_v2

import (
	"context"
	"testing"
)

func TestPrintAstTree(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config_complex.yaml")
	if py == nil {
		t.Fatal("NewParseYaml returned nil")
	}

	root, err := py.parse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rootNode, ok := root["root"].(*AstNode)
	if !ok {
		t.Fatalf("root ast node not found, got %T", root["root"])
	}

	PrintAstTree(ctx, rootNode)
}
