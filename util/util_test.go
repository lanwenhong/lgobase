package util

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/lanwenhong/lgobase/dbenc"
	"github.com/lanwenhong/lgobase/dbpool"
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

}

func TestGenidFromDB(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", "1111")

	/*myconf := &logger.Glogconf{
		RotateMethod: logger.ROTATE_FILE_DAILY,
		Stdout:       true,
		Colorful:     true,
		Loglevel:     logger.DEBUG,
		//Goid:         true,
	}

	logger.Newglog("./", "test.log", "test.log.err", myconf)
	dconfig := &dlog.Config{
		SlowThreshold:             time.Second, // 慢 SQL 阈值
		LogLevel:                  dlog.Info,   // 日志级别
		IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		Colorful:                  true,        // 禁用彩色打印
	}*/

	db_conf := dbenc.DbConfNew(ctx, "/home/lanwenhong/dev/go/lgobase/dbpool/db.ini")
	dbs := dbpool.DbpoolNew(db_conf)
	//dbs.SetormLog(ctx, dconfig)
	tk := "qfconf://test1?maxopen=1000&maxidle=30"
	err := dbs.Add(ctx, "test1", tk, dbpool.USE_GORM)
	if err != nil {
		t.Fatal(err)
	}
	tdb := dbs.OrmPools["test1"]
	id, err := util.Genid(ctx, tdb)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(id)
}

func TestAesCbc(t *testing.T) {
	ctx := context.Background()
	aescbc := util.AesCbc{}
	key := []byte("11111111111111111111111111111111")
	plaintext := []byte("aaaaaaaaaaaaaaaaaaaaaaaa")
	//iv := make([]byte, aes.BlockSize)
	iv := []byte("'qfpay-----token")
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
	fmt.Printf("解密后: %s\n", string(decrypted))
}

func TestTokenDec(t *testing.T) {
	ctx := context.Background()
	aes := util.AesCbc{}
	//token := "CgAAAHSJ7px6VggEx4eVVFGn4p151HI8pONVLuogox9+Fm9q"
	//token := "CgAAANinWXRZdg6b+kO1c+hCUgP2KxVUWwxxQkR9OR0wpf1h"
	token := "CgAAAFKlK/BV/KsMFva5t9dmD2fugmfsoc3yK9wIiqV+CEwX"

	key := []byte("IypMcRkPXkbeNDRl6Km43boHr98udp7o")
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
	udid := string(decrypted[18:22])
	logger.Debugf(ctx, "udid:%s", udid)
}

func TestPackToken(t *testing.T) {
	tkSrc := []byte{}
	//tkEnc := []byte{}
	randKey := []byte("IypMcRkPXkbeNDRl6Km43boHr98udp7o")
	iv := []byte("'qfpay-----token")

	var plen uint8 = 100
	b_plen := []byte{plen}
	tkSrc = append(tkSrc, b_plen...)

	randStr := "ba"
	tkSrc = append(tkSrc, randStr...)

	ctx := context.Background()
	var ver uint8 = 2
	//b_ver := make([]byte, 1)
	//binary.LittleEndian.PutUint8(b_ver, ver)
	b_ver := []byte{ver}
	tkSrc = append(tkSrc, b_ver...)

	var idc uint8 = 22
	//b_idc := make([]byte, 1)
	//binary.LittleEndian.PutUint8(b_idc, idc)
	b_idc := []byte{idc}
	tkSrc = append(tkSrc, b_idc...)

	flag := "u"
	tkSrc = append(tkSrc, flag...)

	var uid uint64 = 66666666666
	b_uid := make([]byte, 8)
	binary.LittleEndian.PutUint64(b_uid, uid)
	tkSrc = append(tkSrc, b_uid...)

	var expire uint32 = 2222
	b_expire := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_expire, expire)
	tkSrc = append(tkSrc, b_expire...)

	var deadline uint64 = 444444444
	b_deadline := make([]byte, 8)
	binary.LittleEndian.PutUint64(b_deadline, deadline)
	tkSrc = append(tkSrc, b_deadline...)

	var udid_len uint8 = 14
	//b_udid_len := make([]byte, 1)
	//binary.LittleEndian.PutUint8(b_udid_len, udid_len)
	b_udid_len := []byte{udid_len}
	tkSrc = append(tkSrc, b_udid_len...)

	udid := "33333333333333"
	tkSrc = append(tkSrc, udid...)

	var mac uint32 = 33333
	b_mac := make([]byte, 4)
	binary.LittleEndian.PutUint32(b_mac, mac)
	tkSrc = append(tkSrc, b_mac...)
	logger.Debugf(ctx, "tkSrc len: %d", len(tkSrc))

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
	ciphertextBase64, err := aescbc.AESEncryptCBC(ctx, tkSrc, k, iv)
	//ciphertextBase64, err := aescbc.AESEncryptCBC(ctx, k, randKey, iv)
	if err != nil {
		logger.Warnf(ctx, "err: %v", err)
		return
	}
	logger.Debugf(ctx, "ciphertextBase64: %s", ciphertextBase64)
}
