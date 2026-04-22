package logger

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

const (
	IGNORE_CALL_CLIENT_SERVIC = "call_client_service"
	IGNORE_CLIENT_SERVICE     = "client_service"
	IGNORE_TRACE_DEPTH        = "trace_depth"
	IGNORE_SRC_FIle           = "srcFile"
	IGNORE_FN                 = "fn"
	IGNORE_REQUEST_ID         = "request_id"
	IGNORE_TRACE_ID           = "trace_id"
	IGNORE_COST               = "cost"
	MASKSTR                   = "******"
)

type xmlNode struct {
	XMLName xml.Name
	Content string    `xml:",chardata"`
	Nodes   []xmlNode `xml:",any"`
}

type DesensitizeHandler struct {
	DesensitizeFieldMap map[string]bool
}

func NewDesensitizeHandler() *DesensitizeHandler {
	d := &DesensitizeHandler{}
	return d
}

func (h *DesensitizeHandler) sensitiveFieldMap(k string) bool {
	return Gfilelog.DesensitizeFieldMap[k]
}

func (h *DesensitizeHandler) desensitizeMap(val reflect.Value) any {
	res := make(map[string]any)
	for _, k := range val.MapKeys() {
		keyStr := strings.ToLower(k.String())
		fieldVal := val.MapIndex(k).Interface()

		if h.sensitiveFieldMap(keyStr) {
			res[k.String()] = MASKSTR
		} else {
			//res[k.String()] = h.Desensitize(fieldVal)
			res[k.String()] = fieldVal
		}
	}
	return res
}

func (h *DesensitizeHandler) desensitizeString(s string) any {
	// JSON 字符串自动解析脱敏
	if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
		var obj any
		if err := json.Unmarshal([]byte(s), &obj); err == nil {
			masked := h.Desensitize(obj)
			bs, _ := json.Marshal(masked)
			//return string(bs)
			return json.RawMessage(bs)
		} else {
			fmt.Println(err)
		}
	}

	// XML 解析脱敏（无正则）
	if strings.Contains(s, "<") && strings.Contains(s, ">") {
		var node xmlNode
		if err := xml.Unmarshal([]byte(s), &node); err == nil {
			h.desensitizeXMLNode(&node)
			bs, _ := xml.Marshal(node)
			return string(bs)
		}
	}
	//return MASKSTR
	return s
}

func (h *DesensitizeHandler) desensitizeStruct(val reflect.Value) any {
	res := make(map[string]any)
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		keyStr := strings.ToLower(field.Name)
		fieldVal := val.Field(i).Interface()

		if h.sensitiveFieldMap(keyStr) {
			res[field.Name] = MASKSTR
		} else {
			res[field.Name] = h.Desensitize(fieldVal)
		}
	}
	return res
}

func (h *DesensitizeHandler) desensitizeXMLNode(n *xmlNode) {
	name := strings.ToLower(n.XMLName.Local)
	if h.sensitiveFieldMap(name) {
		n.Content = MASKSTR
		return
	}
	for i := range n.Nodes {
		h.desensitizeXMLNode(&n.Nodes[i])
	}
}

func (h *DesensitizeHandler) desensitizeSlice(val reflect.Value) any {
	result := make([]any, 0, val.Len())
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i).Interface()
		result = append(result, h.Desensitize(item))
	}
	return result
}

func (h *DesensitizeHandler) Desensitize(v any) any {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		return h.desensitizeString(val.String())
	case reflect.Map:
		return h.desensitizeMap(val)
	case reflect.Struct:
		return h.desensitizeStruct(val)
	case reflect.Slice, reflect.Array:
		return h.desensitizeSlice(val)
	default:
		return v
	}
}

func DesensitizeReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.MessageKey,
		slog.SourceKey, IGNORE_CALL_CLIENT_SERVIC, IGNORE_CLIENT_SERVICE,
		IGNORE_TRACE_DEPTH, IGNORE_SRC_FIle, IGNORE_FN,
		IGNORE_COST, IGNORE_REQUEST_ID, IGNORE_TRACE_ID:
		return a
	}
	//fmt.Println("=========================", a.Key)
	h := NewDesensitizeHandler()
	if h.sensitiveFieldMap(a.Key) {
		a.Value = slog.AnyValue(MASKSTR)
	} else {
		a.Value = slog.AnyValue(h.Desensitize(a.Value.Any()))
	}
	//a.Value = slog.AnyValue(h.Desensitize(a))
	//}
	return a
}
