package confparse

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	config "github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
)

//type UserSelfFunc func(interface{}, string) error

/*const (
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
}*/

func (cp *Cparse) parseSectionWithCtx(ctx context.Context, cfg *config.Gconf, sectionName string) (map[string]string, error) {
	return cfg.GetSection(sectionName)
}

func (cp *Cparse) parseBaseTagWithCtx(ctx context.Context, v reflect.StructField) (string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)
	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", errors.New(fmt_e)
	}
	logger.Debugf(ctx, "cpos: %s", cpos)
	//fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	return slist[0], slist[1], nil
}

func (cp *Cparse) parseMapTagWithCtx(ctx context.Context, v reflect.StructField) (string, string, string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)
	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", "", "", errors.New(fmt_e)
	}
	logger.Debugf(ctx, "cpos: %s", cpos)
	//fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	se := slist[0]
	k := slist[1]

	is := v.Tag.Get(CP_TAG_ITEM_SPLIT)
	if is == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_ITEM_SPLIT)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", "", "", errors.New(fmt_e)
	}

	ks := v.Tag.Get(CP_TAG_KV_SPLIT)
	if ks == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_KV_SPLIT)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", "", "", errors.New(fmt_e)
	}

	return se, k, is, ks, nil
}

func (cp *Cparse) parseSliceTagWithCtx(ctx context.Context, v reflect.StructField) (string, string, string, error) {
	cpos := v.Tag.Get(CP_TAG_POS)
	if cpos == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_POS)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", "", errors.New(fmt_e)
	}
	logger.Debugf(ctx, "cpos: %s", cpos)

	//fmt.Printf("cpos: %s\n", cpos)
	slist := strings.Split(cpos, ":")
	se := slist[0]
	k := slist[1]

	is := v.Tag.Get(CP_TAG_ITEM_SPLIT)
	if is == "" {
		fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_ITEM_SPLIT)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return "", "", "", errors.New(fmt_e)
	}
	return se, k, is, nil
}

func (cp *Cparse) getDatafromIniWithCtx(ctx context.Context, section string, key string, cfg *config.Gconf) (string, error) {
	if cfg.HasSection(section) {
		item, err := cp.parseSectionWithCtx(ctx, cfg, section)
		if err != nil {
			return "", err
		}
		word, ok := item[key]
		if !ok {
			fmt_e := fmt.Sprintf("section %s key %s not exist", section, key)
			logger.Warnf(ctx, fmt_e)
			return "", errors.New(fmt_e)
		}
		return word, nil
	}
	fmt_e := fmt.Sprintf("section %s not exist", section)
	return "", errors.New(fmt_e)
}

func (cp *Cparse) setIntWithCtx(ctx context.Context, v reflect.Value, x string) error {
	real_v, err := strconv.ParseInt(x, 10, 64)
	if err != nil {
		return err
	}
	v.SetInt(real_v)
	return nil
}

func (cp *Cparse) parseIntWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	//fmt.Printf("section: %s key: %s\n", section, key)
	logger.Debugf(ctx, "section: %s key: %s", section, key)

	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setIntWithCtx(ctx, v, word)
}

func (cp *Cparse) setUintWithCtx(ctx context.Context, v reflect.Value, x string) error {
	real_v, err := strconv.ParseUint(x, 10, 64)
	if err != nil {
		return err
	}
	v.SetUint(real_v)
	return nil
}

func (cp *Cparse) parseUintWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	logger.Debugf(ctx, "section: %s key: %s", section, key)
	//fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setUintWithCtx(ctx, v, word)
}

func (cp *Cparse) setFloatWithCtx(ctx context.Context, v reflect.Value, x string) error {
	real_v, err := strconv.ParseFloat(x, 64)
	if err != nil {
		return err
	}
	v.SetFloat(real_v)
	return nil
}

func (cp *Cparse) parseFloatWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	logger.Debugf(ctx, "section: %s key: %s", section, key)

	//fmt.Printf("section: %s key: %s\n", section, key)
	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setFloatWithCtx(ctx, v, word)
}

func (cp *Cparse) parseDurationWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	//fmt.Printf("section: %s key: %s\n", section, key)
	logger.Debugf(ctx, "section: %s key: %s", section, key)

	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setDurationWithCtx(ctx, v, word)
}

func (cp *Cparse) setDurationWithCtx(ctx context.Context, v reflect.Value, x string) (err error) {
	var durationVal time.Duration
	if durationVal, err = time.ParseDuration(x); err == nil {
		v.Set(reflect.ValueOf(durationVal))
	}
	return
}

func (cp *Cparse) setStringWithCtx(ctx context.Context, v reflect.Value, x string) error {
	v.SetString(x)
	return nil
}

func (cp *Cparse) parseStringWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	//fmt.Printf("section: %s key: %s\n", section, key)

	logger.Debugf(ctx, "section: %s key: %s", section, key)

	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setStringWithCtx(ctx, v, word)
}

func (cp *Cparse) setBoolWithCtx(ctx context.Context, v reflect.Value, x string) error {
	ret, err := strconv.ParseBool(x)
	if err != nil {
		return err
	}
	v.SetBool(ret)
	return nil
}

func (cp *Cparse) parseBoolWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	section, key, err := cp.parseBaseTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	logger.Debugf(ctx, "section: %s key: %s", section, key)

	word, err := cp.getDatafromIniWithCtx(ctx, section, key, cfg)
	if err != nil {
		return err
	}
	return cp.setBoolWithCtx(ctx, v, word)
}

func (cp *Cparse) getBaseValueWithCtx(ctx context.Context, k reflect.Kind, v string) (reflect.Value, error) {
	data := 0
	f := 0.0
	b := true
	s := ""
	switch k {
	case reflect.Int:
		x := reflect.ValueOf(&data)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Int8:
		d := int8(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Int16:
		d := int16(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Int32:
		d := int32(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)

		return x, err

	case reflect.Int64:
		d := int64(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Uint8:
		d := uint8(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Uint16:
		d := uint16(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Uint32:
		d := uint32(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Uint64:
		d := uint64(data)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setIntWithCtx(ctx, x, v)
		return x, err

	case reflect.Float32:
		d := float32(f)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setFloatWithCtx(ctx, x, v)
		return x, err

	case reflect.Float64:
		d := float64(f)
		x := reflect.ValueOf(&d)
		x = x.Elem()
		err := cp.setFloatWithCtx(ctx, x, v)
		return x, err

	case reflect.Bool:
		x := reflect.ValueOf(&b)
		x = x.Elem()
		err := cp.setBoolWithCtx(ctx, x, v)
		return x, err

	case reflect.String:
		x := reflect.ValueOf(&s)
		x = x.Elem()
		err := cp.setStringWithCtx(ctx, x, v)
		return x, err
	}
	return reflect.ValueOf(data), nil
}

func (cp *Cparse) fillMapWithCtx(ctx context.Context, v reflect.Value, sk string, sv string) error {
	vk := reflect.ValueOf(sk)
	vtype := v.Type().Elem().Kind()
	vv, err := cp.getBaseValue(vtype, sv)
	if err != nil {
		return err
	}
	v.SetMapIndex(vk, vv)
	return nil
}

func (cp *Cparse) fillSliceWithCtx(ctx context.Context, v reflect.Value, sv string) (reflect.Value, error) {
	vtype := v.Type().Elem().Kind()
	vv, err := cp.getBaseValue(vtype, sv)
	if err != nil {
		return v, err
	}
	v = reflect.Append(v, vv)
	return v, nil
}

func (cp *Cparse) parseMapWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) error {
	ktype := v.Type().Key().Kind()
	//vtype := v.Type().Elem().Kind()

	//fmt.Println("ktype: ", ktype)
	//fmt.Println("vtype: ", vtype)

	if ktype != reflect.String {
		return errors.New("map key must string")
	}
	se, k, is, kvs, err := cp.parseMapTagWithCtx(ctx, tv)
	if err != nil {
		return err
	}
	//logger.Debufg(ctx, "section: %s key: %s", section, key)

	//fmt.Printf("se: %s k: %s is: %s kvs: %s\n", se, k, is, kvs)
	logger.Debugf(ctx, "se: %s k: %s is: %s kvs: %s", se, k, is, kvs)

	word, err := cp.getDatafromIni(se, k, cfg)
	if err != nil {
		return err
	}
	//fmt.Println("====", word)
	ilist := strings.Split(word, is)
	//fmt.Println("ilist: ", ilist)
	for _, item := range ilist {
		kvlist := strings.Split(item, kvs)
		sk := kvlist[0]
		sv := kvlist[1]
		cp.fillMap(v, sk, sv)
	}
	return nil
}

func (cp *Cparse) parseSliceWithCtx(ctx context.Context, v reflect.Value, tv reflect.StructField, cfg *config.Gconf) (reflect.Value, error) {
	//vtype := v.Type().Elem().Kind()
	//fmt.Println("vtype: ", vtype)

	se, k, is, err := cp.parseSliceTag(tv)
	if err != nil {
		return v, err
	}
	//fmt.Printf("se: %s k: %s is: %s\n", se, k, is)
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

func (cp *Cparse) isUserSelfParseWithCtx(ctx context.Context, tv reflect.StructField) (error, bool) {
	dtype := tv.Tag.Get(CP_TAG_DTYPE)
	if dtype == "" {
		//fmt_e := fmt.Sprintf("tag: %s not exist", CP_TAG_DTYPE)
		//logger.Warnf(ctx, "err: %s", fmt_e)
		//return errors.New(fmt_e), false
		logger.Debugf(ctx, "tag: %s not found", CP_TAG_DTYPE)
		return nil, true
	}

	//fmt.Printf("dtype: %s\n", dtype)
	if dtype == CP_DTYPE_BASE {
		return nil, false
	} else if dtype == CP_DTYPE_COMPLEX {
		return nil, true
	} else {
		fmt_e := fmt.Sprintf("dtype val %s not exist", dtype)
		logger.Warnf(ctx, "err: %s", fmt_e)
		return errors.New(fmt_e), false
	}
	return errors.New("unknown error"), false
}

func (cp *Cparse) CparseGoWithCtx(ctx context.Context, stru interface{}, cfg *config.Gconf) error {
	v_stru := reflect.ValueOf(stru).Elem()
	count := v_stru.NumField()
	for i := 0; i < count; i++ {
		t_item := v_stru.Type().Field(i)
		err, isparse := cp.isUserSelfParseWithCtx(ctx, t_item)
		if err != nil {
			return err
		}
		if isparse {
			continue
		}
		item := v_stru.Field(i)
		switch item.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 支持时间类型
			if item.Type().Name() == "Duration" {
				if err := cp.parseDurationWithCtx(ctx, item, t_item, cfg); err != nil {
					return err
				}
			} else {
				if err := cp.parseIntWithCtx(ctx, item, t_item, cfg); err != nil {
					return err
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			err := cp.parseUintWithCtx(ctx, item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Float32, reflect.Float64:
			err := cp.parseFloatWithCtx(ctx, item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.String:
			err := cp.parseStringWithCtx(ctx, item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Bool:
			err := cp.parseBoolWithCtx(ctx, item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Map:
			err := cp.parseMapWithCtx(ctx, item, t_item, cfg)
			if err != nil {
				return err
			}
		case reflect.Slice:
			xv, err := cp.parseSliceWithCtx(ctx, item, t_item, cfg)
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
		cmap, err := cp.parseSectionWithCtx(ctx, cfg, se)
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
