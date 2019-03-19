package dbenc

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	config "github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
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

func DbConfNew(filename string) *DbConf {
	dbc := new(DbConf)
	dbc.DbConfFile = filename
	fi, err := os.Open(filename)
	if err != nil {
		logger.Warnf("open filename %s err: %s", filename, err.Error())
		return nil
	}
	defer fi.Close()
	fbuf, err := ioutil.ReadAll(fi)
	if err != nil {
		logger.Warnf("read filename %s err: %s", filename, err.Error())
		return nil
	}
	flag := string(fbuf[:8])

	if flag != "FFFFFFFF" {
		cfg := config.NewGconf(filename)
		err := cfg.GconfParse()
		if err != nil {
			logger.Warnf("db conf err: %s", err.Error())
			return nil
		}
		dbc.Dbconf = cfg
	} else {
		sbuf := string(fbuf)
		slist := strings.Split(sbuf, "\n")
		for i := 0; i < len(slist); i++ {
			logger.Debugf("get line: %s", slist[i])
			dbc.DbConfFileBuf += slist[i]
		}
		skeylen := dbc.DbConfFileBuf[8:16]
		keylen, err := strconv.ParseInt(skeylen, 16, 64)
		if err != nil {
			logger.Warnf("get keylen %s err %s", skeylen, err.Error())
			return nil
		}
		logger.Debugf("get key len: %d", keylen)
		enckey := dbc.DbConfFileBuf[16 : 16+keylen]
		logger.Debugf("get enckey: %s len: %d", enckey, len(enckey))
		iv, _ := GenAesIv()
		ivc := hex.EncodeToString([]byte(iv[:16]))
		logger.Debugf("iv: %s", ivc)
		key, err := util.AesCbcDec(iv, enckey, ivc)
		if err != nil {
			return nil
		}
		logger.Debugf("get key: %s len: %d", key, len(key))
		padlen := int(key[len(key)-1])
		logger.Debugf("key padlen: %d", padlen)

		encdata := dbc.DbConfFileBuf[16+keylen:]
		realdata, err := util.AesCbcDec(string(key[:len(key)-padlen]), encdata, ivc)
		if err != nil {
			return nil
		}
		padlen = int(realdata[len(realdata)-1])
		logger.Debugf("datalen: %d data padlen: %d", len(realdata), padlen)
		realdata = realdata[:len(realdata)-padlen]
		logger.Debugf("get data: %s", realdata)

		gen := time.Now().UnixNano()
		filename := fmt.Sprintf("/tmp/db_%d.conf", gen)
		fd, err := os.Create(filename)
		defer fd.Close()
		if err != nil {
			logger.Warnf("create file %s %s", filename, err.Error())
			return nil
		}
		fd.WriteString(string(realdata))

		cfg := config.NewGconf(filename)
		err = cfg.GconfParse()
		if err != nil {
			logger.Warnf("db conf err: %s", err.Error())
			return nil
		}
		dbc.Dbconf = cfg
	}
	return dbc
}

func (dbc *DbConf) DbConfReadGroup(group string) map[string]string {
	return dbc.Dbconf.Gcf[group]
}
