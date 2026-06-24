package gconfig_v2

import "regexp"

type ValueType struct {
	floatRegex *regexp.Regexp
	intRegex   *regexp.Regexp
}

func NewValueType() *ValueType {
	vt := &ValueType{
		floatRegex: regexp.MustCompile(`^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?$`),
		intRegex:   regexp.MustCompile(`^[+-]?\d+$`),
	}

	return vt
}

func (vt *ValueType) IsFloat(s string) bool {
	return vt.floatRegex.MatchString(s)
}

func (vt *ValueType) IsInteger(s string) bool {
	return vt.intRegex.MatchString(s)
}
