package util

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/hex"
	"fmt"
	"strings"
)

func PadBufferToNByteBlocks(buffer []byte, filler byte, n int) []byte {
	paddingLen := n - (len(buffer) % n)
	if paddingLen == n {
		return buffer
	}
	return append(buffer, bytes.Repeat([]byte{filler}, paddingLen)...)
}

func ByteXor(a, b []byte) []byte {
	length := len(a)
	result := make([]byte, length)
	
	for i := 0; i < length; i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

func CalcMac(key []byte, macData []byte) (string, error) {
	block, err := GenBlock(key)
	if err != nil {
		return "", err
	}
	
	paddedData := PadBufferToNByteBlocks(macData, 0x00, des.BlockSize)
	// X9.9 mac计算
	prevEncrypted := make([]byte, des.BlockSize) // des.BlockSize=8
	for i := 0; i < len(paddedData); i += des.BlockSize {
		currentBlock := paddedData[i : i+des.BlockSize]
		xorResult := ByteXor(prevEncrypted, currentBlock)
		encrypted := make([]byte, des.BlockSize)
		block.Encrypt(encrypted, xorResult)
		prevEncrypted = encrypted
	}
	finalMAC := append(prevEncrypted[:4], []byte{0x00, 0x00, 0x00, 0x00}...)
	return strings.ToUpper(hex.EncodeToString(finalMAC)), nil
}

func GenBlock(key []byte) (cipher.Block, error) {
	if len(key) != 16 && len(key) != 24 {
		return nil, fmt.Errorf("密钥长度必须为16或24字节")
	}
	if len(key) == 16 {
		key = append(key[:16], key[:8]...)
	}
	block, err := des.NewTripleDESCipher(key)
	return block, err
}
