package gconfig_v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/lanwenhong/lgobase/logger"
)

const (
	NODE_TYPE_UNKNOWN   = 0
	NODE_TYPE_MAP       = 1
	NODE_TYPE_SLICE     = 2
	NODE_TYPE_STRING    = 3
	NODE_TYPE_FLOAT     = 4
	NODE_TYPE_INT       = 5
	NODE_TYPE_TIMESTAMP = 6
	NODE_TYPE_BOOL      = 7
	NODE_TYPE_NULL      = 8
	NODE_TYPE_DURATION  = 9
)

/*type Token struct {
	line     string
	value    string
	key      string
	nodeType int
	indent   int
}*/

type AstNode struct {
	line     string
	m        map[string]interface{}
	s        []interface{}
	indent   int
	nodeName string
	nodeType int
}

type ParseYaml struct {
	confFile string
	content  []byte
	stack    []*AstNode
}

var (
	timeType     = reflect.TypeOf(time.Time{})
	durationType = reflect.TypeOf(time.Duration(0))
)

func (asn *AstNode) NodeType2String(ctx context.Context) string {
	//fmt.Println("------", *asn.nt)
	switch asn.effectiveNodeType() {
	//switch *asn.nt {
	case NODE_TYPE_MAP:
		return "MAP"
	case NODE_TYPE_SLICE:
		return "SLICE"
	}
	return ""
}

func (asn *AstNode) effectiveNodeType() int {
	if asn.nodeType != NODE_TYPE_UNKNOWN {
		return asn.nodeType
	}
	if len(asn.m) > 0 {
		return NODE_TYPE_MAP
	}
	if len(asn.s) > 0 {
		return NODE_TYPE_SLICE
	}
	return NODE_TYPE_UNKNOWN
}

func (asn *AstNode) GetMap(ctx context.Context) map[string]interface{} {
	return asn.m
}

func (asn *AstNode) GetIndent(ctx context.Context) int {
	return asn.indent
}

func NewParseYaml(file string) *ParseYaml {
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("读取文件失败:", err)
		return nil
	}
	return newParseYaml(file, content)
}

func NewParseYamlBytes(content []byte) *ParseYaml {
	return newParseYaml("", append([]byte(nil), content...))
}

func newParseYaml(file string, content []byte) *ParseYaml {
	return &ParseYaml{
		confFile: file,
		content:  content,
	}
}

func (py *ParseYaml) errLine(line string) error {
	errMsg := fmt.Sprintf("line %s format not surport", line)
	err := errors.New(errMsg)
	return err
}

func (py *ParseYaml) errDuplicateKey(line, key string) error {
	return fmt.Errorf("line %s duplicate key %s", line, key)
}

func (py *ParseYaml) errMixedNode(line string) error {
	return fmt.Errorf("line %s can not mix map and slice items", line)
}

func (py *ParseYaml) errUnexpectedIndent(line string) error {
	return fmt.Errorf("line %s unexpected indent", line)
}

func (py *ParseYaml) lineIndent(line string) (int, error) {
	indent := 0
	for _, c := range line {
		switch c {
		case ' ':
			indent++
		case '\t':
			return 0, fmt.Errorf("line %s tab indentation not supported", line)
		default:
			return indent, nil
		}
	}
	return indent, nil
}

func (py *ParseYaml) isSliceFlag(line string) bool {
	return strings.HasPrefix(line, "- ")
}

func (py *ParseYaml) splitMapLine(line string) (string, string, bool, error) {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
		case inDouble:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inDouble = false
			}
		default:
			switch c {
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case ':':
				return line[:i], line[i+1:], true, nil
			}
		}
	}
	if inSingle || inDouble {
		return "", "", false, fmt.Errorf("line %s key quote not closed", line)
	}
	return "", "", false, nil
}

func (py *ParseYaml) normalizeMapKey(k string) (string, error) {
	k = strings.TrimSpace(k)
	if k == "" {
		return "", nil
	}
	if strings.HasPrefix(k, "'") || strings.HasPrefix(k, "\"") {
		lex := NewLexer(k)
		tok := lex.NextToken()
		if lex.err != nil {
			return "", lex.err
		}
		lex.skipWhitespace()
		if tok.Type != TokenString || lex.pos != lex.end {
			return "", fmt.Errorf("invalid quoted key %s", k)
		}
		k = tok.Value
	}
	if strings.TrimSpace(k) == "" {
		return "", nil
	}
	return k, nil
}

func (py *ParseYaml) matchMap(ctx context.Context, line string) (string, string, bool, error) {
	k, v, ok, err := py.splitMapLine(line)
	if err != nil || !ok {
		return "", "", ok, err
	}
	k, err = py.normalizeMapKey(k)
	if err != nil {
		return "", "", true, err
	}
	return k, strings.TrimSpace(v), true, nil
}

func (py *ParseYaml) matchSlice(ctx context.Context, line string) ([]string, bool) {
	if !strings.HasPrefix(line, "- ") {
		return []string{}, false
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if body == "" {
		return []string{}, false
	}

	inSingle := false
	inDouble := false
	flowDepth := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(body) && body[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
		case inDouble:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inDouble = false
			}
		default:
			switch c {
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case '{', '[':
				flowDepth++
			case '}', ']':
				if flowDepth > 0 {
					flowDepth--
				}
			case ':':
				if flowDepth == 0 && (i+1 == len(body) || body[i+1] == ' ' || body[i+1] == '\t') {
					return []string{line, "- ", body[:i], ":", strings.TrimSpace(body[i+1:])}, true
				}
			}
		}
	}
	return []string{line, "- ", body, "", ""}, true
}

func (py *ParseYaml) countIndent(line string) int {
	indent := 0
	for _, c := range line {
		if c == ' ' {
			indent++
			continue
		}
		break
	}
	return indent
}

func (py *ParseYaml) parseBlockScalarHeader(value string) (byte, byte, int, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "!!str ") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "!!str"))
	}
	if value == "" {
		return 0, 0, 0, false
	}
	style := value[0]
	if style != '|' && style != '>' {
		return 0, 0, 0, false
	}

	chomp := byte(0)
	indentIndicator := 0
	for i := 1; i < len(value); i++ {
		switch c := value[i]; {
		case c == '-' || c == '+':
			if chomp != 0 {
				return 0, 0, 0, false
			}
			chomp = c
		case c >= '1' && c <= '9':
			if indentIndicator != 0 {
				return 0, 0, 0, false
			}
			indentIndicator = int(c - '0')
		default:
			return 0, 0, 0, false
		}
	}
	return style, chomp, indentIndicator, true
}

func (py *ParseYaml) isInvalidBlockScalarHeader(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "!!str ") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "!!str"))
	}
	if value == "" || (value[0] != '|' && value[0] != '>') {
		return false
	}
	_, _, _, ok := py.parseBlockScalarHeader(value)
	return !ok
}

func (py *ParseYaml) errBlockScalarHeader(line string) error {
	return fmt.Errorf("line %s invalid block scalar header", line)
}

func (py *ParseYaml) unsupportedLineFeature(trimmed string) (string, bool) {
	switch {
	case trimmed == "---":
		return "document start", true
	case trimmed == "...":
		return "document end", true
	case strings.HasPrefix(trimmed, "? "):
		return "complex key", true
	}
	return "", false
}

func (py *ParseYaml) unsupportedMapKey(k string) (string, bool) {
	k = strings.TrimSpace(k)
	switch {
	case k == "<<":
		return "merge key", true
	case strings.HasPrefix(k, "&"):
		return "anchor", true
	case strings.HasPrefix(k, "*"):
		return "alias", true
	case strings.HasPrefix(k, "!"):
		return "tag", true
	}
	return "", false
}

func (py *ParseYaml) putMapItem(line, key string, m map[string]interface{}, value interface{}) error {
	if _, ok := m[key]; ok {
		return py.errDuplicateKey(line, key)
	}
	m[key] = value
	return nil
}

func (py *ParseYaml) normalizeBlockLines(lines []string, minIndent int) []string {
	res := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			res = append(res, "")
			continue
		}
		if len(line) >= minIndent {
			res = append(res, line[minIndent:])
			continue
		}
		res = append(res, strings.TrimLeft(line, " "))
	}
	return res
}

func (py *ParseYaml) applyBlockChomp(value string, chomp byte) string {
	switch chomp {
	case '-':
		return strings.TrimRight(value, "\n")
	case '+':
		return value
	default:
		if value == "" {
			return value
		}
		return strings.TrimRight(value, "\n") + "\n"
	}
}

func (py *ParseYaml) renderBlockScalar(lines []string, style byte, chomp byte) string {
	if len(lines) == 0 {
		return ""
	}
	if style == '|' {
		return py.applyBlockChomp(strings.Join(lines, "\n")+"\n", chomp)
	}

	var b strings.Builder
	prevText := false
	for _, line := range lines {
		if line == "" {
			b.WriteByte('\n')
			prevText = false
			continue
		}
		if prevText {
			b.WriteByte(' ')
		}
		b.WriteString(line)
		prevText = true
	}
	b.WriteByte('\n')
	return py.applyBlockChomp(b.String(), chomp)
}

func (py *ParseYaml) collectBlockScalar(lines []string, start int, parentIndent int, style byte, chomp byte, indentIndicator int) (string, int) {
	blockLines := make([]string, 0)
	minIndent := -1
	next := start + 1
	for next < len(lines) {
		line := lines[next]
		trimmed := strings.TrimSpace(line)
		indent := py.countIndent(line)
		if trimmed != "" && indent <= parentIndent {
			break
		}
		blockLines = append(blockLines, line)
		if trimmed != "" && (minIndent == -1 || indent < minIndent) {
			minIndent = indent
		}
		next++
	}
	if minIndent == -1 {
		minIndent = parentIndent + 1
	}
	if indentIndicator > 0 {
		minIndent = parentIndent + indentIndicator
	}
	return py.renderBlockScalar(py.normalizeBlockLines(blockLines, minIndent), style, chomp), next
}

func (py *ParseYaml) newParsedToken(ctx context.Context, line, k, v string, indent int, rawStr bool) (*TokenNode, error) {
	token := &TokenNode{
		line:   line,
		value:  v,
		key:    k,
		indent: indent,
		rawStr: rawStr,
	}
	if err := token.TokenParse(ctx); err != nil {
		return nil, err
	}
	return token, nil
}

func (py *ParseYaml) feedSliceScalar(ctx context.Context, line, v string, pNode *AstNode, indent int, rawStr bool) error {
	if pNode.nodeType == NODE_TYPE_UNKNOWN {
		pNode.nodeType = NODE_TYPE_SLICE
	} else if pNode.nodeType != NODE_TYPE_SLICE {
		return py.errMixedNode(line)
	}
	token, err := py.newParsedToken(ctx, line, "", v, indent, rawStr)
	if err != nil {
		return err
	}
	pNode.s = append(pNode.s, token)
	return nil
}

func (py *ParseYaml) feedSlice(ctx context.Context, line string, group []string, pNode *AstNode, indent int) error {
	var err error = nil
	k := strings.Trim(group[2], " ")
	v := strings.Trim(group[4], " ")
	//sliceTag := strings.TrimSpace(group[1])

	split := strings.TrimSpace(group[3])

	if k == "" {
		errMsg := fmt.Sprintf("parse line:%s key is empty", line)
		logger.Warn(ctx, "gconfig_v2", "err", errMsg)
		return errors.New(errMsg)
	}

	if pNode.nodeType == NODE_TYPE_UNKNOWN {
		pNode.nodeType = NODE_TYPE_SLICE
	} else if pNode.nodeType != NODE_TYPE_SLICE {
		return py.errMixedNode(line)
	}
	if len(pNode.s) > 0 {
		switch last := pNode.s[len(pNode.s)-1].(type) {
		case *TokenNode:
			if indent > last.indent {
				return py.errUnexpectedIndent(line)
			}
		}
	}

	logger.Debug(ctx, "gconfig_v2", "k", k, "v", v)
	if k != "" && v == "" && split != "" {
		node := &AstNode{
			line:     line,
			m:        make(map[string]interface{}),
			s:        make([]interface{}, 0, 10),
			indent:   indent,
			nodeName: k,
			nodeType: NODE_TYPE_UNKNOWN,
		}
		py.stack = append(py.stack, node)
		pNode.s = append(pNode.s, node)
	} else if k != "" && v != "" && split != "" {
		token, tokenErr := py.newParsedToken(ctx, line, k, v, indent+2, false)
		if tokenErr != nil {
			return tokenErr
		}
		node := &AstNode{
			line:     line,
			m:        make(map[string]interface{}),
			s:        make([]interface{}, 0, 10),
			indent:   indent,
			nodeName: k,
			nodeType: NODE_TYPE_MAP,
		}
		node.m[k] = token
		pNode.s = append(pNode.s, node)
	} else if k != "" && v == "" && split == "" {
		logger.Debug(ctx, "gconfig_v2", "k", k)
		token, tokenErr := py.newParsedToken(ctx, line, "", k, indent, false)
		if tokenErr != nil {
			return tokenErr
		}
		pNode.s = append(pNode.s, token)
	} else {
		err = py.errLine(line)
		logger.Warn(ctx, "gconfig_v2", "err", err.Error())
	}
	return err
}

func (py *ParseYaml) feedMapValue(ctx context.Context, line, k, v string, pNode *AstNode, indent int, rawStr bool) error {
	token, err := py.newParsedToken(ctx, line, k, v, indent, rawStr)
	if err != nil {
		return err
	}
	switch pNode.nodeType {
	case NODE_TYPE_UNKNOWN:
		pNode.nodeType = NODE_TYPE_MAP
		logger.Debug(ctx, "gconfig_v2", "line", line, "pline", pNode.line, "pNode", pNode.nodeType)
		return py.putMapItem(line, k, pNode.m, token)
	case NODE_TYPE_MAP:
		return py.putMapItem(line, k, pNode.m, token)
	case NODE_TYPE_SLICE:
		slen := len(pNode.s)
		if slen == 0 {
			return py.errLine(line)
		}
		sNode, ok := pNode.s[slen-1].(*AstNode)
		if !ok {
			return py.errMixedNode(line)
		}
		if indent <= sNode.indent {
			return py.errMixedNode(line)
		}
		return py.putMapItem(line, k, sNode.m, token)
	default:
		return py.errLine(line)
	}
	return nil
}

func (py *ParseYaml) feedMap(ctx context.Context, line, k, v string, pNode *AstNode, indent int) error {
	if k == "" {
		errMsg := fmt.Sprintf("parse line:%s key is empty", line)
		return errors.New(errMsg)
	}
	if feature, ok := py.unsupportedMapKey(k); ok {
		return errUnsupportedYAMLFeature(feature)
	}
	var err error = nil
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if k != "" && v == "" {
		//create new node
		node := &AstNode{
			line:     line,
			m:        make(map[string]interface{}),
			s:        make([]interface{}, 0, 10),
			indent:   indent,
			nodeName: k,
			nodeType: NODE_TYPE_UNKNOWN,
		}
		switch pNode.nodeType {
		case NODE_TYPE_UNKNOWN:
			pNode.nodeType = NODE_TYPE_MAP
			err = py.putMapItem(line, k, pNode.m, node)
		case NODE_TYPE_MAP:
			err = py.putMapItem(line, k, pNode.m, node)
		case NODE_TYPE_SLICE:
			slen := len(pNode.s)
			if slen == 0 {
				return py.errLine(line)
			}
			sNode, ok := pNode.s[slen-1].(*AstNode)
			if !ok {
				return py.errMixedNode(line)
			}
			if indent <= sNode.indent {
				return py.errMixedNode(line)
			}
			err = py.putMapItem(line, k, sNode.m, node)
		default:
			err = py.errLine(line)
		}
		if err == nil {
			py.stack = append(py.stack, node)
		}
	} else if k != "" && v != "" {
		err = py.feedMapValue(ctx, line, k, v, pNode, indent, false)
	} else {
		err = py.errLine(line)
	}
	return err
}

func (py *ParseYaml) splitYamlLines(content []byte) []string {
	s := strings.ReplaceAll(string(content), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func (py *ParseYaml) stripInlineComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
		case inDouble:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inDouble = false
			}
		default:
			switch c {
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case '#':
				if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
					return strings.TrimRight(line[:i], " \t")
				}
			}
		}
	}
	return line
}

func (py *ParseYaml) parse(ctx context.Context) (map[string]interface{}, error) {
	root := make(map[string]interface{})
	lines := py.splitYamlLines(py.content)
	rootNode := &AstNode{
		m:        make(map[string]interface{}),
		s:        make([]interface{}, 0, 10),
		indent:   -1,
		nodeName: "root",
		//nodeType: NODE_TYPE_UNKNOWN,
		nodeType: NODE_TYPE_MAP,
	}
	root["root"] = rootNode
	py.stack = []*AstNode{rootNode}

	var err error = nil
	lastIndent := -1
	lastMayNest := false
	for i := 0; i < len(lines); i++ {
		line := py.stripInlineComment(lines[i])
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if feature, ok := py.unsupportedLineFeature(trimmed); ok {
			return nil, errUnsupportedYAMLFeature(feature)
		}
		indent, err := py.lineIndent(line)
		if err != nil {
			return nil, err
		}
		if lastIndent < 0 && indent > 0 {
			return nil, py.errUnexpectedIndent(line)
		}
		if lastIndent >= 0 && indent > lastIndent && !lastMayNest {
			return nil, py.errUnexpectedIndent(line)
		}
		currentMayNest := false

		//find node
		for len(py.stack) > 0 && py.stack[len(py.stack)-1].indent >= indent {
			py.stack = py.stack[:len(py.stack)-1]
		}
		parentNode := py.stack[len(py.stack)-1]
		if !py.isSliceFlag(trimmed) {
			lk, lv, match, matchErr := py.matchMap(ctx, line)
			if matchErr != nil {
				return nil, matchErr
			}
			if match {
				logger.Debug(ctx, "gconfig_v2", "line", line, "indent", indent)
				if feature, ok := py.unsupportedMapKey(lk); ok {
					err = errUnsupportedYAMLFeature(feature)
				} else if feature, ok := unsupportedPlainScalar(lv); ok {
					err = errUnsupportedYAMLFeature(feature)
				} else if style, chomp, indentIndicator, ok := py.parseBlockScalarHeader(lv); ok {
					value, next := py.collectBlockScalar(lines, i, indent, style, chomp, indentIndicator)
					err = py.feedMapValue(ctx, line, lk, value, parentNode, indent, true)
					i = next - 1
					currentMayNest = false
				} else if py.isInvalidBlockScalarHeader(lv) {
					err = py.errBlockScalarHeader(line)
				} else {
					err = py.feedMap(ctx, line, lk, lv, parentNode, indent)
					currentMayNest = strings.TrimSpace(lv) == ""
				}
			} else {
				err = py.errLine(line)
				logger.Warn(ctx, "gconfig_v2", "err", err.Error())
			}
		} else {
			body := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if style, chomp, indentIndicator, ok := py.parseBlockScalarHeader(body); ok {
				value, next := py.collectBlockScalar(lines, i, indent, style, chomp, indentIndicator)
				err = py.feedSliceScalar(ctx, line, value, parentNode, indent, true)
				i = next - 1
				lastIndent = indent
				lastMayNest = false
				continue
			}
			if py.isInvalidBlockScalarHeader(body) {
				err = py.errBlockScalarHeader(line)
			} else if feature, ok := unsupportedPlainScalar(body); ok {
				err = errUnsupportedYAMLFeature(feature)
			} else {
				group, match := py.matchSlice(ctx, trimmed)
				if match {
					if feature, ok := py.unsupportedMapKey(strings.TrimSpace(group[2])); ok {
						err = errUnsupportedYAMLFeature(feature)
					} else if feature, ok := unsupportedPlainScalar(strings.TrimSpace(group[4])); ok {
						err = errUnsupportedYAMLFeature(feature)
					} else if style, chomp, indentIndicator, ok := py.parseBlockScalarHeader(strings.TrimSpace(group[4])); ok {
						value, next := py.collectBlockScalar(lines, i, indent, style, chomp, indentIndicator)
						group = append([]string(nil), group...)
						group[4] = value
						if parentNode.nodeType == NODE_TYPE_UNKNOWN {
							parentNode.nodeType = NODE_TYPE_SLICE
						}
						token, tokenErr := py.newParsedToken(ctx, line, strings.Trim(group[2], " "), value, indent+2, true)
						if tokenErr != nil {
							return nil, tokenErr
						}
						node := &AstNode{
							line:     line,
							m:        make(map[string]interface{}),
							s:        make([]interface{}, 0, 10),
							indent:   indent,
							nodeName: strings.Trim(group[2], " "),
							nodeType: NODE_TYPE_MAP,
						}
						node.m[strings.Trim(group[2], " ")] = token
						parentNode.s = append(parentNode.s, node)
						i = next - 1
						currentMayNest = false
					} else if py.isInvalidBlockScalarHeader(strings.TrimSpace(group[4])) {
						err = py.errBlockScalarHeader(line)
					} else {
						err = py.feedSlice(ctx, line, group, parentNode, indent)
						currentMayNest = strings.TrimSpace(group[3]) == ":"
					}
				} else {
					err = py.errLine(line)
					logger.Warn(ctx, "gconfig_v2", "err", err.Error())
				}
			}
		}
		if err != nil {
			return nil, err
		}
		lastIndent = indent
		lastMayNest = currentMayNest
	}
	return root, err
}

func PrintAstTree(ctx context.Context, rootNode *AstNode) {
	switch rootNode.effectiveNodeType() {
	case NODE_TYPE_MAP:
		for _, v := range rootNode.m {
			switch v.(type) {
			case *TokenNode:
				token := v.(*TokenNode)
				space := strings.Repeat(" ", token.indent)
				t := token.NodeType2String(ctx)
				fmt.Println(fmt.Sprintf("%s%s: %s", space, token.key, token.value))
				if t != "" {
					fmt.Println(fmt.Sprintf("%s%s", space, t))
				}
			case *AstNode:
				astNode := v.(*AstNode)
				space := ""
				for i := 0; i < astNode.GetIndent(ctx); i++ {
					space += " "
				}
				t := astNode.NodeType2String(ctx)
				fmt.Println(astNode.line)
				fmt.Println(space + t)
				PrintAstTree(ctx, astNode)
			}
		}
	case NODE_TYPE_SLICE:
		for _, v := range rootNode.s {
			switch v.(type) {
			case *TokenNode:
				token := v.(*TokenNode)
				indent := strings.Repeat(" ", token.indent)
				text := token.key
				if token.value != "" {
					if token.key != "" {
						text = fmt.Sprintf("%s: %s", token.key, token.value)
					} else {
						text = token.value
					}
				}
				t := token.NodeType2String(ctx)
				fmt.Println(fmt.Sprintf("%s%s", indent, text))
				if t != "" {
					fmt.Println(fmt.Sprintf("%s%s", indent, t))
				}
			case *AstNode:
				astNode := v.(*AstNode)
				space := ""
				for i := 0; i < astNode.GetIndent(ctx); i++ {
					space += " "
				}
				t := astNode.NodeType2String(ctx)
				fmt.Println(astNode.line)
				fmt.Println(space + t)
				PrintAstTree(ctx, astNode)
				//case map[string]interface{}:
			}
		}
	}
}

func (node *AstNode) isNamedSliceNode() bool {
	line := strings.TrimSpace(node.line)
	if !strings.HasPrefix(line, "- ") {
		return false
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	return strings.HasSuffix(body, ":") && strings.TrimSpace(strings.TrimSuffix(body, ":")) == node.nodeName
}

func (node *AstNode) toMap() (map[string]any, error) {
	res := make(map[string]any, len(node.m))
	for k, v := range node.m {
		switch item := v.(type) {
		case *TokenNode:
			res[k] = item.toAny()
		case *AstNode:
			val, err := item.toAny()
			if err != nil {
				return nil, err
			}
			res[k] = val
		default:
			res[k] = item
		}
	}
	return res, nil
}

func (node *AstNode) toSlice() ([]any, error) {
	res := make([]any, 0, len(node.s))
	for _, v := range node.s {
		switch item := v.(type) {
		case *TokenNode:
			res = append(res, item.toAny())
		case *AstNode:
			val, err := item.toAny()
			if err != nil {
				return nil, err
			}
			if item.isNamedSliceNode() {
				res = append(res, map[string]any{item.nodeName: val})
			} else {
				res = append(res, val)
			}
		default:
			res = append(res, item)
		}
	}
	return res, nil
}

func (node *AstNode) toAny() (any, error) {
	switch node.nodeType {
	case NODE_TYPE_MAP:
		return node.toMap()
	case NODE_TYPE_SLICE:
		return node.toSlice()
	case NODE_TYPE_UNKNOWN:
		if len(node.m) > 0 {
			return node.toMap()
		}
		if len(node.s) > 0 {
			return node.toSlice()
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported ast node type %d", node.nodeType)
	}
}

func (py *ParseYaml) parseToMap(ctx context.Context) (map[string]any, error) {
	root, err := py.parse(ctx)
	if err != nil {
		return nil, err
	}
	rootNode, ok := root["root"].(*AstNode)
	if !ok {
		return nil, errors.New("root ast node not found")
	}
	obj, err := rootNode.toAny()
	if err != nil {
		return nil, err
	}
	m, ok := obj.(map[string]any)
	if !ok {
		return nil, errors.New("root ast node is not map")
	}
	return m, nil
}

func (py *ParseYaml) tagNames() []string {
	return []string{"gconfig", "yaml", "json"}
}

func (py *ParseYaml) fieldKey(field reflect.StructField) (string, bool) {
	for _, tagName := range py.tagNames() {
		tag := field.Tag.Get(tagName)
		if tag == "" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			return "", true
		}
		if name != "" {
			return name, false
		}
	}
	return field.Name, false
}

func (py *ParseYaml) lookupMapValue(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	if key == "" {
		return nil, false
	}
	lowerFirst := strings.ToLower(key[:1]) + key[1:]
	if v, ok := m[lowerFirst]; ok {
		return v, true
	}
	return nil, false
}

func (py *ParseYaml) decode(ctx context.Context, data any, m map[string]any) error {
	if data == nil {
		return errors.New("data must not be nil")
	}
	val := reflect.ValueOf(data)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("data must be a non-nil pointer")
	}
	elem := val.Elem()
	switch elem.Kind() {
	case reflect.Map:
		return py.setMapValue("root", elem, m)
	case reflect.Struct:
		return py.setStructValue("root", elem, m)
	default:
		return fmt.Errorf("data must point to map or struct, got %s", elem.Kind())
	}
}

func (py *ParseYaml) setStructValue(path string, dst reflect.Value, src map[string]any) error {
	dstType := dst.Type()
	for i := 0; i < dst.NumField(); i++ {
		field := dstType.Field(i)
		fieldVal := dst.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		key, skip := py.fieldKey(field)
		if skip {
			continue
		}
		if field.Anonymous && key == field.Name {
			target := fieldVal
			if target.Kind() == reflect.Ptr {
				if target.IsNil() {
					target.Set(reflect.New(target.Type().Elem()))
				}
				target = target.Elem()
			}
			if target.Kind() == reflect.Struct && target.Type() != timeType {
				if err := py.setStructValue(path+"."+field.Name, target, src); err != nil {
					return err
				}
				continue
			}
		}
		value, ok := py.lookupMapValue(src, key)
		if !ok {
			continue
		}
		if err := py.setValue(path+"."+field.Name, fieldVal, value); err != nil {
			return err
		}
	}
	return nil
}

func (py *ParseYaml) setValue(path string, dst reflect.Value, src any) error {
	if !dst.CanSet() {
		return fmt.Errorf("%s can not be set", path)
	}
	if src == nil {
		dst.SetZero()
		return nil
	}
	if dst.Kind() == reflect.Ptr {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return py.setValue(path, dst.Elem(), src)
	}
	if dst.Type() == timeType {
		t, ok := src.(time.Time)
		if !ok {
			return fmt.Errorf("%s want time.Time, got %T", path, src)
		}
		dst.Set(reflect.ValueOf(t))
		return nil
	}
	if dst.Type() == durationType {
		d, ok := src.(time.Duration)
		if !ok {
			return fmt.Errorf("%s want time.Duration, got %T", path, src)
		}
		dst.Set(reflect.ValueOf(d))
		return nil
	}
	srcVal := reflect.ValueOf(src)
	if srcVal.IsValid() && srcVal.Type().AssignableTo(dst.Type()) {
		dst.Set(srcVal)
		return nil
	}

	switch dst.Kind() {
	case reflect.Interface:
		if !srcVal.Type().AssignableTo(dst.Type()) {
			return fmt.Errorf("%s want %s, got %T", path, dst.Type(), src)
		}
		dst.Set(srcVal)
		return nil
	case reflect.Struct:
		m, ok := src.(map[string]any)
		if !ok {
			return fmt.Errorf("%s want struct map, got %T", path, src)
		}
		return py.setStructValue(path, dst, m)
	case reflect.Map:
		return py.setMapValue(path, dst, src)
	case reflect.Slice:
		return py.setSliceValue(path, dst, src)
	case reflect.Array:
		return py.setArrayValue(path, dst, src)
	case reflect.String:
		s, ok := src.(string)
		if !ok {
			return fmt.Errorf("%s want string, got %T", path, src)
		}
		dst.SetString(s)
		return nil
	case reflect.Bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("%s want bool, got %T", path, src)
		}
		dst.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return py.setIntValue(path, dst, src)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return py.setUintValue(path, dst, src)
	case reflect.Float32, reflect.Float64:
		return py.setFloatValue(path, dst, src)
	}
	return fmt.Errorf("%s unsupported field type %s", path, dst.Type())
}

func (py *ParseYaml) setMapValue(path string, dst reflect.Value, src any) error {
	srcMap, ok := src.(map[string]any)
	if !ok {
		return fmt.Errorf("%s want map, got %T", path, src)
	}
	if dst.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("%s map key must be string, got %s", path, dst.Type().Key())
	}
	if dst.IsNil() {
		dst.Set(reflect.MakeMapWithSize(dst.Type(), len(srcMap)))
	}
	for key, value := range srcMap {
		elem := reflect.New(dst.Type().Elem()).Elem()
		if err := py.setValue(path+"."+key, elem, value); err != nil {
			return err
		}
		dst.SetMapIndex(reflect.ValueOf(key).Convert(dst.Type().Key()), elem)
	}
	return nil
}

func (py *ParseYaml) setSliceValue(path string, dst reflect.Value, src any) error {
	srcSlice, ok := src.([]any)
	if !ok {
		return fmt.Errorf("%s want slice, got %T", path, src)
	}
	res := reflect.MakeSlice(dst.Type(), len(srcSlice), len(srcSlice))
	for i, value := range srcSlice {
		if err := py.setValue(fmt.Sprintf("%s[%d]", path, i), res.Index(i), value); err != nil {
			return err
		}
	}
	dst.Set(res)
	return nil
}

func (py *ParseYaml) setArrayValue(path string, dst reflect.Value, src any) error {
	srcSlice, ok := src.([]any)
	if !ok {
		return fmt.Errorf("%s want array, got %T", path, src)
	}
	if len(srcSlice) > dst.Len() {
		return fmt.Errorf("%s array len %d is smaller than input len %d", path, dst.Len(), len(srcSlice))
	}
	for i, value := range srcSlice {
		if err := py.setValue(fmt.Sprintf("%s[%d]", path, i), dst.Index(i), value); err != nil {
			return err
		}
	}
	return nil
}

func (py *ParseYaml) setIntValue(path string, dst reflect.Value, src any) error {
	srcVal := reflect.ValueOf(src)
	if !srcVal.IsValid() {
		dst.SetZero()
		return nil
	}
	if srcVal.Type() == durationType {
		return fmt.Errorf("%s want %s, got time.Duration", path, dst.Type())
	}
	switch srcVal.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := srcVal.Int()
		if dst.OverflowInt(n) {
			return fmt.Errorf("%s value %d overflows %s", path, n, dst.Type())
		}
		dst.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n := srcVal.Uint()
		if n > uint64(^uint(0)>>1) || dst.OverflowInt(int64(n)) {
			return fmt.Errorf("%s value %d overflows %s", path, n, dst.Type())
		}
		dst.SetInt(int64(n))
		return nil
	}
	return fmt.Errorf("%s want integer, got %T", path, src)
}

func (py *ParseYaml) setUintValue(path string, dst reflect.Value, src any) error {
	srcVal := reflect.ValueOf(src)
	if !srcVal.IsValid() {
		dst.SetZero()
		return nil
	}
	if srcVal.Type() == durationType {
		return fmt.Errorf("%s want %s, got time.Duration", path, dst.Type())
	}
	switch srcVal.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := srcVal.Int()
		if n < 0 || dst.OverflowUint(uint64(n)) {
			return fmt.Errorf("%s value %d overflows %s", path, n, dst.Type())
		}
		dst.SetUint(uint64(n))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n := srcVal.Uint()
		if dst.OverflowUint(n) {
			return fmt.Errorf("%s value %d overflows %s", path, n, dst.Type())
		}
		dst.SetUint(n)
		return nil
	}
	return fmt.Errorf("%s want unsigned integer, got %T", path, src)
}

func (py *ParseYaml) setFloatValue(path string, dst reflect.Value, src any) error {
	srcVal := reflect.ValueOf(src)
	if !srcVal.IsValid() {
		dst.SetZero()
		return nil
	}
	switch srcVal.Kind() {
	case reflect.Float32, reflect.Float64:
		f := srcVal.Float()
		if dst.OverflowFloat(f) {
			return fmt.Errorf("%s value %f overflows %s", path, f, dst.Type())
		}
		dst.SetFloat(f)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		dst.SetFloat(float64(srcVal.Int()))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		dst.SetFloat(float64(srcVal.Uint()))
		return nil
	}
	return fmt.Errorf("%s want float, got %T", path, src)
}

func Unmarshal(ctx context.Context, content []byte, data any) error {
	py := NewParseYamlBytes(content)
	m, err := py.parseToMap(ctx)
	if err != nil {
		return err
	}
	return py.decode(ctx, data, m)
}

func UnmarshalFile(ctx context.Context, file string, data any) error {
	py := NewParseYaml(file)
	if py == nil {
		return fmt.Errorf("read yaml file %s failed", file)
	}
	m, err := py.parseToMap(ctx)
	if err != nil {
		return err
	}
	return py.decode(ctx, data, m)
}
