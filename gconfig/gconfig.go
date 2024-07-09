package gconfig

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type Gconf struct {
	Gcf         map[string]map[string]string
	GlineExtend map[string][]string
	file        string
}

func NewGconf(filename string) *Gconf {
	gcf := new(Gconf)
	gcf.file = filename
	gcf.Gcf = make(map[string]map[string]string)
	gcf.GlineExtend = make(map[string][]string)
	return gcf
}

/*func (gcf *Gconf) StripBlank(s string) string {
	//reg := regexp.MustCompile("\\s+")
	//return reg.ReplaceAllString(s, "")
	return strings.TrimPrefix(s, " ")
}*/

func (gcf *Gconf) getMk(line string, re *regexp.Regexp) (string, error) {
	/*if len(line) < 3 {
		return "", errors.New(fmt.Sprintf("%s len %d error", line, len(line)))
	}
	item := line[1 : len(line)-1]
	//mk := gcf.StripBlank(item)
	mk := strings.Trim(item, " ")
	//注释掉的行
	if mk[0] == '#' || mk[0] == ';' {
		return "", nil
	}*/
	//return mk, nil
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
	/*if len(line) < 2 {
		return "", "", errors.New(fmt.Sprintf("%s len %d error", line, len(line)))
	}
	index := strings.IndexAny(line, "=")
	var ik string
	var iv string
	//like config f=
	if len(line) == 2 {
		//ik = gcf.StripBlank(line[0:index])
		ik = strings.Trim(line[0:index], " ")
		iv = ""
	} else {
		//ik = gcf.StripBlank(line[0:index])
		ik = strings.Trim(line[0:index], " ")
		//iv = gcf.StripBlank(line[index+1:])
		iv = strings.Trim(line[index+1:], " ")
	}
	//注释掉的行
	if ik[0] == '#' || ik[0] == ';' {
		return "", "", nil
	}
	return ik, iv, nil*/

	rets := []string{}
	groups := re.SubexpNames()
	match := re.FindStringSubmatch(line)
	fmt.Println(match)
	for i, _ := range groups {
		if i != 0 {
			rets = append(rets, match[i])
		}

	}
	ret0 := strings.Trim(rets[0], " ")
	ret1 := strings.Trim(rets[1], " ")
	return ret0, ret1, nil
}

func (gcf *Gconf) getIkExd(line_key string, exd_line string, re *regexp.Regexp) error {
	groups := re.SubexpNames()
	match := re.FindStringSubmatch(exd_line)

	for i, _ := range groups {
		if i != 0 {
			fmt.Println(match[i])
			if val, ok := gcf.GlineExtend[line_key]; ok {
				item := strings.Trim(match[i], " ")
				val = append(val, item)
				gcf.GlineExtend[line_key] = val
			} else {
				val := []string{}
				item := strings.Trim(match[i], " ")
				val = append(val, item)
				gcf.GlineExtend[line_key] = val
			}
		}
	}
	return nil
}

func (gcf *Gconf) GconfParse() error {
	fi, err := os.Open(gcf.file)
	if err != nil {
		fmt.Printf("read err %s", err.Error())
		return nil
	}
	defer fi.Close()
	br := bufio.NewReader(fi)
	//mkey_reg := `^\[.*\]$`
	mkey_reg := `^\[(\w+)]$`
	//ikey_reg := `^(.*)\=(.*)$`
	ikey_reg := `^(?P<k>\w+)\s*\=\s*(?P<v>.*)$`
	ikey_reg_ex := `^\s{1,4}([\w][a-z,A-Z,0-9,=,<,>,>=,<=,!=,_,\,\s,\-,\\.]+)$`

	mreg := regexp.MustCompile(mkey_reg)
	ireg := regexp.MustCompile(ikey_reg)
	ireg_ex := regexp.MustCompile(ikey_reg_ex)

	var mkey string
	var imap map[string]string
	var line_key string = ""
	for {
		line, _, err := br.ReadLine()
		if err == io.EOF {
			break
		}
		sline := string(line)
		if mreg.MatchString(sline) {
			mk, err := gcf.getMk(sline, mreg)
			if err != nil {
				return err
			}
			//处理注释掉的行
			if mk == "" {
				fmt.Printf("%s note\n", sline)
				imap = nil
			} else {
				mkey = mk
				fmt.Printf("mk: %s\n", mkey)
				imap = make(map[string]string)
				gcf.Gcf[mkey] = imap
			}
		} else if ireg.MatchString(sline) {
			if imap == nil {
				fmt.Printf("%s not found section\n", sline)
				continue
			}
			k, v, err := gcf.getIk(sline, ireg)
			if err != nil {
				return err
			}
			//处理注释掉的行
			if k == "" {
				fmt.Printf("%s note\n", sline)
				continue
			}
			fmt.Printf("k=%s v=%s\n", k, v)
			imap[k] = v
			line_key = k
		} else if ireg_ex.MatchString(sline) {
			//处理扩展行
			gcf.getIkExd(line_key, sline, ireg_ex)
		} else {
			fmt.Println("no match continue")
		}
	}
	fmt.Println(gcf.Gcf)
	fmt.Println(gcf.GlineExtend)
	return nil
}

func (gcf *Gconf) HasSection(section string) bool {
	_, ok := gcf.Gcf[section]
	return ok
}

func (gcf *Gconf) GetSection(section string) (map[string]string, error) {
	xs, ok := gcf.Gcf[section]
	if ok {
		return xs, nil
	}
	return nil, errors.New(fmt.Sprintf("%s not exist\n", section))
}
