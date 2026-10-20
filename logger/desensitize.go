package logger

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"unicode"
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

type DesensitizeFunc func(any) any

// jsonLogValue embeds JSON in JSON logs and renders readable JSON in text logs.
// RawMessage alone formats as byte numbers when passed to fmt.Sprint.
type jsonLogValue struct {
	json.RawMessage
}

func (v jsonLogValue) String() string { return string(v.RawMessage) }

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
		keyStr := k.String()
		fieldVal := val.MapIndex(k).Interface()
		if h.sensitiveFieldMap(keyStr) {
			//res[k.String()] = MASKSTR
			if dfunc, ok := Gfilelog.DesensitizeFuncMap[keyStr]; ok {
				//fmt.Println(keyStr)
				//fmt.Println(dfunc(fieldVal))
				res[k.String()] = dfunc(fieldVal)
			} else {
				res[k.String()] = MASKSTR
			}
		} else {
			res[k.String()] = h.Desensitize(fieldVal)
		}
	}
	return res
}

func (h *DesensitizeHandler) IsJSON(s string) bool {
	trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '{' || trimmed[0] == '['
}

func (h *DesensitizeHandler) IsXML(s string) bool {
	trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '<'
}

func (h *DesensitizeHandler) desensitizeString(s string) any {
	// JSON 字符串自动解析脱敏
	if h.IsJSON(s) {
		return h.desensitizeJSON(s)
	}

	// XML 解析脱敏（无正则）
	if h.IsXML(s) {
		var node xmlNode
		if err := xml.Unmarshal([]byte(s), &node); err == nil {
			h.desensitizeXMLNode(&node)
			bs, _ := xml.Marshal(node)
			return string(bs)
		}
	}

	return s
}

func (h *DesensitizeHandler) desensitizeJSON(s string) any {
	decoder := json.NewDecoder(strings.NewReader(s))
	// Keep ID, amount and other numeric tokens exact instead of rounding through
	// float64. Custom masks for JSON numeric fields receive json.Number.
	decoder.UseNumber()
	var obj any
	if err := decoder.Decode(&obj); err != nil {
		return s
	}
	// Decode must consume exactly one value, just like json.Unmarshal.
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return s
	}
	bs, err := json.Marshal(h.Desensitize(obj))
	if err != nil {
		// A custom mask may return an unsupported value. Do not fall back to the
		// original sensitive JSON, or silently emit null for a failed encoding.
		return MASKSTR
	}
	return jsonLogValue{RawMessage: bs}
}

// jsonAttrValue recognizes JSON field values when masking is disabled without
// traversing unrelated maps or structs. Message and metadata attributes bypass
// this conversion in DesensitizeReplaceAttr.
func jsonAttrValue(v any) (any, bool) {
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return nil, false
	}
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, false
		}
		val = val.Elem()
	}
	switch value := val.Interface().(type) {
	case jsonLogValue:
		return value, true
	case json.RawMessage:
		if value == nil {
			return nil, true
		}
		if json.Valid(value) {
			return jsonLogValue{RawMessage: value}, true
		}
		return string(value), true
	}
	if val.Kind() == reflect.String {
		s := val.String()
		h := NewDesensitizeHandler()
		if h.IsJSON(s) && json.Valid([]byte(s)) {
			return jsonLogValue{RawMessage: json.RawMessage(s)}, true
		}
	}
	return nil, false
}

func (h *DesensitizeHandler) desensitizeStruct(val reflect.Value) any {
	res := make(map[string]any)
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		//keyStr := strings.ToLower(field.Name)
		keyStr := field.Name
		if !val.Field(i).CanInterface() {
			res[keyStr] = val.Field(i)
			continue
		}
		fieldVal := val.Field(i).Interface()
		if h.sensitiveFieldMap(keyStr) {
			if dfunc, ok := Gfilelog.DesensitizeFuncMap[keyStr]; ok {
				//fmt.Println(keyStr)
				//fmt.Println(dfunc(fieldVal))
				res[field.Name] = dfunc(fieldVal)
			} else {
				res[field.Name] = MASKSTR
			}
		} else {
			res[field.Name] = h.Desensitize(fieldVal)
		}
	}
	return res
}

func (h *DesensitizeHandler) desensitizeXMLNode(n *xmlNode) {
	//name := strings.ToLower(n.XMLName.Local)
	name := n.XMLName.Local
	//fmt.Println(name)
	if h.sensitiveFieldMap(name) {
		//n.Content = MASKSTR
		if dfunc, ok := Gfilelog.DesensitizeFuncMap[name]; ok {
			n.Content = dfunc(n.Content).(string)
		} else {
			n.Content = MASKSTR
		}
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
	//error不处理
	if val.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return v
	}
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	switch value := val.Interface().(type) {
	case json.Number:
		return value
	case json.RawMessage:
		if value == nil {
			return nil
		}
		return h.desensitizeJSON(string(value))
	case jsonLogValue:
		return h.desensitizeJSON(string(value.RawMessage))
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
	h := NewDesensitizeHandler()
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.MessageKey,
		slog.SourceKey, IGNORE_CALL_CLIENT_SERVIC, IGNORE_CLIENT_SERVICE,
		IGNORE_TRACE_DEPTH, IGNORE_SRC_FIle, IGNORE_FN,
		IGNORE_COST, IGNORE_REQUEST_ID, IGNORE_TRACE_ID:
		return a
	}
	if Gfilelog.Logconf.DesensitizeField == "" {
		if value, ok := jsonAttrValue(a.Value.Any()); ok {
			a.Value = slog.AnyValue(value)
		}
		return a
	}
	if h.sensitiveFieldMap(a.Key) {
		if dfunc, ok := Gfilelog.DesensitizeFuncMap[a.Key]; ok {
			a.Value = slog.AnyValue(dfunc(a.Value.Any()))
		} else {
			a.Value = slog.AnyValue(MASKSTR)
		}
	} else {
		a.Value = slog.AnyValue(h.Desensitize(a.Value.Any()))
	}
	return a
}
