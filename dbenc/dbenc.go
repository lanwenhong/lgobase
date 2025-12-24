package dbenc

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	config "github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

type DbConf struct {
	DbConfFile    string
	Dbconf        *config.Gconf
	DbConfFileBuf string
}

func GenAesIv() (string, error) {
	gen_str := "qfpay.com"
	md5ctx := md5.New()
	md5ctx.Write([]byte(gen_str))
	md5_cipher := md5ctx.Sum(nil)
	return hex.EncodeToString(md5_cipher), nil
}

func DbConfNew(ctx context.Context, filename string) *DbConf {
	dbc := new(DbConf)
	dbc.DbConfFile = filename
	fi, err := os.Open(filename)
	if err != nil {
		logger.Warnf(ctx, "open filename %s err: %s", filename, err.Error())
		return nil
	}
	defer fi.Close()
	fbuf, err := ioutil.ReadAll(fi)
	if err != nil {
		logger.Warnf(ctx, "read filename %s err: %s", filename, err.Error())
		return nil
	}
	flag := string(fbuf[:8])

	if flag != "FFFFFFFF" {
		cfg := config.NewGconf(filename)
		err := cfg.GconfParse()
		if err != nil {
			logger.Warnf(ctx, "db conf err: %s", err.Error())
			return nil
		}
		dbc.Dbconf = cfg
	} else {
		sbuf := string(fbuf)
		slist := strings.Split(sbuf, "\n")
		for i := 0; i < len(slist); i++ {
			logger.Debugf(ctx, "get line: %s", slist[i])
			dbc.DbConfFileBuf += slist[i]
		}
		skeylen := dbc.DbConfFileBuf[8:16]
		keylen, err := strconv.ParseInt(skeylen, 16, 64)
		if err != nil {
			logger.Warnf(ctx, "get keylen %s err %s", skeylen, err.Error())
			return nil
		}
		logger.Debugf(ctx, "get key len: %d", keylen)
		enckey := dbc.DbConfFileBuf[16 : 16+keylen]
		logger.Debugf(ctx, "get enckey: %s len: %d", enckey, len(enckey))
		iv, _ := GenAesIv()
		ivc := hex.EncodeToString([]byte(iv[:16]))
		logger.Debugf(ctx, "iv: %s", ivc)
		key, err := util.AesCbcDec(ctx, iv, enckey, ivc)
		if err != nil {
			return nil
		}
		logger.Debugf(ctx, "get key: %s len: %d", key, len(key))
		padlen := int(key[len(key)-1])
		logger.Debugf(ctx, "key padlen: %d", padlen)

		encdata := dbc.DbConfFileBuf[16+keylen:]
		realdata, err := util.AesCbcDec(ctx, string(key[:len(key)-padlen]), encdata, ivc)
		if err != nil {
			return nil
		}
		padlen = int(realdata[len(realdata)-1])
		logger.Debugf(ctx, "datalen: %d data padlen: %d", len(realdata), padlen)
		realdata = realdata[:len(realdata)-padlen]
		logger.Debugf(ctx, "get data: %s", realdata)

		gen := time.Now().UnixNano()
		filename := fmt.Sprintf("/tmp/db_%d.conf", gen)
		fd, err := os.Create(filename)
		defer fd.Close()
		if err != nil {
			logger.Warnf(ctx, "create file %s %s", filename, err.Error())
			return nil
		}
		fd.WriteString(string(realdata))

		cfg := config.NewGconf(filename)
		err = cfg.GconfParse()
		if err != nil {
			logger.Warnf(ctx, "db conf err: %s", err.Error())
			return nil
		}
		dbc.Dbconf = cfg
	}
	return dbc
}

func (dbc *DbConf) DbConfReadGroup(group string) map[string]string {
	ret := make(map[string]string)
	if vGroup, ok := dbc.Dbconf.Gcf[group]; ok {
		//ret[group] = v[0]
		for k, v := range vGroup {
			ret[k] = v[0]
		}
	}
	//return dbc.Dbconf.Gcf[group]
	return ret
}

func (dbc *DbConf) DbConfReadGroupWithCtx(ctx context.Context, group string) map[string]string {
	ret := make(map[string]string)
	if vGroup, ok := dbc.Dbconf.Gcf[group]; ok {
		//ret[group] = v[0]
		for k, v := range vGroup {
			ret[k] = v[0]
		}
	}
	return ret
}
