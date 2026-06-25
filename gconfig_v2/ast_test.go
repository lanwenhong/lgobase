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

func TestPrintAstTree1(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config6.yaml")
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

func TestPrintAstTree2(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config7.yaml")
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

func TestPrintAstTree3(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config8.yaml")
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

func TestPrintAstTree4(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config9.yaml")
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

func TestPrintAstTree5(t *testing.T) {
	ctx := context.Background()
	py := NewParseYaml("config10.yaml")
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
