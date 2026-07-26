package gconfig_v2_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalConfig11PayServers(t *testing.T) {
	type PayServer struct {
		Addr         string `yaml:"addr"`
		Rule         string `yaml:"rule"`
		Salience     int64  `yaml:"Salience"`
		MaxConns     int64  `yaml:"MaxConns"`
		MaxIdleConns int64  `yaml:"MaxIdleConns"`
		MaxConnLife  int64  `yaml:"MaxConnLife"`
		Proto        string `yaml:"proto"`
	}
	type Config struct {
		PayServers []PayServer `yaml:"payservers"`
	}

	var cfg Config
	if err := gconfig_v2.UnmarshalFile(context.Background(), "config11.yaml", &cfg); err != nil {
		t.Fatal(err)
	}

	want := Config{
		PayServers: []PayServer{
			{
				Addr:         "192.168.100.103/1000",
				Rule:         `trade.busicd == "1000" && trade.txamt == 2000`,
				Salience:     16,
				MaxConns:     100,
				MaxIdleConns: 10,
				MaxConnLife:  3600,
				Proto:        "thrift_ext",
			},
			{
				Addr:         "192.168.100.106/1000",
				Rule:         `trade.busicd == "802801"`,
				Salience:     16,
				MaxConns:     100,
				MaxIdleConns: 10,
				MaxConnLife:  3600,
				Proto:        "thrift_ext",
			},
			{
				Addr:         "192.168.100.104/1000",
				Rule:         "1 == 1",
				Salience:     16,
				MaxConns:     100,
				MaxIdleConns: 10,
				MaxConnLife:  3600,
				Proto:        "thrift_ext",
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg mismatch\nwant: %+v\n got: %+v", want, cfg)
	}
}
