package confparse

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	config "github.com/lanwenhong/lgobase/gconfig"
	"github.com/lanwenhong/lgobase/logger"
)

type ExtendConf struct {
	Sk    string
	Key   string
	Index int
}

func ParseExt(ctx context.Context, section string, key string, index int, cfg *config.Gconf) ([]map[string]string, error) {
	qk := key + " = " + cfg.Gcf[section][key][index]
	return cfg.GlineExtend[key][qk], nil
}

func NewExtendConf(sec, key string, index int) *ExtendConf {
	return &ExtendConf{
		Sk:    sec,
		Key:   key,
		Index: index,
	}
}

func (ec *ExtendConf) setFieldValue(field reflect.Value, strVal string) error {
	if !field.CanSet() {
		return errors.New("字段不可设置")
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(strVal)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type().Name() == "Duration" {
			var durationVal time.Duration
			var err error
			if durationVal, err = time.ParseDuration(strVal); err == nil {
				field.Set(reflect.ValueOf(durationVal))
			}
		} else {
			num, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(num)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		num, err := strconv.ParseUint(strVal, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(num)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(strVal)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)

	default:
		return errors.New("不支持的类型: " + field.Kind().String())
	}
	return nil
}

func (ec *ExtendConf) MapToStruct(data map[string]string, obj interface{}) error {
	// 必须是结构体指针
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("必须传入非空结构体指针")
	}

	elemVal := val.Elem()
	elemType := elemVal.Type()

	// 遍历结构体字段
	for i := 0; i < elemVal.NumField(); i++ {
		field := elemVal.Field(i)
		sField := elemType.Field(i)

		// 读取 mapkey tag
		mapKey := sField.Tag.Get("mapkey")
		if mapKey == "" {
			continue
		}

		// 从 map[string]string 取值
		strVal, ok := data[mapKey]
		if !ok {
			continue // 不存在就跳过
		}
		// 自动类型转换并赋值
		if err := ec.setFieldValue(field, strVal); err != nil {
			return errors.New(fmt.Sprintf("字段 %s 解析失败: %w", sField.Name, err))
		}
	}
	return nil
}

func (ec *ExtendConf) ParseExtStru(ctx context.Context, stru interface{}, cfg *config.Gconf) error {
	if v, ok := cfg.GlineExtend[ec.Sk]; ok {
		logger.Debug(ctx, "confparse extend", "v", v)
		if vlist, ok := v[ec.Key]; ok {
			if ec.Index >= len(vlist) {
				logger.Warn(ctx, "confparse extend error", "ec.Index", ec.Index, "vlist len", len(vlist))
				return errors.New(fmt.Sprintf("Index: %d vlen: %d", ec.Index, len(vlist)))

			}
			return ec.MapToStruct(vlist[ec.Index], stru)
		}
		return errors.New(fmt.Sprintf("not found %s", ec.Key))
	}
	return errors.New(fmt.Sprintf("not found %s", ec.Sk))
}
