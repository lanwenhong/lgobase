package gconfig_v2_test

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalServersConfig(t *testing.T) {
	content := []byte(`servers:
  - name: server1
    addr:
      - 127.0.0.1
      - 192.168.0.1
      - 10.0.0.1
    port: 6379

  - name: server2
    addr:
      - 192.168.0.2
      - 10.0.0.2
    port: 6380
`)

	type Server struct {
		Name string   `yaml:"name"`
		Addr []string `yaml:"addr"`
		Port int      `yaml:"port"`
	}
	type Config struct {
		Servers []Server `yaml:"servers"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(context.Background(), content, &cfg); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Servers) != 2 {
		t.Fatalf("len(cfg.Servers) = %d", len(cfg.Servers))
	}
	if cfg.Servers[0].Name != "server1" ||
		cfg.Servers[0].Port != 6379 ||
		len(cfg.Servers[0].Addr) != 3 ||
		cfg.Servers[0].Addr[0] != "127.0.0.1" ||
		cfg.Servers[0].Addr[1] != "192.168.0.1" ||
		cfg.Servers[0].Addr[2] != "10.0.0.1" {
		t.Fatalf("cfg.Servers[0] = %+v", cfg.Servers[0])
	}
	if cfg.Servers[1].Name != "server2" ||
		cfg.Servers[1].Port != 6380 ||
		len(cfg.Servers[1].Addr) != 2 ||
		cfg.Servers[1].Addr[0] != "192.168.0.2" ||
		cfg.Servers[1].Addr[1] != "10.0.0.2" {
		t.Fatalf("cfg.Servers[1] = %+v", cfg.Servers[1])
	}
}
