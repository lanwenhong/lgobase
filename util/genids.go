package util

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"time"
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
