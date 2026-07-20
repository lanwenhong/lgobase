package gconfig_v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type TokenType int

const (
	TokenEOF      TokenType = iota // 0
	TokenLBrace                    // 1 {
	TokenRBrace                    // 2 }
	TokenLBracket                  // 3 [
	TokenRBracket                  // 4 ]
	TokenColon                     // 5 :
	TokenComma                     // 6 ,
	TokenString                    // 7 字符串/标量
)

type TokenNode struct {
	line     string
	value    string
	key      string
	iv       any
	nodeType int
	indent   int
	Obj      any
	rawStr   bool
}

type Token struct {
	Type   TokenType
	Value  string
	Quoted bool
}

type Lexer struct {
	input string
	pos   int
	end   int
	err   error
}

type Parser struct {
	lex *Lexer
	tok Token
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		pos:   0,
		end:   len(input),
	}
}

func (l *Lexer) current() byte {
	if l.pos >= l.end {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() {
	if l.pos < l.end {
		l.pos++
	}
}

func (l *Lexer) skipWhitespace() {
	for l.pos < l.end && l.current() == ' ' {
		l.advance()
	}
}

func (l *Lexer) readString(stopAtColon bool) string {
	start := l.pos
	for l.pos < l.end {
		c := l.current()
		switch c {
		case '{', '}', '[', ']', ',', ' ':
			return l.input[start:l.pos]
		case ':':
			if stopAtColon {
				return l.input[start:l.pos]
			}
		}
		l.advance()
	}
	return l.input[start:l.pos]
}

func (l *Lexer) readSingleQuote() string {
	l.advance()
	var buf strings.Builder
	for l.pos < l.end {
		c := l.current()
		if c == '\'' {
			l.advance()
			if l.current() == '\'' {
				buf.WriteByte('\'')
				l.advance()
				continue
			}
			return buf.String()
		}
		buf.WriteByte(c)
		l.advance()
	}
	l.err = fmt.Errorf("字符串缺少闭合 '")
	return buf.String()
}

func (l *Lexer) readDoubleQuote() string {
	l.advance()
	var buf strings.Builder
	for l.pos < l.end {
		c := l.current()
		if c == '\\' {
			l.advance()
			if l.pos >= l.end {
				l.err = fmt.Errorf("字符串转义缺少字符")
				return buf.String()
			}
			buf.WriteByte(l.current())
			l.advance()
			continue
		}
		if c == '"' {
			l.advance()
			return buf.String()
		}
		buf.WriteByte(c)
		l.advance()
	}
	l.err = fmt.Errorf("字符串缺少闭合 \"")
	return buf.String()
}

func (l *Lexer) nextToken(stopAtColon bool) Token {
	l.skipWhitespace()
	if l.pos >= l.end {
		return Token{Type: TokenEOF}
	}

	c := l.current()
	switch c {
	case '{':
		l.advance()
		return Token{Type: TokenLBrace, Value: "{"}
	case '}':
		l.advance()
		return Token{Type: TokenRBrace, Value: "}"}
	case '[':
		l.advance()
		return Token{Type: TokenLBracket, Value: "["}
	case ']':
		l.advance()
		return Token{Type: TokenRBracket, Value: "]"}
	case ':':
		l.advance()
		return Token{Type: TokenColon, Value: ":"}
	case ',':
		l.advance()
		return Token{Type: TokenComma, Value: ","}
	case '\'':
		s := l.readSingleQuote()
		return Token{Type: TokenString, Value: s, Quoted: true}
	case '"':
		s := l.readDoubleQuote()
		return Token{Type: TokenString, Value: s, Quoted: true}
	default:
		s := l.readString(stopAtColon)
		return Token{Type: TokenString, Value: s}
	}
}

func (l *Lexer) NextToken() Token {
	return l.nextToken(true)
}

func (l *Lexer) NextKeyToken() Token {
	return l.nextToken(true)
}

func (l *Lexer) NextValueToken() Token {
	return l.nextToken(false)
}

func hasCRLF(s string) bool {
	return strings.Contains(s, "\r") || strings.Contains(s, "\n")
}

func unsupportedPlainScalar(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "!!str") {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "&"):
		return "anchor", true
	case strings.HasPrefix(s, "*"):
		return "alias", true
	case strings.HasPrefix(s, "!"):
		return "tag", true
	}
	return "", false
}

func errUnsupportedYAMLFeature(feature string) error {
	return fmt.Errorf("unsupported yaml feature: %s", feature)
}

func NewTokenNode(line, k, v string, indent int) *TokenNode {
	tk := &TokenNode{
		line:   line,
		key:    k,
		value:  v,
		indent: indent,
	}
	return tk
}

func (token *TokenNode) toAny() any {
	if token.value == "" && token.Obj == nil && token.key != "" {
		return token.key
	}
	return token.Obj
}

func (tkn *TokenNode) NodeType2String(ctx context.Context) string {
	switch tkn.nodeType {
	//switch *asn.nt {
	case NODE_TYPE_MAP:
		return "MAP"
	case NODE_TYPE_SLICE:
		return "SLICE"
	case NODE_TYPE_STRING:
		return "STRING"
	case NODE_TYPE_FLOAT:
		return "FLOAT"
	case NODE_TYPE_INT:
		return "INT"
	case NODE_TYPE_TIMESTAMP:
		return "TIMESTAMP"
	case NODE_TYPE_BOOL:
		return "BOOL"
	case NODE_TYPE_NULL:
		return "NULL"
	case NODE_TYPE_DURATION:
		return "DURATION"
	}
	return ""
}

func (tkn *TokenNode) IsInteger(s string) bool {
	if strings.Contains(s, "\r") || strings.Contains(s, "\n") {
		return false
	}

	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return false
	}

	idx := 0
	if s[0] == '+' || s[0] == '-' {
		idx++
		if idx >= n {
			return false
		}
	}

	for ; idx < n; idx++ {
		c := s[idx]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (tkn *TokenNode) IsFloat(s string) bool {

	if strings.Contains(s, "\r") || strings.Contains(s, "\n") {
		return false
	}

	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		return false
	}

	var (
		idx    = 0
		hasDot = false
		hasExp = false
	)

	// 处理开头正负号
	if s[0] == '+' || s[0] == '-' {
		idx++
		if idx >= n {
			return false
		}
	}

	for ; idx < n; idx++ {
		c := s[idx]
		switch c {
		case '.':
			if hasDot || hasExp {
				return false
			}
			hasDot = true

		case 'e', 'E':
			if hasExp {
				return false
			}
			hasExp = true
			idx++
			if idx >= n {
				return false
			}
			if s[idx] == '+' || s[idx] == '-' {
				idx++
				if idx >= n {
					return false
				}
			}

		default:
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	// 必须包含小数点 或 指数符，才判定为浮点数
	return hasDot || hasExp
}

func (tkn *TokenNode) IsBool(s string) bool {
	if hasCRLF(s) {
		return false
	}
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	switch lower {
	case "true", "yes", "on",
		"false", "no", "off":
		return true
	}
	return false
}

func (tkn *TokenNode) IsNull(s string) bool {
	if hasCRLF(s) {
		return false
	}
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	return lower == "null" || s == "~"
}

func (tkn *TokenNode) IsTimestamp(s string) (bool, time.Time) {
	var t time.Time
	var err error = nil
	if hasCRLF(s) {
		return false, t
	}
	s = strings.TrimSpace(s)
	// 优先尝试 RFC3339(ISO8601) YAML 主流时间格式
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return true, t
	}
	// 兼容 空格分隔 日期时间: 2026-06-11 12:00:00
	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return true, t
	}
	// 仅日期
	t, err = time.Parse("2006-01-02", s)
	return err == nil, t
}

func (tkn *TokenNode) IsDuration(s string) (bool, time.Duration) {
	if hasCRLF(s) {
		return false, 0
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return false, 0
	}
	d, err := time.ParseDuration(s)
	return err == nil, d
}

func (tkn *TokenNode) parseExplicitStringTag(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s != "!!str" && !strings.HasPrefix(s, "!!str ") {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(s, "!!str"))
	if value == "" {
		return "", true
	}
	if strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") {
		lex := NewLexer(value)
		tok := lex.NextToken()
		if tok.Type == TokenString {
			return tok.Value, true
		}
	}
	return value, true
}

func (tkn *TokenNode) parseScalar(s string) (any, error) {
	s = strings.TrimSpace(s)
	if value, ok := tkn.parseExplicitStringTag(s); ok {
		tkn.nodeType = NODE_TYPE_STRING
		return value, nil
	}
	if tkn.IsNull(s) {
		tkn.nodeType = NODE_TYPE_NULL
		return nil, nil
	}
	if tkn.IsBool(s) {
		lower := strings.ToLower(s)
		tkn.nodeType = NODE_TYPE_BOOL
		switch lower {
		case "true", "yes", "on":
			return true, nil
		case "false", "no", "off":
			return false, nil
		}
	}
	if tkn.IsInteger(s) {
		res, err := strconv.ParseInt(s, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", s, err)
		}
		tkn.nodeType = NODE_TYPE_INT
		return int(res), nil
	}
	if tkn.IsFloat(s) {
		res, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", s, err)
		}
		tkn.nodeType = NODE_TYPE_FLOAT
		return res, nil
	}
	if isTime, t := tkn.IsTimestamp(s); isTime {
		tkn.nodeType = NODE_TYPE_TIMESTAMP
		return t, nil
	}
	if isDuration, d := tkn.IsDuration(s); isDuration {
		tkn.nodeType = NODE_TYPE_DURATION
		return d, nil
	}
	tkn.nodeType = NODE_TYPE_STRING
	return s, nil
}

func NewParser(lex *Lexer) *Parser {
	p := &Parser{lex: lex}
	p.nextValue()
	return p
}

func (p *Parser) nextKey() {
	p.tok = p.lex.NextKeyToken()
}

func (p *Parser) nextValue() {
	p.tok = p.lex.NextValueToken()
}

func (p *Parser) parseValue(ctx context.Context, tkn *TokenNode) (any, error) {
	switch p.tok.Type {
	case TokenLBrace:
		p.nextKey()
		m := make(map[string]any)
		tkn.nodeType = NODE_TYPE_MAP
		for p.tok.Type != TokenRBrace && p.tok.Type != TokenEOF {
			if p.tok.Type != TokenString {
				return nil, fmt.Errorf("需要字符串类型 key")
			}
			key := p.tok.Value
			if _, ok := m[key]; ok {
				return nil, fmt.Errorf("duplicate key %s", key)
			}
			p.nextValue()

			if p.tok.Type != TokenColon {
				return nil, fmt.Errorf("key 后必须有冒号")
			}
			p.nextValue()

			val, err := p.parseValue(ctx, tkn)
			if err != nil {
				return nil, err
			}
			m[key] = val

			if p.tok.Type == TokenComma {
				p.nextKey()
			} else if p.tok.Type == TokenEOF {
				continue
			} else if p.tok.Type != TokenRBrace {
				return nil, fmt.Errorf("映射元素后必须是逗号或 }")
			}
		}
		if p.tok.Type != TokenRBrace {
			return nil, fmt.Errorf("映射缺少闭合 }")
		}
		p.nextValue()
		return m, nil

	case TokenLBracket:
		p.nextValue()
		arr := make([]any, 0)
		tkn.nodeType = NODE_TYPE_SLICE
		for p.tok.Type != TokenRBracket && p.tok.Type != TokenEOF {
			elem, err := p.parseValue(ctx, tkn)
			if err != nil {
				return nil, err
			}
			arr = append(arr, elem)

			if p.tok.Type == TokenComma {
				p.nextValue()
			} else if p.tok.Type == TokenEOF {
				continue
			} else if p.tok.Type != TokenRBracket {
				return nil, fmt.Errorf("数组元素后必须是逗号或 ]")
			}
		}
		if p.tok.Type != TokenRBracket {
			return nil, fmt.Errorf("数组缺少闭合 ]")
		}
		p.nextValue()
		return arr, nil
	case TokenString:
		if p.tok.Quoted {
			val := p.tok.Value
			tkn.nodeType = NODE_TYPE_STRING
			p.nextValue()
			return val, nil
		}
		if p.tok.Value == "!!str" {
			p.nextValue()
			if p.tok.Type == TokenEOF {
				tkn.nodeType = NODE_TYPE_STRING
				return "", nil
			}
			if p.tok.Type != TokenString {
				return nil, fmt.Errorf("!!str 后必须是字符串值")
			}
			val := p.tok.Value
			tkn.nodeType = NODE_TYPE_STRING
			p.nextValue()
			return val, nil
		}
		if !p.tok.Quoted {
			if feature, ok := unsupportedPlainScalar(p.tok.Value); ok {
				return nil, errUnsupportedYAMLFeature(feature)
			}
		}
		val, err := tkn.parseScalar(p.tok.Value)
		if err != nil {
			return nil, err
		}
		p.nextValue()
		return val, nil

	default:
		return nil, fmt.Errorf("非法 Token: %d", p.tok.Type)
	}
}

func (tkn *TokenNode) TokenParse(ctx context.Context) error {
	if tkn.rawStr {
		tkn.nodeType = NODE_TYPE_STRING
		tkn.Obj = tkn.value
		return nil
	}

	value := strings.TrimSpace(tkn.value)
	if value == "" {
		return nil
	}
	if feature, ok := unsupportedPlainScalar(value); ok {
		return errUnsupportedYAMLFeature(feature)
	}
	if !strings.HasPrefix(value, "{") &&
		!strings.HasPrefix(value, "[") &&
		!strings.HasPrefix(value, "'") &&
		!strings.HasPrefix(value, "\"") {
		obj, err := tkn.parseScalar(value)
		if err != nil {
			return err
		}
		tkn.Obj = obj
		return nil
	}

	lex := NewLexer(value)
	parser := NewParser(lex)
	obj, err := parser.parseValue(ctx, tkn)
	if err != nil {
		return err
	}
	if parser.lex.err != nil {
		return parser.lex.err
	}
	if parser.tok.Type != TokenEOF {
		return fmt.Errorf("unexpected token after value: %s", parser.tok.Value)
	}
	switch obj.(type) {
	case map[string]any:
		tkn.nodeType = NODE_TYPE_MAP
	case []any:
		tkn.nodeType = NODE_TYPE_SLICE
	}
	tkn.Obj = obj
	return nil
}
