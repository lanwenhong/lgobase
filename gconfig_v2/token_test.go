package gconfig_v2_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestTokenParse(t *testing.T) {
	ctx := context.Background()
	tkn := gconfig_v2.NewTokenNode("", "", `{a: 111, b: 222, c:[1, 2, 3], d:{a: 1, b: 2}}`, 0)
	tkn.TokenParse(ctx)
	fmt.Println(tkn.Obj)
}
