package gconfig_v2_test

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalConfig8Tags(t *testing.T) {
	type Log struct {
		Level  string `yaml:"level"`
		MaxAge int    `yaml:"max_age"`
	}
	type Redis struct {
		Addr     string  `yaml:"addr"`
		DB       int     `yaml:"db"`
		Password int     `yaml:"password"`
		Timeout  float64 `yaml:"timeout"`
	}
	type Server struct {
		Addr    string  `yaml:"addr"`
		DB      int     `yaml:"db"`
		Timeout float64 `yaml:"timeout"`
	}
	type Service struct {
		Name    string   `yaml:"name"`
		Port    int      `yaml:"port"`
		Tags    []string `yaml:"tags"`
		Redis   Redis    `yaml:"redis"`
		Servers []Server `yaml:"servers"`
	}
	type Config struct {
		Enable  bool      `yaml:"enable"`
		Ratio   float64   `yaml:"ratio"`
		Log     Log       `yaml:"log"`
		Service []Service `yaml:"service"`
	}

	var cfg Config
	if err := gconfig_v2.UnmarshalFile(context.Background(), "config8.yaml", &cfg); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Service) != 1 {
		t.Fatalf("len(cfg.Service) = %d", len(cfg.Service))
	}
	tags := cfg.Service[0].Tags
	if len(tags) != 2 || tags[0] != "web" || tags[1] != "api" {
		t.Fatalf("cfg.Service[0].Tags = %#v", tags)
	}
}
