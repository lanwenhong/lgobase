package token

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/token"
)

func BenchmarkToken(b *testing.B) {
	ctx := context.Background()

	myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		//Loglevel:     logger.DEBUG,
		Loglevel: logger.INFO,
	}
	logger.Newglog("./", "test.log", "test.log.err", myconf)

	for n := 0; n < b.N; n++ {
		tk := &token.Token{
			Ver:    1,
			Idc:    2,
			FlagUC: "u",
			//Uid:      0xFFFFFFFFF,
			Uid:      1,
			OpUid:    10,
			Expire:   100000,
			Deadline: 0xFF,
			Udid:     "0",
			Tkey:     "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
		}

		a, _ := tk.Pack(ctx)
		//tk.Pack(ctx)
		utk := &token.Token{
			Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
		}

		utk.UnPack(ctx, a)
	}
}
