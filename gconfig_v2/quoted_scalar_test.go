package gconfig_v2_test

import (
	"context"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalQuotedScalarsRemainStrings(t *testing.T) {
	type AmexConf struct {
		Host                  string `yaml:"host"`
		Port                  int    `yaml:"port"`
		APIVersion            string `yaml:"api_version"`
		MethodNotifyURL       string `yaml:"method_notify_url"`
		MethodTokenTTLSeconds int    `yaml:"method_token_ttl_seconds"`
	}

	content := []byte(`
host: api.example.com
port: 443
api_version: "100"
method_notify_url: 'https://example.com/notify'
method_token_ttl_seconds: 300
`)
	var cfg AmexConf
	if err := gconfig_v2.Unmarshal(context.Background(), content, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.APIVersion != "100" {
		t.Fatalf("cfg.APIVersion = %q, want %q", cfg.APIVersion, "100")
	}
	if cfg.Port != 443 || cfg.MethodTokenTTLSeconds != 300 {
		t.Fatalf("unquoted integers were not inferred: %+v", cfg)
	}

	var values map[string]any
	content = []byte(`
quoted_int: "100"
quoted_bool: 'true'
quoted_float: "1.25"
quoted_time: "2026-07-20"
unquoted_int: 100
unquoted_bool: true
unquoted_float: 1.25
unquoted_time: 2026-07-20
plain_string: v1.0.0
`)
	if err := gconfig_v2.Unmarshal(context.Background(), content, &values); err != nil {
		t.Fatal(err)
	}

	for key, want := range map[string]string{
		"quoted_int":   "100",
		"quoted_bool":  "true",
		"quoted_float": "1.25",
		"quoted_time":  "2026-07-20",
		"plain_string": "v1.0.0",
	} {
		got, ok := values[key].(string)
		if !ok || got != want {
			t.Fatalf("%s = %#v (%T), want string %q", key, values[key], values[key], want)
		}
	}
	if got, ok := values["unquoted_int"].(int); !ok || got != 100 {
		t.Fatalf("unquoted_int = %#v (%T), want int 100", values["unquoted_int"], values["unquoted_int"])
	}
	if got, ok := values["unquoted_bool"].(bool); !ok || !got {
		t.Fatalf("unquoted_bool = %#v (%T), want bool true", values["unquoted_bool"], values["unquoted_bool"])
	}
	if got, ok := values["unquoted_float"].(float64); !ok || got != 1.25 {
		t.Fatalf("unquoted_float = %#v (%T), want float64 1.25", values["unquoted_float"], values["unquoted_float"])
	}
	if _, ok := values["unquoted_time"].(time.Time); !ok {
		t.Fatalf("unquoted_time = %#v (%T), want time.Time", values["unquoted_time"], values["unquoted_time"])
	}
}
