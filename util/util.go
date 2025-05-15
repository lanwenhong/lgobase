package util

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lanwenhong/lgobase/logger"
)

func SvrSign(ctx context.Context, data map[string]string, key string) string {
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

	logger.Debugf(ctx, "===server sign data: %s", sdata)
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(sdata))
	cipherStr := md5Ctx.Sum(nil)

	sign := hex.EncodeToString(cipherStr)
	logger.Debugf(ctx, "===server sign: %s", sign)
	return sign
}

func SvrVerify(ctx context.Context, data map[string]string, key string) error {
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

	logger.Debugf(ctx, "===server check sign data: %s", sdata)

	md5Ctx := md5.New()
	md5Ctx.Write([]byte(sdata))
	cipherStr := md5Ctx.Sum(nil)

	s_sign := hex.EncodeToString(cipherStr)
	logger.Debugf(ctx, "s_sign: %s c_sign: %s", s_sign, c_sign)
	if c_sign != s_sign {
		return errors.New("sever verify error")
	}
	return nil
}

func AesCbcDec(ctx context.Context, key string, enc string, iv string) ([]byte, error) {
	bkey := []byte(key)
	benc, _ := hex.DecodeString(iv + enc)

	logger.Debugf(ctx, "bkey: %s len: %d", bkey, len(bkey))
	block, err := aes.NewCipher(bkey)
	if err != nil {
		logger.Warnf(ctx, "aes key %s init: %s", key, err.Error())
		return []byte(""), err
	}
	if len(benc) < aes.BlockSize {
		logger.Warnf(ctx, "ciphertext too short")
		return []byte(""), errors.New("ciphertext too short")
	}
	ivc := benc[:aes.BlockSize]
	benc = benc[aes.BlockSize:]
	if len(benc)%aes.BlockSize != 0 {
		logger.Warnf(ctx, "ciphertext is not a multiple of the block size")
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

// aes ecb support key len 128bit, 192bit, 256bit
func AesEcbDecrypt(ctx context.Context, crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		logger.Warnf(ctx, "err is:", err)
		return nil, err
	}
	blockMode := NewECBDecrypter(block)
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS5UnPadding(origData)
	//logger.Debugf("source is :", origData, string(origData))
	return origData, nil
}

func AesEcbEncrypt(ctx context.Context, src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	ecb := NewECBEncrypter(block)
	content := []byte(src)
	content = PKCS5Padding(content, block.BlockSize())
	logger.Debugf(ctx, "content: %x\n", content)
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

func Mapget(m map[string]interface{}, key string, defv interface{}) interface{} {
	if val, ok := m[key]; ok {
		return val
	}
	return defv
}

type AesCbc struct {
}

func (aescbc *AesCbc) PKCS7Padding(ctx context.Context, data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

func (aescbc *AesCbc) PKCS7UnPadding(ctx context.Context, data []byte) []byte {
	length := len(data)
	if length == 0 {
		return nil
	}
	unpadding := int(data[length-1])
	if unpadding > length || unpadding == 0 {
		return nil
	}
	for i := length - unpadding; i < length; i++ {
		if data[i] != byte(unpadding) {
			return nil
		}
	}
	return data[:(length - unpadding)]
}

// func (aescbc *AesCbc) AESEncryptCBC(ctx context.Context, plaintext []byte, key []byte) (string, error) {
func (aescbc *AesCbc) AESEncryptCBC(ctx context.Context, plaintext []byte, key []byte, iv []byte) (string, error) {
	// 检查密钥长度（必须为32字节，即256位）
	if len(key) != 32 {
		return "", fmt.Errorf("AES-256密钥长度必须为32字节，当前长度：%d", len(key))
	}

	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("iv必须16字节，当前长度: %d", len(iv))
	}

	// 创建加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// 填充明文
	plaintext = aescbc.PKCS7Padding(ctx, plaintext, block.BlockSize())

	// 生成随机IV
	/*ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}*/
	ciphertext := make([]byte, len(plaintext))

	// 创建CBC模式
	mode := cipher.NewCBCEncrypter(block, iv)

	// 加密数据
	//mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)
	mode.CryptBlocks(ciphertext, plaintext)

	// 返回Base64编码的结果（IV+密文）
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (aescbc *AesCbc) AESDecryptCBCSb(ctx context.Context, ciphertextBase64 string, key []byte) ([]byte, error) {
	// 检查密钥长度
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256密钥长度必须为32字节，当前长度：%d", len(key))
	}

	// 解码Base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, err
	}

	logger.Debugf(ctx, "ciphertext len: %d", len(ciphertext))
	// 检查数据长度
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("密文长度过短，至少需要 %d 字节", aes.BlockSize)
	}

	// 提取IV和密文
	//iv := ciphertext[:aes.BlockSize]
	//iv := [aes.BlockSize]byte{}
	iv := make([]byte, aes.BlockSize)
	//ciphertext = ciphertext[aes.BlockSize:]
	ciphertext = ciphertext[4:]

	// 创建解密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 检查密文长度是否为块大小的整数倍
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小(%d字节)的整数倍", aes.BlockSize)
	}

	// 创建CBC模式
	mode := cipher.NewCBCDecrypter(block, iv)

	// 解密数据
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除填充
	/*plaintext = aescbc.PKCS7UnPadding(ctx, plaintext)
	if plaintext == nil {
		return nil, fmt.Errorf("无效的填充数据")
	}*/

	return plaintext, nil
}

// func (aescbc *AesCbc) AESDecryptCBC(ctx context.Context, ciphertextBase64 string, key []byte) ([]byte, error) {
func (aescbc *AesCbc) AESDecryptCBC(ctx context.Context, ciphertextBase64 string, key []byte, iv []byte) ([]byte, error) {
	// 检查密钥长度
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256密钥长度必须为32字节，当前长度：%d", len(key))
	}

	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("iv必须16字节，当前长度：%d", len(iv))
	}

	// 解码Base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, err
	}

	logger.Debugf(ctx, "ciphertext len: %d", len(ciphertext))
	// 检查数据长度
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("密文长度过短，至少需要 %d 字节", aes.BlockSize)
	}

	// 提取IV和密文
	//iv := ciphertext[:aes.BlockSize]
	//iv := make([]byte, aes.BlockSize)

	// 创建解密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 检查密文长度是否为块大小的整数倍
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小(%d字节)的整数倍", aes.BlockSize)
	}

	// 创建CBC模式
	mode := cipher.NewCBCDecrypter(block, iv)

	// 解密数据
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除填充
	plaintext = aescbc.PKCS7UnPadding(ctx, plaintext)
	if plaintext == nil {
		return nil, fmt.Errorf("无效的填充数据")
	}
	return plaintext, nil
}
