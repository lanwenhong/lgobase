package token

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func (tk *Token) UnPackOld(ctx context.Context, bdata string) error {
	if len(bdata) > 256 {
		return errors.New("session data is too long")
	}

	bdata = tk.UnpackReplace(ctx, bdata)
	aescbc := util.AesCbc{}
	decrypted, err := aescbc.AESDecryptCBCSb(ctx, bdata, []byte(tk.Tkey))
	if err != nil {
		logger.Warnf(ctx, "aes dec err: %s", err.Error())
		return err
	}

	ver := uint8(decrypted[0])
	//logger.Debugf(ctx, "ver: %d", ver)
	tk.Ver = uint32(ver)

	tk.Idc = uint8(decrypted[1])
	//logger.Debugf(ctx, "idc: %d", idc)
	tk.FlagUC = string(decrypted[2])
	//logger.Debugf(ctx, "flag: %d", flag)
	//uid := binary.BigEndian.Uint32(decrypted[3:7])
	tk.Uid = uint64(binary.LittleEndian.Uint32(decrypted[3:7]))
	//logger.Debugf(ctx, "uid: %d", uid)
	tk.OpUid = binary.LittleEndian.Uint16(decrypted[7:9])
	//logger.Debugf(ctx, "opuid: %d", opuid)
	tk.Expire = binary.LittleEndian.Uint32(decrypted[9:13])
	//logger.Debugf(ctx, "expire: %d", expire)
	tk.Deadline = uint64(binary.LittleEndian.Uint32(decrypted[13:17]))
	//logger.Debugf(ctx, "deadline: %d", deadline)
	udid_len := uint8(decrypted[17])
	logger.Debugf(ctx, "udid_len: %d", udid_len)
	bUlen := len(decrypted[18:])
	udid := ""
	if bUlen >= int(udid_len) {
		udid = string(decrypted[18 : 18+udid_len])
	} else {
		udid = string(decrypted[18:])
	}
	//udid := string(decrypted[18:])
	//logger.Debugf(ctx, "udid:%s", udid)
	tk.Udid = udid
	logger.Debugf(ctx, "tk: %v", tk)
	return nil
}
