package token

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

const (
	TOKEN_HEADER_MAGIC = uint32(0xEF3F2F30)
)

/*
|version(4byte)|mac(8byte)|flag_uc(1byte)|uid(4byte or 8byte)|opuid(2byte)|expire(4byte)|deadline(4byte or 8byte)|udid(21byte)
*/
type Token struct {
	//header
	Magic  uint32 `json:"-"`
	Ver    uint32 `json:"-"` //第32bit位标记（uid64或32）第31bit位标记（deadline 64或32） 第25bit-第30bit（idc标记）2byte-3byte(随机数）1byte（version）
	Mac    uint64 `json:"-"` //8字节校验mac
	Idc    uint8  `json:"-"` //idc机房标记， 最高支持0-31
	FlagUC string `json:"-"` //1byte区分存储的是uid还是customer_id

	//body
	Uid       uint64 `json:"userid,omitempty"`   //8字节Uid
	OpUid     uint16 `json:"opuid,omitempty"`    //2字节opuid
	Expire    uint32 `json:"__expi__,omitempty"` //4byte过期时间
	Deadline  uint64 `json:"-"`                  //过期时间戳
	Udid      string `json:"udid,omitempty"`     //21byte
	Tkey      string `json:"-"`                  //机密key
	Del       int    `json:"__del__,omitempty"`
	CustomeId uint64 `json:"customer_id,omitempty"`
}

func (tk *Token) PackReplace(ctx context.Context, src string) string {
	b := []byte(src)
	n := 0
	for i, _ := range b {
		switch b[i] {
		case '+':
			b[i] = '-'
		case '/':
			b[i] = '_'
		case '=':
			b[i] = 0x00
			n++
		}
	}
	return string(b[:len(b)-n])
}

func (k *Token) UnpackReplace(ctx context.Context, src string) string {
	b := []byte(src)
	for i, _ := range b {
		switch b[i] {
		case '-':
			b[i] = '+'
		case '_':
			b[i] = '/'
		}
	}
	if len(b)%4 != 0 {
		padding := 4 - len(b)%4
		for i := 0; i < padding; i++ {
			b = append(b, "="...)
		}
	}
	return string(b)
}

func (tk *Token) xor8Bytes(ctx context.Context, a, b []byte) ([]byte, error) {
	if len(a) != 8 || len(b) != 8 {
		return nil, errors.New("输入必须是8字节切片")
	}

	result := make([]byte, 8)
	for i := 0; i < 8; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result, nil
}

func (tk *Token) generateRandom2Bytes(ctx context.Context) []byte {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, 2)
	// 生成两个随机字节（0-255）
	b[0] = byte(rand.Intn(256))
	b[1] = byte(rand.Intn(256))
	return b
}

func (tk *Token) TkMac(ctx context.Context, data []byte, randk []byte, iv []byte) ([]byte, error) {
	data_len := len(data)
	mdata := make([]byte, data_len)
	mdata = append(mdata, data...)
	padding_len := 8 - data_len%8
	logger.Debugf(ctx, "data: %v len: %d padding_len: %d", data, data_len, padding_len)
	for i := 0; i < padding_len; i++ {
		mdata = append(mdata, []byte{0xFF}...)
	}
	if len(data) == 8 {
		return data, nil
	}
	/*tmp, _ := tk.xor8Bytes(data[0:8], data[8:16])
	for j := 2; j < padding_len; j++ {
		tmp = tk.xor8Bytes(tmp, data[j])
	}*/
	j := 8
	var tmp []byte
	tmp = mdata[0:8]
	for {
		if j >= padding_len {
			break
		}
		tmp, _ = tk.xor8Bytes(ctx, tmp, mdata[j:j+8])
		j += 8
	}
	aescbc := util.AesCbc{}
	ret, err := aescbc.AESEncryptCBCWithNoBase64(ctx, tmp, randk, iv)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		return ret, err
	}
	logger.Debugf(ctx, "ret: %v", ret)
	mac, _ := tk.xor8Bytes(ctx, ret[0:8], ret[8:16])
	return mac, nil
}

func (tk *Token) UnPack(ctx context.Context, bdata string) error {
	var macBuf bytes.Buffer
	bdata = tk.UnpackReplace(ctx, bdata)
	//iv := []byte("llss-------token")
	//iv := make([]byte, 0, 16)
	iv := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	data, err := base64.StdEncoding.DecodeString(bdata)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		return err
	}

	if len(data) < 18 {
		return errors.New("token format len error")
	}

	//parse version
	b_magic := data[0:4]
	tk.Magic = binary.LittleEndian.Uint32(b_magic)
	logger.Debugf(ctx, "magic: %d", tk.Magic)
	if tk.Magic != TOKEN_HEADER_MAGIC {
		return tk.UnPackOld(ctx, bdata)
	}

	b_ver := data[4:8]
	ver := binary.LittleEndian.Uint32(b_ver)
	logger.Debugf(ctx, "ver: %d", ver)
	uidBit := ver>>31 | 0x00
	logger.Debugf(ctx, "uidBit: %d", uidBit)
	dlBit := ver >> 30 & 0x01
	logger.Debugf(ctx, "dlBit: %d", dlBit)
	//parse randB
	rt := (ver & uint32(0x00FFFF00)) >> 8
	rt16 := uint16(rt)
	randB := make([]byte, 2)
	binary.LittleEndian.PutUint16(randB, rt16)
	logger.Debugf(ctx, "randB: %v", randB)

	//mac := data[4:12]
	mac := data[8:16]
	logger.Debugf(ctx, "mac: %v", mac)

	//randk
	aescbc := util.AesCbc{}
	/*iv = append(iv, randB...)
	for i := 0; i < 14; i++ {
		iv = append(iv, 0xFF)
	}*/
	iv[0] = randB[0]
	iv[1] = randB[1]
	gkey, err := aescbc.AESEncryptCBC(ctx, randB, []byte(tk.Tkey), iv)
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return err
	}
	logger.Debugf(ctx, "gkey: %s", gkey)

	randk := []byte{}
	k1 := []byte(gkey)
	k2 := []byte(gkey[0:8])
	randk = append(randk, k1...)
	randk = append(randk, k2...)
	logger.Debugf(ctx, "randk: %v", randk)

	//tkBody := data[12:]
	tkBody := data[16:]
	logger.Debugf(ctx, "tkBody: %v", tkBody)
	tkSrc, err := aescbc.AESDecryptCBCWithNoBase64(ctx, tkBody, randk, iv)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		return err
	}
	logger.Debugf(ctx, "tkSrc: %v", tkSrc)
	macBuf.Write(b_ver)
	macBuf.Write(tkSrc)
	//in_mac, _ := tk.TkMac(ctx, tkSrc, randk, iv)
	in_mac, _ := tk.TkMac(ctx, macBuf.Bytes(), randk, iv)
	logger.Debugf(ctx, "mac: %v in_mac: %v", mac, in_mac)

	int_mac := binary.LittleEndian.Uint64(mac)
	int_in_mac := binary.LittleEndian.Uint64(in_mac)

	if int_mac == int_in_mac {
		logger.Debugf(ctx, "mac check succ")
	} else {
		logger.Warnf(ctx, "mac error")
		return errors.New("mac error")
	}
	//unpack idc
	start := 0
	end := 0
	logger.Debugf(ctx, "bidc: %v", tkSrc[start])
	tk.Idc = tkSrc[start]
	start += 1
	end += 1
	logger.Debugf(ctx, "idc:  %d", tk.Idc)
	//unpack flag_uc
	tk.FlagUC = string(tkSrc[start])
	start += 1
	end += 1
	logger.Debugf(ctx, "flag_uc: %s", tk.FlagUC)
	//unpack uid
	if uidBit == 0 {
		//end = 4
		start = end
		end += 4
		buid := tkSrc[start:end]
		logger.Debugf(ctx, "buid: %v", buid)
		uid32 := binary.LittleEndian.Uint32(buid)
		tk.Uid = uint64(uid32)
		logger.Debugf(ctx, "uid: %d", uid32)

	} else if uidBit == 1 {
		//end = 8
		start = end
		end += 8
		buid := tkSrc[0:end]
		uid64 := binary.LittleEndian.Uint64(buid)
		tk.Uid = uid64
		logger.Debugf(ctx, "uid: %d", uid64)
	}
	//unpack opuid
	start = end
	end += 2
	b_opuid := tkSrc[start:end]
	tk.OpUid = binary.LittleEndian.Uint16(b_opuid)
	logger.Debugf(ctx, "opuid: %d", tk.OpUid)

	//unpack expire
	start = end
	end += 4
	b_expire := tkSrc[start:end]
	logger.Debugf(ctx, "b_expire: %v", b_expire)
	tk.Expire = binary.LittleEndian.Uint32(b_expire)
	logger.Debugf(ctx, "expire: %d", tk.Expire)

	//unpack deadline
	if dlBit == 0 {
		start = end
		end += 4
		b_deadline := tkSrc[start:end]
		logger.Debugf(ctx, "b_deadline: %v", b_deadline)
		deadline32 := binary.LittleEndian.Uint32(b_deadline)
		tk.Deadline = uint64(deadline32)

	} else if dlBit == 1 {
		start = end
		end += 8
		b_deadline := tkSrc[start:end]
		logger.Debugf(ctx, "b_deadline: %v", b_deadline)
		deadline64 := binary.LittleEndian.Uint64(b_deadline)
		tk.Deadline = uint64(deadline64)

	}
	logger.Debugf(ctx, "deadline: %d", tk.Deadline)

	//unpack udid
	start = end
	end += 1
	b_udidlen := uint8(tkSrc[start])
	logger.Debugf(ctx, "b_udidlen: %d", b_udidlen)
	start = end
	budid := tkSrc[start : start+int(b_udidlen)]
	logger.Debugf(ctx, "budid: %v", budid)
	tk.Udid = string(budid)
	logger.Debugf(ctx, "udid: %s", tk.Udid)
	return nil
}

func (tk *Token) Pack(ctx context.Context) (string, error) {
	//tkSrc := []byte{}
	//tkEnc := []byte{}
	tkSrc := make([]byte, 0, 1024)
	tkEnc := make([]byte, 0, 1024)
	randk := make([]byte, 0, 128)
	var macBuf bytes.Buffer
	//iv := []byte("llss-------token")
	//iv := make([]byte, 0, 16)
	iv := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	//header pack
	//body pack
	//pack idc
	bIdc := []byte{tk.Idc}
	//binary.LittleEndian.PutUint8(bIdc, tk.Idc)
	tkSrc = append(tkSrc, bIdc...)
	//pack flag_uc
	uc := []byte(tk.FlagUC)
	logger.Debugf(ctx, "uc: %v", uc)
	tkSrc = append(tkSrc, uc...)

	//pack uid
	if tk.Uid > 0xFFFFFFFF {
		b_uid := make([]byte, 8)
		binary.LittleEndian.PutUint64(b_uid, tk.Uid)
		tkSrc = append(tkSrc, b_uid...)
	} else {
		new_uid := uint32(tk.Uid)
		b_uid := make([]byte, 4)
		binary.LittleEndian.PutUint32(b_uid, new_uid)
		tkSrc = append(tkSrc, b_uid...)
	}

	//pack opuid
	b_opuid := make([]byte, 2)
	binary.LittleEndian.PutUint16(b_opuid, tk.OpUid)
	tkSrc = append(tkSrc, b_opuid...)

	//pack expire
	b_expire := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_expire, tk.Expire)
	logger.Debugf(ctx, "b_expire: %v", b_expire)
	tkSrc = append(tkSrc, b_expire...)

	//pack deadline
	if tk.Deadline > 0xFFFFFFFF {
		b_deadline := make([]byte, 8)
		binary.LittleEndian.PutUint64(b_deadline, tk.Deadline)
		tkSrc = append(tkSrc, b_deadline...)
	} else {
		new_deadline := uint32(tk.Deadline)
		b_deadline := make([]byte, 4)
		binary.LittleEndian.PutUint32(b_deadline, new_deadline)
		tkSrc = append(tkSrc, b_deadline...)
	}
	//pack udid
	ulen := uint8(len(tk.Udid))
	if ulen > 21 {
		ulen = 21
	}
	udid := tk.Udid[0:ulen]
	b_udid_len := []byte{ulen}
	tkSrc = append(tkSrc, b_udid_len...)
	tkSrc = append(tkSrc, udid...)

	//gen mackey
	randB := tk.generateRandom2Bytes(ctx)
	logger.Debugf(ctx, "randB: %v len: %d", randB, len(randB))
	//randB = []byte{0x39, 0x39}

	//pack header
	//pack magic
	bMagic := make([]byte, 4)
	binary.LittleEndian.PutUint32(bMagic, TOKEN_HEADER_MAGIC)
	tkEnc = append(tkEnc, bMagic...)
	//pack version
	if tk.Uid > 0xFFFFFFFF {
		uidBit := uint32(0x01)
		uidBit = uidBit << 31
		tk.Ver = tk.Ver | uidBit
	}
	if tk.Deadline > 0xFFFFFFFF {
		dlBit := uint32(0x01)
		dlBit = dlBit << 30
		tk.Ver = tk.Ver | dlBit
	}
	randInt := binary.LittleEndian.Uint16(randB)
	tk.Ver = tk.Ver | uint32(randInt)<<8

	b_ver := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_ver, tk.Ver)
	tkEnc = append(tkEnc, b_ver...)

	aescbc := util.AesCbc{}
	/*iv = append(iv, randB...)
	for i := 0; i < 14; i++ {
		iv = append(iv, 0xFF)
	}*/
	iv[0] = randB[0]
	iv[1] = randB[1]
	gkey, err := aescbc.AESEncryptCBC(ctx, randB, []byte(tk.Tkey), iv)
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return "", err
	}
	logger.Debugf(ctx, "gkey: %s", gkey)

	k1 := []byte(gkey)
	k2 := []byte(gkey[0:8])
	randk = append(randk, k1...)
	randk = append(randk, k2...)

	logger.Debugf(ctx, "randk: %v", randk)
	logger.Debugf(ctx, "tkSrc: %v", tkSrc)
	macBuf.Write(b_ver)
	macBuf.Write(tkSrc)
	//mac, _ := tk.TkMac(ctx, tkSrc, randk, iv)
	mac, _ := tk.TkMac(ctx, macBuf.Bytes(), randk, iv)
	logger.Debugf(ctx, "mac: %v", mac)
	tkEnc = append(tkEnc, mac...)

	logger.Debugf(ctx, "tkSrc: %v", tkSrc)
	tkBody, err := aescbc.AESEncryptCBCWithNoBase64(ctx, tkSrc, randk, iv)
	logger.Debugf(ctx, "tkBody: %v", tkBody)
	tkEnc = append(tkEnc, tkBody...)
	ciphertextBase64 := base64.StdEncoding.EncodeToString(tkEnc)
	logger.Debugf(ctx, "ciphertextBase64: %s", ciphertextBase64)
	token := tk.PackReplace(ctx, ciphertextBase64)
	logger.Debugf(ctx, "token: %s", token)
	return token, nil

}
