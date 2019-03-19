package confparse

import (
	"errors"
	"fmt"
	config "github.com/lanwenhong/lgobase/gconfig"
	"reflect"
	"strconv"
	"strings"
)

type UserSelfFunc func(interface{}, string) error

const (
	CP_TAG_POS        = "confpos"
	CP_TAG_ITEM_SPLIT = "item_split"
	CP_TAG_KV_SPLIT   = "kv_split"
	CP_TAG_DTYPE      = "dtype"
)

const (
	CP_DTYPE_BASE    = "base"
	CP_DTYPE_COMPLEX = "complex"
)

type Cparse struct {
	Filename string
	Funcs    map[string]UserSelfFunc
}

func CpaseNew(fname string) *Cparse {
	cp := new(Cparse)
	cp.Filename = fname
	cp.Funcs = make(map[string]UserSelfFunc)
	return cp
}

func (cp *Cparse) parseSection(cfg *config.Gconf, sectionName string) (map[string]string, error) {
	return cfg.GetSection(sectionName)
}

func (cp *Cparse) parseBaseTag(v reflect.StructField) (string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)
	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		return "", "", errors.New(fmt_e)
	}
	fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	return slist[0], slist[1], nil
}

func (cp *Cparse) parseMapTag(v reflect.StructField) (string, string, string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)
	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		return "", "", "", "", errors.New(fmt_e)
	}
	fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	se := slist[0]
	k := slist[1]

	is := v.Tag.Get(CP_TAG_ITEM_SPLIT)
	if is == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_ITEM_SPLIT)
		return "", "", "", "", errors.New(fmt_e)
	}

	ks := v.Tag.Get(CP_TAG_KV_SPLIT)
	if ks == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_KV_SPLIT)
		return "", "", "", "", errors.New(fmt_e)
	}

	return se, k, is, ks, nil
}

func (cp *Cparse) parseSliceTag(v reflect.StructField) (string, string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)

	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		return "", "", "", errors.New(fmt_e)
	}
	fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	se := slist[0]
	k := slist[1]

	is := v.Tag.Get(CP_TAG_ITEM_SPLIT)
	if is == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_ITEM_SPLIT)
		return "", "", "", errors.New(fmt_e)
	}
	return se, k, is, nil
}

func (cp *Cparse) getDatafromIni(section string, key string, cfg *config.Gconf) (string, error) {
	if cfg.HasSection(section) {
		item, err := cp.parseSection(cfg, section)
		if err != nil {
			return "", err
		}
		word, ok := item[key]
		if !ok {
			fmt_e := fmt.Sprintf("section %s key %s not exist", section, key)
			return "", errors.New(fmt_e)
		}
		return word, nil
	}
	fmt_e := fmt.Sprintf("section %s not exist", section)
	return "", errors.New(fmt_e)
}

func (cp *Cparse) setInt(v reflect.Value, x string) error {
	real_v, err := strconv.ParseInt(x, 10, 64)
	if err != nil {
		return err
	}
	v.SetInt(real_v)
	return nil
}

func (cp *Cparse) parseInt(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIni(section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setInt(v, word)
}

func (cp *Cparse) setUint(v reflect.Value, x string) error {
	real_v, err := strconv.ParseUint(x, 10, 64)
	if err != nil {
		return err
	}
	v.SetUint(real_v)
	return nil
}

func (cp *Cparse) parseUint(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIni(section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setUint(v, word)
}

func (cp *Cparse) setFloat(v reflect.Value, x string) error {
	real_v, err := strconv.ParseFloat(x, 64)
	if err != nil {
		return err
	}
	v.SetFloat(real_v)
	return nil
}

func (cp *Cparse) parseFloat(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIni(section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setFloat(v, word)
}

func (cp *Cparse) setString(v reflect.Value, x string) error {
	v.SetString(x)
	return nil
}

func (cp *Cparse) parseString(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIni(section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setString(v, word)
}

func (cp *Cparse) setBool(v reflect.Value, x string) error {
	ret, err := strconv.ParseBool(x)
	if err != nil {
		return err
	}
	v.SetBool(ret)
	return nil
}

func (cp *Cparse) parseBool(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIni(section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setBool(v, word)
}

func (cp *Cparse) getBaseValue(k reflect.Kind, v string) (reflect.Value, error) {
	data := 0
	f := 0.0
	b := true
	s := ""
	switch k {
	case reflect.Int:
		x := reflect.ValueOf(&data)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Int8:
		d := int8(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Int16:
		d := int16(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Int32:
		d := int32(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)

		return x, err

	case reflect.Int64:
		d := int64(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Uint8:
		d := uint8(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Uint16:
		d := uint16(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Uint32:
		d := uint32(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Uint64:
		d := uint64(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setInt(x, v)
		return x, err

	case reflect.Float32:
		d := float32(f)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setFloat(x, v)
		return x, err

	case reflect.Float64:
		d := float64(f)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setFloat(x, v)
		return x, err

	case reflect.Bool:
		x := reflect.ValueOf(&b)
		x = x.Elem()
		err := cp.setBool(x, v)
		return x, err

	case reflect.String:
		x := reflect.ValueOf(&s)
		x = x.Elem()
		err := cp.setString(x, v)
		return x, err
	}
	return reflect.ValueOf(data), nil
}

func (cp *Cparse) fillMap(v reflect.Value, sk string, sv string) error {
	vk := reflect.ValueOf(sk)
	vtype := v.Type().Elem().Kind()
	vv, err := cp.getBaseValue(vtype, sv)
	if err != nil {
		return err
	}
	v.SetMapIndex(vk, vv)
	return nil
}

func (cp *Cparse) fillSlice(v reflect.Value, sv string) (reflect.Value, error) {
	vtype := v.Type().Elem().Kind()
	vv, err := cp.getBaseValue(vtype, sv)
	if err != nil {
		return v, err
	}
	v = reflect.Append(v, vv)
	return v, nil
}

func (cp *Cparse) parseMap(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	ktype := v.Type().Key().Kind()
	vtype := v.Type().Elem().Kind()

	fmt.Println("ktype: ", ktype)
	fmt.Println("vtype: ", vtype)

	if ktype != reflect.String {
		return errors.New("map key must string")
	}
	se, k, is, kvs, err := cp.parseMapTag(tv)
	if err != nil {
		return err
	}
	fmt.Printf("se: %s k: %s is: %s kvs: %s\n", se, k, is, kvs)

	word, err := cp.getDatafromIni(se, k, cfg)
	if err != nil {
		return err
	}
	fmt.Println("====", word)
	ilist := strings.Split(word, is)
	fmt.Println("ilist: ", ilist)
	for _, item := range ilist {
		kvlist := strings.Split(item, kvs)
		sk := kvlist[0]
		sv := kvlist[1]
		cp.fillMap(v, sk, sv)
	}
	return nil
}

func (cp *Cparse) parseSlice(v reflect.Value, tv reflect.StructField, cfg *config.Gconf) (reflect.Value, error) {
	vtype := v.Type().Elem().Kind()
	fmt.Println("vtype: ", vtype)

	se, k, is, err := cp.parseSliceTag(tv)
	if err != nil {
		return v, err
	}
	fmt.Printf("se: %s k: %s is: %s\n", se, k, is)
	word, err := cp.getDatafromIni(se, k, cfg)
	if err != nil {
		return v, err
	}
	ilist := strings.Split(word, is)
	for _, item := range ilist {
		v, _ = cp.fillSlice(v, item)
	}
	return v, nil
}

func (cp *Cparse) isUserSelfParse(tv reflect.StructField) (error, bool) {
	dtype := tv.Tag.Get(CP_TAG_DTYPE)
	if dtype == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_DTYPE)
		return errors.New(fmt_e), false
	}

	fmt.Printf("dtype: %s\n", dtype)
	if dtype == CP_DTYPE_BASE {
		return nil, false
	} else if dtype == CP_DTYPE_COMPLEX {
		return nil, true
	} else {
		fmt_e := fmt.Sprintf("dtype val %s not exist", dtype)
		return errors.New(fmt_e), false
	}
	return errors.New("unknown error"), false
}

func (cp *Cparse) CparseGo(stru interface{}, cfg *config.Gconf) error {
	v_stru := reflect.ValueOf(stru).Elem()
	count := v_stru.NumField()
	for i := 0; i < count; i++ {
		t_item := v_stru.Type().Field(i)
		err, isparse := cp.isUserSelfParse(t_item)
		if err != nil {
			return err
		}
		if isparse {
			continue
		}
		item := v_stru.Field(i)
		switch item.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			err := cp.parseInt(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			err := cp.parseUint(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Float32, reflect.Float64:
			err := cp.parseFloat(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.String:
			err := cp.parseString(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Bool:
			err := cp.parseBool(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Map:
			err := cp.parseMap(item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Slice:
			xv, err := cp.parseSlice(item, t_item, cfg)
			if err != nil {
				return err
			}
			v_stru.Field(i).Set(xv)
		}
	}

	for mk, one_func := range cp.Funcs {
		slist := strings.Split(mk, ":")
		se := slist[0]
		k := slist[1]
		cmap, err := cp.parseSection(cfg, se)
		if err != nil {
			return err
		}
		sv := cmap[k]
		err = one_func(stru, sv)
		if err != nil {
			return err
		}
	}
	return nil
}
