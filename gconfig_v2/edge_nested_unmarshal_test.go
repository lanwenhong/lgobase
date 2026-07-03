package gconfig_v2_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalEdgeSpecialCharsAndFlow(t *testing.T) {
	content := []byte(`edge:
  routes:
    - name: "order:create#v1"
      endpoint: "https://api.example.com/a,b?token='x:y#z'"
      methods: [GET, "POST,JSON", !!str on, "*"]
      headers: {x-token: "abc,123:xyz#hash", x-star: "*", x-anchor: "&anchor", x-tag: "!tag", x-brackets: "[a,b]{c:d}"}
      flags: {enabled: true, ratio: 0.875, retry_codes: [500, 502, 503], empty: []}
`)

	type Flags struct {
		Enabled    bool     `yaml:"enabled"`
		Ratio      float64  `yaml:"ratio"`
		RetryCodes []int    `yaml:"retry_codes"`
		Empty      []string `yaml:"empty"`
	}
	type Route struct {
		Name     string            `yaml:"name"`
		Endpoint string            `yaml:"endpoint"`
		Methods  []string          `yaml:"methods"`
		Headers  map[string]string `yaml:"headers"`
		Flags    Flags             `yaml:"flags"`
	}
	type Edge struct {
		Routes []Route `yaml:"routes"`
	}
	type Config struct {
		Edge Edge `yaml:"edge"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(context.Background(), content, &cfg); err != nil {
		t.Fatal(err)
	}

	want := Config{
		Edge: Edge{
			Routes: []Route{
				{
					Name:     "order:create#v1",
					Endpoint: "https://api.example.com/a,b?token='x:y#z'",
					Methods:  []string{"GET", "POST,JSON", "on", "*"},
					Headers: map[string]string{
						"x-token":    "abc,123:xyz#hash",
						"x-star":     "*",
						"x-anchor":   "&anchor",
						"x-tag":      "!tag",
						"x-brackets": "[a,b]{c:d}",
					},
					Flags: Flags{
						Enabled:    true,
						Ratio:      0.875,
						RetryCodes: []int{500, 502, 503},
						Empty:      []string{},
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg mismatch\nwant: %+v\n got: %+v", want, cfg)
	}
}

func TestUnmarshalEdgePipelineBlockScalars(t *testing.T) {
	content := []byte(`pipelines:
  - name: release@prod
    stages:
      - name: build:test
        env:
          GOFLAGS: "-mod=mod -tags='netgo,prod'"
          STAR: "*"
          HASH: "value#still"
        scripts:
          - |
            echo "build: start"
            go test ./...
          - >-
            deploy to
            prod cluster
      - name: notify
        env: {}
        scripts:
          - !!str |
            curl -H 'X-Token: a,b#c' https://hook.local/send
`)

	type Stage struct {
		Name    string            `yaml:"name"`
		Env     map[string]string `yaml:"env"`
		Scripts []string          `yaml:"scripts"`
	}
	type Pipeline struct {
		Name   string  `yaml:"name"`
		Stages []Stage `yaml:"stages"`
	}
	type Config struct {
		Pipelines []Pipeline `yaml:"pipelines"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(context.Background(), content, &cfg); err != nil {
		t.Fatal(err)
	}

	want := Config{
		Pipelines: []Pipeline{
			{
				Name: "release@prod",
				Stages: []Stage{
					{
						Name: "build:test",
						Env: map[string]string{
							"GOFLAGS": "-mod=mod -tags='netgo,prod'",
							"STAR":    "*",
							"HASH":    "value#still",
						},
						Scripts: []string{
							"echo \"build: start\"\ngo test ./...\n",
							"deploy to prod cluster",
						},
					},
					{
						Name: "notify",
						Env:  map[string]string{},
						Scripts: []string{
							"curl -H 'X-Token: a,b#c' https://hook.local/send\n",
						},
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg mismatch\nwant: %+v\n got: %+v", want, cfg)
	}
}

func TestUnmarshalEdgeQuotedKeysAndNestedFlowGraph(t *testing.T) {
	content := []byte(`quoted_registry:
  "host:port": ["127.0.0.1:8080"]
  'comma,key#1': ["v:1", "x,y", "#hash", "&amp", "!bang"]
flow_graph:
  topology: {name: "core{edge}:v2", nodes: [{id: n1, labels: ["primary", "zone:a"]}, {id: n2, labels: []}], meta: {owner: "team,a#b", pattern: "a:b,c#d"}}
`)

	type Node struct {
		ID     string   `yaml:"id"`
		Labels []string `yaml:"labels"`
	}
	type Topology struct {
		Name  string            `yaml:"name"`
		Nodes []Node            `yaml:"nodes"`
		Meta  map[string]string `yaml:"meta"`
	}
	type FlowGraph struct {
		Topology Topology `yaml:"topology"`
	}
	type Config struct {
		QuotedRegistry map[string][]string `yaml:"quoted_registry"`
		FlowGraph      FlowGraph           `yaml:"flow_graph"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(context.Background(), content, &cfg); err != nil {
		t.Fatal(err)
	}

	want := Config{
		QuotedRegistry: map[string][]string{
			"host:port":   {"127.0.0.1:8080"},
			"comma,key#1": {"v:1", "x,y", "#hash", "&amp", "!bang"},
		},
		FlowGraph: FlowGraph{
			Topology: Topology{
				Name: "core{edge}:v2",
				Nodes: []Node{
					{ID: "n1", Labels: []string{"primary", "zone:a"}},
					{ID: "n2", Labels: []string{}},
				},
				Meta: map[string]string{
					"owner":   "team,a#b",
					"pattern": "a:b,c#d",
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg mismatch\nwant: %+v\n got: %+v", want, cfg)
	}
}
