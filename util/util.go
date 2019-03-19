package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/lgobase/logger"
	"sort"
	"strings"
)

func SvrSign(data map[string]string, key string) string {
	sorted_keys := make([]string, 0)
	dlist := make([]string, 0)
	for k, _ := range data {
		sorted_keys = append(sorted_keys, k)
	}
	sort.Strings(sorted_keys)
	for _, k := range sorted_keys {
		item := fmt.Sprintf("%s=%s", k, data[k])
		dlist = append(dlist, item)
	}
	sdata := strings.Join(dlist, "&") + key

	logger.Debugf("===server sign data: %s", sdata)
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(sdata))
	cipherStr := md5Ctx.Sum(nil)

	sign := hex.EncodeToString(cipherStr)
	logger.Debugf("===server sign: %s", sign)
	return sign
}

func SvrVerify(data map[string]string, key string) error {
	c_sign := data["sign"]

	sorted_keys := make([]string, 0)
	dlist := make([]string, 0)
	for k, _ := range data {
		if k != "sign" {
			sorted_keys = append(sorted_keys, k)
		}
	}
	sort.Strings(sorted_keys)
	for _, k := range sorted_keys {
		item := fmt.Sprintf("%s=%s", k, data[k])
		dlist = append(dlist, item)
	}
	sdata := strings.Join(dlist, "&") + key

	logger.Debugf("===server check sign data: %s", sdata)

	md5Ctx := md5.New()
	md5Ctx.Write([]byte(sdata))
	cipherStr := md5Ctx.Sum(nil)

	s_sign := hex.EncodeToString(cipherStr)
	logger.Debugf("s_sign: %s c_sign: %s", s_sign, c_sign)
	if c_sign != s_sign {
		return errors.New("sever verify error")
	}
	return nil
}

func AesCbcDec(key string, enc string, iv string) ([]byte, error) {
	bkey := []byte(key)
	benc, _ := hex.DecodeString(iv + enc)

	logger.Debugf("bkey: %s len: %d", bkey, len(bkey))
	block, err := aes.NewCipher(bkey)
	if err != nil {
		logger.Warnf("aes key %s init: %s", key, err.Error())
		return []byte(""), err
	}
	if len(benc) < aes.BlockSize {
		logger.Warnf("ciphertext too short")
		return []byte(""), errors.New("ciphertext too short")
	}
	ivc := benc[:aes.BlockSize]
	benc = benc[aes.BlockSize:]
	if len(benc)%aes.BlockSize != 0 {
		logger.Warnf("ciphertext is not a multiple of the block size")
		return []byte(""), errors.New("ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, ivc)
	mode.CryptBlocks(benc, benc)
	return benc, nil
}

type ecbDecrypter Ecb
type ecbEncrypter Ecb
type Ecb struct {
	b         cipher.Block
	blockSize int
}

func newECB(b cipher.Block) *Ecb {
	return &Ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

func NewECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

func NewECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}

func (x *ecbEncrypter) BlockSize() int { return x.blockSize }
func (x *ecbDecrypter) BlockSize() int { return x.blockSize }

func (x *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Encrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

func (x *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Decrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

//aes ecb support key len 128bit, 192bit, 256bit
func AesEcbDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		logger.Warnf("err is:", err)
		return nil, err
	}
	blockMode := NewECBDecrypter(block)
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS5UnPadding(origData)
	//logger.Debugf("source is :", origData, string(origData))
	return origData, nil
}

func AesEcbEncrypt(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	ecb := NewECBEncrypter(block)
	content := []byte(src)
	content = PKCS5Padding(content, block.BlockSize())
	logger.Debugf("content: %x\n", content)
	crypted := make([]byte, len(content))
	ecb.CryptBlocks(crypted, content)
	return crypted, nil
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
