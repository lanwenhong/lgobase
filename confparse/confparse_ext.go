package confparse

import (
	"context"

	config "github.com/lanwenhong/lgobase/gconfig"
)

func ParseExt(ctx context.Context, section string, key string, index int, cfg *config.Gconf) ([]string, error) {
	qk := key + " = " + cfg.Gcf[section][key][index]
	return cfg.GlineExtend[key][qk], nil
}
