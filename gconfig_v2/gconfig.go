package gconfig_v2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

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
	reMap    *regexp.Regexp
	reSlice  *regexp.Regexp
}

func (asn *AstNode) NodeType2String(ctx context.Context) string {
	//fmt.Println("------", *asn.nt)
	switch asn.nodeType {
	//switch *asn.nt {
	case NODE_TYPE_MAP:
		return "MAP"
	case NODE_TYPE_SLICE:
		return "SLICE"
	}
	return ""
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
	return &ParseYaml{
		confFile: file,
		content:  content,
		reMap:    regexp.MustCompile(`^(.*):(.*)$`),
		reSlice:  regexp.MustCompile(`^(- )([^:\s]+)(:?)(.*)$`),
	}
}

func (py *ParseYaml) errLine(line string) error {
	errMsg := fmt.Sprintf("line %s format not surport", line)
	err := errors.New(errMsg)
	return err
}

func (py *ParseYaml) isSliceFlag(line string) bool {
	if line[0] == '-' && line[1] == ' ' {
		return true
	}
	return false
}

func (py *ParseYaml) matchMap(ctx context.Context, line string) (string, string, bool) {
	match := py.reMap.FindStringSubmatch(line)
	if len(match) < 3 {
		return "", "", false
	}
	return strings.TrimSpace(match[1]), strings.TrimSpace(match[2]), true
}

func (py *ParseYaml) matchSlice(ctx context.Context, line string) ([]string, bool) {
	groups := py.reSlice.FindStringSubmatch(line)
	if len(groups) < 5 {
		return []string{}, false
	}
	return groups, true
}

func (py *ParseYaml) feedSlice(ctx context.Context, line string, group []string, pNode *AstNode, indent int) error {
	var err error = nil
	k := strings.Trim(group[2], " ")
	v := strings.Trim(group[4], " ")
	//sliceTag := strings.TrimSpace(group[1])

	split := strings.TrimSpace(group[3])

	if k == "" {
		errMsg := fmt.Sprintf("parse line:%s key is empty", line)
		logger.Warn(ctx, "gconfig_v2", "err", err.Error())
		return errors.New(errMsg)
	}

	if pNode.nodeType == NODE_TYPE_UNKNOWN {
		pNode.nodeType = NODE_TYPE_SLICE
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
		token := &TokenNode{
			line:   line,
			value:  v,
			key:    k,
			indent: indent + 2,
			//nodeType: NODE_TYPE_STRING,
		}
		token.TokenParse(ctx)
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
		token := &TokenNode{
			line: line,
			key:  k,
			//nodeType: NODE_TYPE_STRING,
			indent: indent,
		}
		token.TokenParse(ctx)
		pNode.s = append(pNode.s, token)
	} else {
		err = py.errLine(line)
		logger.Warn(ctx, "gconfig_v2", "err", err.Error())
	}
	return err
}

func (py *ParseYaml) feedMap(ctx context.Context, line, k, v string, pNode *AstNode, indent int) error {
	if k == "" {
		errMsg := fmt.Sprintf("parse line:%s key is empty", line)
		return errors.New(errMsg)
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
		py.stack = append(py.stack, node)
		pNode.m[k] = node
	} else if k != "" && v != "" {
		token := &TokenNode{
			line:  line,
			value: v,
			key:   k,
			//nodeType: NODE_TYPE_STRING,
			indent: indent,
		}
		token.TokenParse(ctx)
		//logger.Debug(ctx, "gconfig_v2", "line", line, "pline", pNode.line, "pNode", pNode.nodeType)
		switch pNode.nodeType {
		case NODE_TYPE_UNKNOWN:
			pNode.nodeType = NODE_TYPE_MAP
			logger.Debug(ctx, "gconfig_v2", "line", line, "pline", pNode.line, "pNode", pNode.nodeType)
			//pNode.m[k] = v
			pNode.m[k] = token
		case NODE_TYPE_MAP:
			//pNode.m[k] = v
			pNode.m[k] = token
		case NODE_TYPE_SLICE:
			//pNode.s[-1][k] = v
			slen := len(pNode.s)
			sTmp := pNode.s[slen-1]
			sNode := sTmp.(*AstNode)
			sNode.m[k] = token
			//sTmp.(map[string]interface{})[k] = token
			//pNode.s[slen-1][k] = token
		default:
			err = py.errLine(line)
		}
	} else {
		err = py.errLine(line)
	}
	return err
}

func (py *ParseYaml) parse(ctx context.Context) (map[string]interface{}, error) {
	root := make(map[string]interface{})
	scanner := bufio.NewScanner(strings.NewReader(string(py.content)))
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
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := 0
		for _, c := range line {
			if c == ' ' {
				indent++
			} else {
				break
			}
		}

		//find node
		for len(py.stack) > 0 && py.stack[len(py.stack)-1].indent >= indent {
			py.stack = py.stack[:len(py.stack)-1]
		}
		parentNode := py.stack[len(py.stack)-1]
		if !py.isSliceFlag(trimmed) {
			lk, lv, match := py.matchMap(ctx, line)
			if match {
				logger.Debug(ctx, "gconfig_v2", "line", line, "indent", indent)
				err = py.feedMap(ctx, line, lk, lv, parentNode, indent)
			} else {
				err = py.errLine(line)
				logger.Warn(ctx, "gconfig_v2", "err", err.Error())
			}
		} else {
			group, match := py.matchSlice(ctx, trimmed)
			if match {
				err = py.feedSlice(ctx, line, group, parentNode, indent)
			} else {
				err = py.errLine(line)
				logger.Warn(ctx, "gconfig_v2", "err", err.Error())
			}
		}
	}
	return root, err
}

func PrintAstTree(ctx context.Context, rootNode *AstNode) {
	if rootNode.nodeType == NODE_TYPE_MAP {
		for _, v := range rootNode.m {
			switch v.(type) {
			case *TokenNode:
				token := v.(*TokenNode)
				space := ""
				for i := 0; i < token.indent; i++ {
					space += " "
				}
				t := token.NodeType2String(ctx)
				fmt.Println(fmt.Sprintf("%s%s", space, t))
				fmt.Println(fmt.Sprintf("%s%s: %s", space, token.key, token.value))
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
	} else if rootNode.nodeType == NODE_TYPE_SLICE {
		for _, v := range rootNode.s {
			switch v.(type) {
			case *TokenNode:
				token := v.(*TokenNode)
				space := ""
				for i := 0; i < token.indent; i++ {
					space += " "
				}
				space += token.key
				if token.value != "" {
					space += ": "
					space += token.value
				}
				t := token.NodeType2String(ctx)
				//logger.Debug(ctx, "gconfig_v2", "nodeType", t)
				fmt.Println(fmt.Sprintf("%s%s", space, t))
				fmt.Println(fmt.Sprintf("%s", space))
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

func Unmarshal(ctx context.Context, file string, data *map[string]any) error {
	py := NewParseYaml(file)
	root, _ := py.parse(ctx)
	rootNode := root["root"]
	PrintAstTree(ctx, rootNode.(*AstNode))
	return nil
}
