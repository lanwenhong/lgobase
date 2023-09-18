package util

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/chilts/sid"
	"github.com/google/uuid"

	//"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kjk/betterguid"
	"github.com/oklog/ulid"
	"github.com/rs/xid"
	"github.com/segmentio/ksuid"
	"github.com/sony/sonyflake"
)

type UuidShort struct {
	Id int64 `db:"uuid_short()"`
}

type SvrId struct {
	Name string `db:"Variable_name"`
	Id   int64  `db:"Value"`
}

func Genid(conn *sqlx.DB) (int64, error) {
	sql := "select uuid_short()"
	var u UuidShort
	err := conn.Get(&u, sql)
	if err != nil {
		return -1, err
	}
	sql = "show variables like 'server_id'"
	var id SvrId
	err = conn.Get(&id, sql)
	if err != nil {
		return -1, err
	}
	msec := time.Now().UnixNano() / 1e6
	fmt.Printf("msec: %d\n", msec)
	seq := u.Id % 65535
	gid := msec<<22 + id.Id<<16 + seq
	return int64(gid), nil
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
