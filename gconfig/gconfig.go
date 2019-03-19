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
	Gcf  map[string]map[string]string
	file string
}

func NewGconf(filename string) *Gconf {
	gcf := new(Gconf)
	gcf.file = filename
	gcf.Gcf = make(map[string]map[string]string)
	return gcf
}

/*func (gcf *Gconf) StripBlank(s string) string {
	//reg := regexp.MustCompile("\\s+")
	//return reg.ReplaceAllString(s, "")
	return strings.TrimPrefix(s, " ")
}*/

func (gcf *Gconf) getMk(line string) (string, error) {
	if len(line) < 3 {
		return "", errors.New(fmt.Sprintf("%s len %d error", line, len(line)))
	}
	item := line[1 : len(line)-1]
	//mk := gcf.StripBlank(item)
	mk := strings.Trim(item, " ")
	//注释掉的行
	if mk[0] == '#' || mk[0] == ';' {
		return "", nil
	}
	return mk, nil
}

func (gcf *Gconf) getIk(line string) (string, string, error) {
	if len(line) < 2 {
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
	return ik, iv, nil
}

func (gcf *Gconf) GconfParse() error {
	fi, err := os.Open(gcf.file)
	if err != nil {
		fmt.Printf("read err %s", err.Error())
		return nil
	}
	defer fi.Close()
	br := bufio.NewReader(fi)
	mkey_reg := `^\[.*\]$`
	ikey_reg := `^(.*)\=(.*)$`
	mreg := regexp.MustCompile(mkey_reg)
	ireg := regexp.MustCompile(ikey_reg)

	var mkey string
	var imap map[string]string
	for {
		line, _, err := br.ReadLine()
		if err == io.EOF {
			break
		}
		sline := string(line)
		if mreg.MatchString(sline) {
			mk, err := gcf.getMk(sline)
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
			k, v, err := gcf.getIk(sline)
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
		} else {
			fmt.Println("no match continue\n")
		}
	}
	fmt.Println(gcf.Gcf)
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
