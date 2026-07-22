package token_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/token"
)

func TestTokenPack(t *testing.T) {
	ctx := context.Background()
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

	bytes, _ := json.Marshal(tk)
	logger.Debug(ctx, "token test replacement result", "value", string(bytes))
	tk.Pack(ctx)
}

func TestTokenUnPack(t *testing.T) {
	//bdata := "____7wFCigBmgqBijZpsRTZZGAFEl5ew4A5tAoeoNrKO8UyoFHEWEHTcfMHXJciN"
	//bdata := "____7wHv9AA_g1FizA_i5S9-HJc02Qe4efAwSO2LUpO-uplzI3dB7VubFgnSW2TU"
	//bdata := "MC8_7wGjMQAECuy0LQG1hQiPvrMdpKeGc38c_j3DRSYnJjxZ_gJNGQ9X5J-zsUuC"
	bdata := "MC8_7wAJ6gAPYR2BiP869Bqb3SWv8eeWNbocq3OfU624ckkEiO7a3WMztqXu-1C-"
	ctx := context.Background()
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	err := tk.UnPack(ctx, bdata)
	if err != nil {
		logger.Warn(ctx, "token test unpack failed", "err", err)
		t.Fatal(err)
	}
	logger.Debug(ctx, "token test unpack result", "token", tk)
}

func TestTokenReplace(t *testing.T) {
	ctx := context.Background()
	//a := "/+8B45pAJipAlhkEdKqdZVS2kcUx5ziMGExEoA/VPtAzqz9iYflFRhf9mvhMQw=="
	a := "/+8BDk4AcIFWE44elu94L52nGmLSjYHP2xnWu9dcW8Ngp1K1msVLQRK7wK26Fw=="
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	b := tk.PackReplace(ctx, a)

	logger.Debug(ctx, "token test packed value", "value", b, "length", len(b))

	a = tk.UnpackReplace(ctx, b)
	logger.Debug(ctx, "token test unpacked value", "value", a, "length", len(a))
}

func TestUnpackReplace(t *testing.T) {
	ctx := context.Background()
	a := "CgAAAN-GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc-PNHbkhMYKJ"
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}

	b := tk.UnpackReplace(ctx, a)
	logger.Debug(ctx, "token test packed value", "value", b)
}

func TestUnpackOld(t *testing.T) {
	ctx := context.Background()
	//a := "CgAAAN-GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc-PNHbkhMYKJ"
	a := "CgAAAFtcGXQcKlXIBlQuqCe7Fn98bPN45iixZoWh5lNiqt6U"
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	tk.UnPackOld(ctx, a)

}
