package gconfig

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/lanwenhong/lgobase/logger"
)

type Gconf struct {
	Gcf         map[string]map[string][]string
	GlineExtend map[string]map[string][]map[string]string
	file        string
	//GlineExtend map[string]map[string][][]string
}

func NewGconf(filename string) *Gconf {
	gcf := new(Gconf)
	gcf.file = filename
	gcf.Gcf = make(map[string]map[string][]string)
	//gcf.GlineExtend = make(map[string]map[string][][]string)
	gcf.GlineExtend = make(map[string]map[string][]map[string]string)
	return gcf
}

/*func (gcf *Gconf) StripBlank(s string) string {
	//reg := regexp.MustCompile("\\s+")
	//return reg.ReplaceAllString(s, "")
	return strings.TrimPrefix(s, " ")
}*/

func (gcf *Gconf) getMk(line string, re *regexp.Regexp) (string, error) {
	groups := re.SubexpNames()
	match := re.FindStringSubmatch(line)
	item := ""
	for i, _ := range groups {
		if i == 1 {
			//return match[i], nil
			item = strings.Trim(match[i], " ")
		}
	}
	return item, nil
}

func (gcf *Gconf) getIk(line string, re *regexp.Regexp) (string, string, error) {
	rets := []string{}
	groups := re.SubexpNames()
	match := re.FindStringSubmatch(line)
	//fmt.Println(match)
	for i, _ := range groups {
		if i != 0 {
			rets = append(rets, match[i])
		}

	}
	ret0 := strings.Trim(rets[0], " ")
	ret1 := strings.Trim(rets[1], " ")
	return ret0, ret1, nil
}

/*func (gcf *Gconf) AddExd(line_key, ex_key string) {
	ext := []string{}
	item := "1 == 1"
	ext = append(ext, item)
	ext = append(ext, "0")
	if _, ok := gcf.GlineExtend[line_key]; ok {
		gcf.GlineExtend[line_key][ex_key] = append(gcf.GlineExtend[line_key][ex_key], ext)
	} else {
		xval := make(map[string][][]string)
		xval[ex_key] = append(xval[ex_key], ext)
		gcf.GlineExtend[line_key] = xval
	}

}*/

func (gcf *Gconf) getIkExd(line_key string, ex_key string, exd_line string, re *regexp.Regexp, newLineTag *bool) error {
	//groups := re.SubexpNames()
	match := re.FindStringSubmatch(exd_line)
	ctx := context.Background()
	if len(match) >= 3 {
		k := strings.TrimSpace(match[1])
		v := strings.TrimSpace(match[2])
		logger.Debug(ctx, "loaded config entry", "key", k, "value", v)
		if _, ok := gcf.GlineExtend[line_key][ex_key]; ok {
			exDataLen := len(gcf.GlineExtend[line_key][ex_key])
			if *newLineTag {
				oneMap := make(map[string]string)
				oneMap[k] = v
				gcf.GlineExtend[line_key][ex_key] = append(gcf.GlineExtend[line_key][ex_key], oneMap)
				*newLineTag = false
			} else {
				gcf.GlineExtend[line_key][ex_key][exDataLen-1][k] = v
			}
		} else {
			if _, ok := gcf.GlineExtend[line_key]; ok {
				logger.Debug(ctx, "loaded extended config entry", "key", line_key, "value", gcf.GlineExtend[line_key])
				oneMap := make(map[string]string)
				oneMap[k] = v
				xval := make([]map[string]string, 0)
				xval = append(xval, oneMap)
				gcf.GlineExtend[line_key][ex_key] = xval

			} else {
				oneMap := make(map[string]string)
				oneMap[k] = v
				xval := make([]map[string]string, 0)
				xval = append(xval, oneMap)

				eval := make(map[string][]map[string]string)
				eval[ex_key] = xval
				gcf.GlineExtend[line_key] = eval
			}
			*newLineTag = false
		}
	}
	return nil
}

func (gcf *Gconf) DoConfParse(ctx context.Context, br *bufio.Reader) error {
	//mkey_reg := `^\[.*\]$`
	mkey_reg := `^\[(\w+)]$`
	//ikey_reg := `^(.*)\=(.*)$`
	ikey_reg := `^(?P<k>\w+)\s*\=\s*(?P<v>.*)$`
	//ikey_reg_ex := `^\s{1,4}([\w][a-z,A-Z,0-9,=,<,>,>=,<=,!=,_,\,\s,\-,\\.]+)$`
	//ikey_reg_ex := `^\s{1,4}(.*)$`
	ikey_reg_ex := `^\s*(rule|MaxConns|MaxIdleConns|MaxConnLife|MaxIdleConnLife|proto|Salience)\s*=\s*(.+)$`

	mreg := regexp.MustCompile(mkey_reg)
	ireg := regexp.MustCompile(ikey_reg)
	ireg_ex := regexp.MustCompile(ikey_reg_ex)

	var mkey string
	//var imap map[string]string
	var imap map[string][]string
	var line_key string = ""
	var ex_key string = ""
	var newLinetag = false
	for {
		line, _, err := br.ReadLine()
		if err == io.EOF {
			break
		}
		sline := string(line)
		if mreg.MatchString(sline) {
			//parse section
			mk, err := gcf.getMk(sline, mreg)
			if err != nil {
				return err
			}
			//处理注释掉的行
			if mk == "" {
				//fmt.Printf("%s note\n", sline)
				logger.Debug(ctx, "skip config comment", "line", sline)
				imap = nil
			} else {
				mkey = mk
				//fmt.Printf("mk: %s\n", mkey)
				logger.Debug(ctx, "parse config section", "section", mkey)
				//imap = make(map[string]string)
				imap = make(map[string][]string)
				gcf.Gcf[mkey] = imap
			}
		} else if ireg.MatchString(sline) {
			//parse line
			if imap == nil {
				//fmt.Printf("%s not found section\n", sline)
				logger.Debug(ctx, "config entry has no section", "line", sline)
				continue
			}
			k, v, err := gcf.getIk(sline, ireg)
			if err != nil {
				return err
			}
			//处理注释掉的行
			if k == "" {
				//fmt.Printf("%s note\n", sline)
				logger.Debug(ctx, "skip config comment", "line", sline)
				continue
			}
			//fmt.Printf("k=%s v=%s\n", k, v)
			logger.Debug(ctx, "parse config key-value", "key", k, "value", v)
			//imap[k] = v
			imap[k] = append(imap[k], v)
			line_key = k
			ex_key = sline
			newLinetag = true
		} else if ireg_ex.MatchString(sline) {
			//处理扩展行
			//gcf.getIkExd(line_key, sline, ireg_ex)
			gcf.getIkExd(line_key, ex_key, sline, ireg_ex, &newLinetag)

		} else {
			//fmt.Println("no match continue")
			logger.Debug(ctx, "skip unmatched config line", "line", sline)
		}
	}
	//fmt.Println(gcf.Gcf)
	//fmt.Println(gcf.GlineExtend)
	//logger.Debug(ctx, "gconfig", "Gcf", gcf.Gcf)
	//logger.Debug(ctx, "gconfig", "gcf.GlineExtend", gcf.GlineExtend)
	return nil
}

func (gcf *Gconf) GconfParse() error {
	ctx := context.Background()
	fi, err := os.Open(gcf.file)
	if err != nil {
		//fmt.Printf("read err %s", err.Error())
		logger.Warn(ctx, "read config file failed", "filename", gcf.file, "err", err)
		return nil
	}
	defer fi.Close()
	br := bufio.NewReader(fi)
	err = gcf.DoConfParse(ctx, br)
	return err
}

func (gcf *Gconf) GconfParseFromString(content string) error {
	ctx := context.Background()
	br := bufio.NewReader(strings.NewReader(content))
	err := gcf.DoConfParse(ctx, br)
	return err
}

func (gcf *Gconf) HasSection(section string) bool {
	_, ok := gcf.Gcf[section]
	return ok
}

// func (gcf *Gconf) GetSection(section string) (map[string]string, error) {
func (gcf *Gconf) GetSection(section string) (map[string][]string, error) {
	xs, ok := gcf.Gcf[section]
	if ok {
		return xs, nil
	}
	return nil, errors.New(fmt.Sprintf("%s not exist\n", section))
}
