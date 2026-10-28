package guides

import (
	"errors"
	"strconv"
	"strings"
	"text/scanner"
)

// OP.GG may encode tool results as class schemas followed by constructors.
// This parser accepts literals and declared positional constructors only. It
// never evaluates code, resolves attributes, or executes provider content.
func decodeCompact(input string) (any, error) {
	if len(input) > 2<<20 {
		return nil, errors.New("攻略响应过大")
	}
	schemas := map[string][]string{}
	lines := strings.Split(input, "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(l, "class ") {
			parts := strings.SplitN(strings.TrimPrefix(l, "class "), ":", 2)
			if len(parts) != 2 {
				return nil, errors.New("攻略结构不可用")
			}
			fields := strings.Split(strings.TrimSpace(parts[1]), ",")
			schemas[strings.TrimSpace(parts[0])] = fields
			start = i + 1
		}
	}
	p := literalParser{schemas: schemas}
	p.s.Init(strings.NewReader(strings.Join(lines[start:], "\n")))
	p.s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanInts | scanner.ScanFloats
	p.s.Error = func(*scanner.Scanner, string) { p.failed = true }
	p.next()
	value := p.value(0)
	if p.failed || p.t != scanner.EOF {
		return nil, errors.New("攻略结构不可用")
	}
	return value, nil
}

type literalParser struct {
	s       scanner.Scanner
	t       rune
	schemas map[string][]string
	failed  bool
	nodes   int
}

func (p *literalParser) next() { p.t = p.s.Scan() }
func (p *literalParser) expect(t rune) {
	if p.t != t {
		p.failed = true
		return
	}
	p.next()
}
func (p *literalParser) value(depth int) any {
	p.nodes++
	if depth > 64 || p.nodes > 100000 {
		p.failed = true
		return nil
	}
	switch p.t {
	case scanner.String:
		v, e := strconv.Unquote(p.s.TokenText())
		if e != nil {
			p.failed = true
		}
		p.next()
		return v
	case scanner.Int, scanner.Float:
		v, e := strconv.ParseFloat(p.s.TokenText(), 64)
		if e != nil {
			p.failed = true
		}
		p.next()
		return v
	case '-':
		p.next()
		v, ok := p.value(depth + 1).(float64)
		if !ok {
			p.failed = true
		}
		return -v
	case '[':
		p.next()
		a := []any{}
		for p.t != ']' && !p.failed {
			a = append(a, p.value(depth+1))
			if p.t != ',' {
				break
			}
			p.next()
		}
		p.expect(']')
		return a
	case scanner.Ident:
		name := p.s.TokenText()
		p.next()
		switch name {
		case "None", "null":
			return nil
		case "True", "true":
			return true
		case "False", "false":
			return false
		}
		fields, ok := p.schemas[name]
		if !ok {
			p.failed = true
			return nil
		}
		p.expect('(')
		m := map[string]any{}
		for i, f := range fields {
			if p.failed {
				break
			}
			if i > 0 {
				p.expect(',')
			}
			m[strings.TrimSpace(f)] = p.value(depth + 1)
		}
		p.expect(')')
		return m
	default:
		p.failed = true
		return nil
	}
}
