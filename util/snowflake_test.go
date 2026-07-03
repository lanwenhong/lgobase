package util_test

import (
	"testing"

	"github.com/lanwenhong/lgobase/util"
)

func TestSnowFlake(t *testing.T) {
	sf, _ := util.NewSnowflake(1, 1)
	id, _ := sf.NextID()
	t.Log(id)
}
