package dbenc

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	config "github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
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
		logger.Warn(ctx, "open database config failed", "filename", filename, "err", err)
		return nil
	}
	defer fi.Close()
	fbuf, err := ioutil.ReadAll(fi)
	if err != nil {
		logger.Warn(ctx, "read database config failed", "filename", filename, "err", err)
		return nil
	}
	flag := string(fbuf[:8])

	if flag != "FFFFFFFF" {
		cfg := config.NewGconf(filename)
		err := cfg.GconfParseFromString(string(fbuf))
		if err != nil {
			logger.Warn(ctx, "parse database config failed", "filename", filename, "err", err)
			return nil
		}
		dbc.Dbconf = cfg
	} else {
		sbuf := string(fbuf)
		slist := strings.Split(sbuf, "\n")
		for i := 0; i < len(slist); i++ {
			logger.Debug(ctx, "read encrypted database config line", "line_index", i, "line", slist[i])
			dbc.DbConfFileBuf += slist[i]
		}
		skeylen := dbc.DbConfFileBuf[8:16]
		keylen, err := strconv.ParseInt(skeylen, 16, 64)
		if err != nil {
			logger.Warn(ctx, "parse encrypted database key length failed", "encoded_length", skeylen, "err", err)
			return nil
		}
		logger.Debug(ctx, "parsed encrypted database key length", "key_length", keylen)
		enckey := dbc.DbConfFileBuf[16 : 16+keylen]
		logger.Debug(ctx, "read encrypted database key", "encrypted_key", enckey, "length", len(enckey))
		iv, _ := GenAesIv()
		ivc := hex.EncodeToString([]byte(iv[:16]))
		logger.Debug(ctx, "generated database config IV", "iv", ivc)
		key, err := util.AesCbcDec(ctx, iv, enckey, ivc)
		if err != nil {
			return nil
		}
		logger.Debug(ctx, "decrypted database key", "key", key, "length", len(key))
		padlen := int(key[len(key)-1])
		logger.Debug(ctx, "parsed database key padding", "padding_length", padlen)

		encdata := dbc.DbConfFileBuf[16+keylen:]
		realdata, err := util.AesCbcDec(ctx, string(key[:len(key)-padlen]), encdata, ivc)
		if err != nil {
			return nil
		}
		padlen = int(realdata[len(realdata)-1])
		logger.Debug(ctx, "parsed database config padding", "data_length", len(realdata), "padding_length", padlen)
		realdata = realdata[:len(realdata)-padlen]
		logger.Debug(ctx, "decrypted database config", "data", realdata)
		cfg := config.NewGconf(filename)
		err = cfg.GconfParseFromString(string(realdata))
		if err != nil {
			logger.Warn(ctx, "parse decrypted database config failed", "filename", filename, "err", err)
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
