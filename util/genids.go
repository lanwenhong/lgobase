package util

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/chilts/sid"
	"github.com/google/uuid"
	"gorm.io/gorm"

	//"github.com/google/uuid"

	"github.com/kjk/betterguid"
	"github.com/oklog/ulid"
	"github.com/rs/xid"
	"github.com/segmentio/ksuid"
	"github.com/sony/sonyflake"
)

type Serverid struct {
	Svrid uint64 `gorm:"column:@@server_id"`
}

type UuidShort struct {
	UuidShort uint64 `gorm:"column:uuid_short()"`
}

func Genid(ctx context.Context, conn *gorm.DB) (uint64, error) {
	s := []Serverid{}
	su := []UuidShort{}

	sql := "select @@server_id"
	ret := conn.WithContext(ctx).Raw(sql).Scan(&s)
	if ret.Error != nil {
		return 0, ret.Error
	}
	ret = conn.WithContext(ctx).Raw("select uuid_short()").Scan(&su)
	if ret.Error != nil {
		return 0, ret.Error
	}
	seq := su[0].UuidShort % 65535
	tt := time.Now().Unix()
	msec := tt * 1000
	id := uint64(msec)<<22 + s[0].Svrid<<16 + uint64(seq)
	return id, nil
}

func NewRequestID() string {
	s := strings.Replace(uuid.New().String(), "-", "", -1)
	return s[16:]
}

func GenXid() string {
	id := xid.New()
	sid := fmt.Sprintf("%s", id)
	return sid
}

func GenKsuid() string {
	id := ksuid.New()
	sid := fmt.Sprintf("%s", id)
	return sid
}

func GenBetterGUID() string {
	id := betterguid.New()
	return id
}

func GenUlid() string {
	t := time.Now().UTC()
	entropy := rand.New(rand.NewSource(t.UnixNano()))
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	return id.String()
}

func GenSonyflake() string {
	flake := sonyflake.NewSonyflake(sonyflake.Settings{})
	id, err := flake.NextID()
	if err != nil {
		log.Fatalf("flake.NextID() failed with %s\n", err)
	}
	// Note: this is base16, could shorten by encoding as base62 string
	//fmt.Printf("github.com/sony/sonyflake:   %x\n", id)
	sid := fmt.Sprintf("%x", id)
	return sid
}

func GenSid() string {
	id := sid.Id()
	return id
}

func GenerateSecureRandomString(length int) (string, error) {
	// 需要的字节数，base64编码会增加约1/3的长度
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}
	// 使用URLEncoding避免生成+和/字符
	return base64.URLEncoding.EncodeToString(buffer)[:length], nil
}

// 生成指定长度的普通随机字符串（性能更好，适用于非敏感场景）
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// 初始化随机数生成器
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func generateRandomString(length int) (string, error) {
	// 计算需要的字节数，base64编码会增加约1/3的长度
	bytesNeeded := (length * 3) / 4
	if (length*3)%4 != 0 {
		bytesNeeded++
	}

	buffer := make([]byte, bytesNeeded)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	// 使用URLEncoding避免生成+和/字符，并截取到指定长度
	return base64.URLEncoding.EncodeToString(buffer)[:length], nil
}

func GenerateUniqueStringWithTimestamp(prefix string) string {
	// 时间戳+随机字符串组合
	timestamp := time.Now().UnixNano()
	randomPart, _ := generateRandomString(8)
	//return fmt.Sprintf("%s_%d_%s", prefix, timestamp, randomPart)
	ret := prefix + "_" + strconv.FormatInt(timestamp, 10) + "_" + randomPart
	return ret
}
