package token

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/token"
)

func TestTokenPack(t *testing.T) {
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
	ctx := context.Background()
	tk.Pack(ctx)
}

func TestTokenUnPack(t *testing.T) {
	//bdata := "_-8Bv3MATgug4-mcVb9CvHsuhVd0ti4kIq-eZCzZgZ4yPkIky9O0IhmtY6pqnA"
	bdata := "CgAAAN+GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc+PNHbkhMYKJ"
	ctx := context.Background()
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	err := tk.UnPack(ctx, bdata)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		t.Fatal(err)
	}
	logger.Debugf(ctx, "tk: %v", tk)
}

func TestTokenReplace(t *testing.T) {
	ctx := context.Background()
	//a := "/+8B45pAJipAlhkEdKqdZVS2kcUx5ziMGExEoA/VPtAzqz9iYflFRhf9mvhMQw=="
	a := "/+8BDk4AcIFWE44elu94L52nGmLSjYHP2xnWu9dcW8Ngp1K1msVLQRK7wK26Fw=="
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	b := tk.PackReplace(ctx, a)

	logger.Debugf(ctx, "b: %s len: %d", b, len(b))

	a = tk.UnpackReplace(ctx, b)
	logger.Debugf(ctx, "a: %s len: %d", a, len(a))
}

func TestUnpackReplace(t *testing.T) {
	ctx := context.Background()
	a := "CgAAAN-GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc-PNHbkhMYKJ"
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}

	b := tk.UnpackReplace(ctx, a)
	logger.Debugf(ctx, "b: %s", b)
}

func TestUnpackOld(t *testing.T) {
	ctx := context.Background()
	a := "CgAAAN-GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc-PNHbkhMYKJ"
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	tk.UnPackOld(ctx, a)

}
