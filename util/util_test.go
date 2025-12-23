package util

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func TestGenid(t *testing.T) {
	id := util.GenXid()
	t.Log(id)
	id = util.GenKsuid()
	t.Log(id)

	id = util.GenBetterGUID()
	t.Log(id)

	id = util.GenUlid()
	t.Log(id)

	id = util.GenSonyflake()
	t.Log(id)

	id = util.GenSid()
	t.Log(id)

	s, _ := util.GenerateSecureRandomString(20)
	t.Log(s)

	id = util.GenerateUniqueStringWithTimestamp("id")
	t.Log(id)

}

func TestAesCbc(t *testing.T) {
	ctx := context.Background()
	aescbc := util.AesCbc{}
	key := []byte("11111111111111111111111111111111")
	//key := []byte("8888888888888888AAAAAAAAAAAAAAAAFFFFFFFFFFFFFFFF1111111111111111")
	plaintext := []byte("111111111111111111111111111111111111111111111111")
	//plaintext := []byte{117, 1, 0, 0, 0, 10, 0, 10, 0, 0, 0, 255, 255, 255, 255, 15, 0, 0, 0, 1, 48}
	//iv := make([]byte, aes.BlockSize)
	//iv := []byte("'qfpay-----token")
	iv := []byte("llss-------token")
	// 加密
	ciphertextBase64, err := aescbc.AESEncryptCBC(ctx, plaintext, key, iv)
	if err != nil {
		fmt.Println("Error encrypting:", err)
		return
	}
	fmt.Printf("加密后(Base64): %s\n", ciphertextBase64)

	// 解密
	decrypted, err := aescbc.AESDecryptCBC(ctx, ciphertextBase64, key, iv)
	if err != nil {
		fmt.Println("Error decrypting:", err)
		return
	}
	fmt.Printf("解密后: %s len: %d\n", string(decrypted), len(decrypted))
}

func TestAesCbcWithNoB64(t *testing.T) {
	ctx := context.Background()
	aescbc := util.AesCbc{}
	//key := []byte("11111111111111111111111111111111")
	//key := []byte{68, 87, 80, 80, 47, 112, 77, 57, 51, 86, 112, 67, 48, 74, 90, 90, 82, 106, 47, 69, 118, 103, 61, 61, 68, 87, 80, 80, 47, 112, 77, 57}
	key := []byte{78, 102, 68, 120, 72, 112, 117, 77, 120, 112, 54, 57, 97, 49, 108, 82, 82, 79, 53, 81, 50, 103, 61, 61, 78, 102, 68, 120, 72, 112, 117, 77}
	fmt.Printf("key: %v\n", key)
	//plaintext := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	plaintext := []byte{117, 1, 0, 0, 0, 10, 0, 10, 0, 0, 0, 255, 255, 255, 255, 15, 0, 0, 0, 1, 48}
	//iv := make([]byte, aes.BlockSize)
	//iv := []byte("'qfpay-----token")
	iv := []byte("llss-------token")
	// 加密
	ciphertext, err := aescbc.AESEncryptCBCWithNoBase64(ctx, plaintext, key, iv)
	if err != nil {
		fmt.Println("Error encrypting:", err)
		return
	}
	fmt.Printf("加密后(Base64): %v\n", ciphertext)
	//ciphertext = []byte{87, 180, 206, 244, 69, 224, 210, 178, 62, 90, 23, 129, 222, 132, 35, 34, 173, 112, 230, 44, 195, 114, 15, 204, 28, 158, 89, 140, 207, 133, 226, 97}

	// 解密
	decrypted, err := aescbc.AESDecryptCBCWithNoBase64(ctx, ciphertext, key, iv)
	if err != nil {
		fmt.Println("Error decrypting:", err)
		return
	}
	/*if string(decrypted) == string(plaintext) {
		fmt.Printf("succ\n")
	}
	fmt.Printf("解密后: %s\n", string(decrypted))*/
	fmt.Printf("%v\n", decrypted)

}

func TestTokenDec(t *testing.T) {
	ctx := context.Background()
	aes := util.AesCbc{}
	//token := "CgAAAHSJ7px6VggEx4eVVFGn4p151HI8pONVLuogox9+Fm9q"
	//token := "CgAAANinWXRZdg6b+kO1c+hCUgP2KxVUWwxxQkR9OR0wpf1h"
	//token := "CgAAAFKlK/BV/KsMFva5t9dmD2fugmfsoc3yK9wIiqV+CEwX"
	//token := "CgAAAN+GTvV4QPdOsGRa5NhGx5vDXcwWqpYuc+PNHbkhMYKJ"
	//token := "CgAAAK725CK5we3ucxp6/DyJUH2nmtbGC9FEsEZaFeoSRadM"

	key := []byte("IypMcRkPXkbeNDRl6Km43boHr98udp7o")
	//key := []byte("11111111111111111111111111111111")
	decrypted, err := aes.AESDecryptCBCSb(ctx, token, key)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		return
	}
	logger.Debugf(ctx, "%v", decrypted)
	ver := uint8(decrypted[0])
	logger.Debugf(ctx, "ver: %d", ver)
	idc := uint8(decrypted[1])
	logger.Debugf(ctx, "idc: %d", idc)
	flag := uint8(decrypted[2])
	logger.Debugf(ctx, "flag: %d", flag)
	//uid := binary.BigEndian.Uint32(decrypted[3:7])
	uid := binary.LittleEndian.Uint32(decrypted[3:7])
	logger.Debugf(ctx, "uid: %d", uid)
	opuid := binary.LittleEndian.Uint16(decrypted[7:9])
	logger.Debugf(ctx, "opuid: %d", opuid)
	expire := binary.LittleEndian.Uint32(decrypted[9:13])
	logger.Debugf(ctx, "expire: %d", expire)
	deadline := binary.LittleEndian.Uint32(decrypted[13:17])
	logger.Debugf(ctx, "deadline: %d", deadline)
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
	logger.Debugf(ctx, "udid:%s", udid)
}

func TestPackToken(t *testing.T) {
	ctx := context.Background()
	randStr := "dx"
	tkSrc := []byte{}
	tkEnc := []byte{}
	randKey := []byte("IypMcRkPXkbeNDRl6Km43boHr98udp7o")
	iv := []byte("'qfpay-----token")

	//uid_flag64 := 0x80000000

	var idc uint8 = 22
	b_idc := []byte{idc}
	tkSrc = append(tkSrc, b_idc...)

	flag := "u"
	tkSrc = append(tkSrc, flag...)

	//var uid uint64 = 66666666666
	var uid uint64 = 66666666
	if uid > 0xFFFFFFFF {
		logger.Debugf(ctx, "use uint64 pack")
		b_uid := make([]byte, 8)
		binary.LittleEndian.PutUint64(b_uid, uid)
		tkSrc = append(tkSrc, b_uid...)
	} else {
		logger.Debugf(ctx, "use uint32 pack")
		new_uid := uint32(uid)
		b_uid := make([]byte, 4)
		binary.LittleEndian.PutUint32(b_uid, new_uid)
		tkSrc = append(tkSrc, b_uid...)
	}

	var opuid uint16 = 16
	logger.Debugf(ctx, "use uint16 pack")
	b_uid := make([]byte, 16)
	binary.LittleEndian.PutUint16(b_uid, opuid)
	tkSrc = append(tkSrc, b_uid...)

	var expire uint32 = 2222
	b_expire := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_expire, expire)
	tkSrc = append(tkSrc, b_expire...)

	var deadline uint64 = 444444
	//var deadline uint64 = 44444444444
	if deadline > 0xFFFFFFFF {
		b_deadline := make([]byte, 8)
		binary.LittleEndian.PutUint64(b_deadline, deadline)
		tkSrc = append(tkSrc, b_deadline...)
	} else {
		new_deadline := uint32(deadline)
		b_deadline := make([]byte, 4)
		binary.LittleEndian.PutUint32(b_deadline, new_deadline)
		tkSrc = append(tkSrc, b_deadline...)
	}

	var udid_len uint8 = 14
	//b_udid_len := make([]byte, 1)
	//binary.LittleEndian.PutUint8(b_udid_len, udid_len)
	b_udid_len := []byte{udid_len}
	tkSrc = append(tkSrc, b_udid_len...)

	udid := "0"
	tkSrc = append(tkSrc, udid...)

	/*var mac uint32 = 33333
	b_mac := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_mac, mac)
	tkSrc = append(tkSrc, b_mac...)
	logger.Debugf(ctx, "tkSrc len: %d", len(tkSrc))*/

	aescbc := util.AesCbc{}
	//rand key
	gkey, err := aescbc.AESEncryptCBC(ctx, []byte(randStr), randKey, iv)
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return
	}
	logger.Debugf(ctx, "gkey: %s", gkey)

	k := []byte{}
	k1 := []byte(gkey)
	k2 := []byte(gkey[0:8])
	k = append(k, k1...)
	k = append(k, k2...)

	logger.Debugf(ctx, "k len: %d", len(k))
	//ciphertextBase64, err := aescbc.AESEncryptCBC(ctx, tkSrc, k, iv)
	//ciphertextBase64, err := aescbc.AESEncryptCBC(ctx, k, randKey, iv)

	logger.Debugf(ctx, "tkSrc len: %d", len(tkSrc))
	tkBody, err := aescbc.AESEncryptCBCWithNoBase64(ctx, tkSrc, k, iv)
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return
	}

	logger.Debugf(ctx, "body len: %d", len(tkBody))
	//header
	var ver uint32 = 1
	if uid > 0xFFFFFFFF {
		flag := uint32(0x01)
		flag = flag << 31
		logger.Debugf(ctx, "flag: %x", flag)
		ver = ver | flag
	}
	if deadline > 0xFFFFFFFF {
		flag := uint32(0x01)
		flag = flag << 30
		logger.Debugf(ctx, "flag: %x", flag)
		ver = ver | flag
	}
	logger.Debugf(ctx, "ver: %x", ver)

	b_ver := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_ver, ver)
	tkEnc = append(tkEnc, b_ver...)

	var mac uint64 = 100
	b_mac := make([]byte, 8)
	binary.LittleEndian.PutUint64(b_mac, mac)
	tkEnc = append(tkEnc, b_mac...)

	randInt := binary.LittleEndian.Uint16([]byte(randStr))
	logger.Debugf(ctx, "randInt: %x", randInt)
	ver = ver | (uint32(randInt) << 8)
	logger.Debugf(ctx, "ver: %x", ver)

	if 1 == 1 {
		rt := (ver & uint32(0x00FFFF00)) >> 8
		rt16 := uint16(rt)
		brt := make([]byte, 2)
		binary.LittleEndian.PutUint16(brt, rt16)
		srt := string(brt)
		logger.Debugf(ctx, "srt: %s", srt)
	}

	tkEnc = append(tkEnc, tkBody...)
	logger.Debugf(ctx, "tkEnc len: %d", len(tkEnc))

	ciphertextBase64 := base64.StdEncoding.EncodeToString(tkEnc)
	logger.Debugf(ctx, "ciphertextBase64: %s", ciphertextBase64)
}
