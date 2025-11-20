package util

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestCalcMac(t *testing.T) {
	key, _ := hex.DecodeString("25d630d897253a2b541f89f2eb820630")
	fmt.Println(CalcMac(key, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}))
}
