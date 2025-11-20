package util

import (
	"testing"
)

func TestSnowFlake(t *testing.T) {
	sf, _ := NewSnowflake(1, 1)
	id, _ := sf.NextID()
	t.Log(id)
}
