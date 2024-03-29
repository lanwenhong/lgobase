package util

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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
	return strings.Replace(uuid.New().String(), "-", "", -1)
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
