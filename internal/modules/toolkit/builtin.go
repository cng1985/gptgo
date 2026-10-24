package toolkit

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	_ "time/tzdata"
)

type builtinTool struct {
	Name        string
	Description string
	Parameters  string
}

func builtinCatalog() []builtinTool {
	return []builtinTool{
		{
			Name:        "current_time",
			Description: "读取当前日期和时间。需要现在几点、今天几号或指定时区时使用。",
			Parameters:  `{"type":"object","properties":{"timezone":{"type":"string","description":"IANA 时区，例如 Asia/Shanghai。留空使用 Asia/Shanghai"}},"additionalProperties":false}`,
		},
		{
			Name:        "calculator",
			Description: "计算只含数字、小数、括号和加减乘除的算式。",
			Parameters:  `{"type":"object","properties":{"expression":{"type":"string","description":"算式，例如 (12+3)*4"}},"required":["expression"],"additionalProperties":false}`,
		},
	}
}

func runCurrentTime(arguments string) (bool, string) {
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := decodeObject(arguments, &body); err != nil {
		return false, "参数需要是 JSON 对象"
	}
	zone := strings.TrimSpace(body.Timezone)
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	if len(zone) > 80 {
		return false, "时区无效"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false, "时区无效"
	}
	now := time.Now().In(loc)
	weeks := []string{"日", "一", "二", "三", "四", "五", "六"}
	return true, now.Format("2006-01-02 15:04:05") + " " + zone + " 星期" + weeks[now.Weekday()]
}

func runCalculator(arguments string) (bool, string) {
	var body struct {
		Expression string `json:"expression"`
	}
	if err := decodeObject(arguments, &body); err != nil {
		return false, "参数需要是 JSON 对象"
	}
	value, err := evalExpr(body.Expression)
	if err != nil {
		return false, err.Error()
	}
	return true, formatNumber(value)
}

func decodeObject(raw string, dest any) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !json.Valid([]byte(raw)) || raw[0] != '{' {
		return fmt.Errorf("参数需要是 JSON 对象")
	}
	return json.Unmarshal([]byte(raw), dest)
}

func evalExpr(input string) (float64, error) {
	expr := strings.TrimSpace(input)
	if expr == "" {
		return 0, fmt.Errorf("算式不能为空")
	}
	if len(expr) > 120 {
		return 0, fmt.Errorf("算式过长")
	}
	for _, r := range expr {
		if unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '+', '-', '*', '/', '(', ')', '.':
		default:
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("算式只支持数字和加减乘除")
			}
		}
	}
	p := &parser{s: expr}
	value, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skip()
	if p.i != len(p.s) {
		return 0, fmt.Errorf("算式没有识别完")
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("算式结果无效")
	}
	return value, nil
}

func formatNumber(value float64) string {
	rounded := math.Round(value)
	if math.Abs(value-rounded) < 1e-9 && math.Abs(rounded) < 1e15 {
		return strconv.FormatInt(int64(rounded), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

type parser struct {
	s string
	i int
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skip()
		if p.eat('+') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
			continue
		}
		if p.eat('-') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
			continue
		}
		return left, nil
	}
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skip()
		if p.eat('*') {
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			left *= right
			continue
		}
		if p.eat('/') {
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("不能除以 0")
			}
			left /= right
			continue
		}
		return left, nil
	}
}

func (p *parser) parseFactor() (float64, error) {
	p.skip()
	if p.eat('+') {
		return p.parseFactor()
	}
	if p.eat('-') {
		value, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -value, nil
	}
	if p.eat('(') {
		value, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skip()
		if !p.eat(')') {
			return 0, fmt.Errorf("括号没有闭合")
		}
		return value, nil
	}
	start := p.i
	dot := false
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c >= '0' && c <= '9' {
			p.i++
			continue
		}
		if c == '.' && !dot {
			dot = true
			p.i++
			continue
		}
		break
	}
	if start == p.i {
		return 0, fmt.Errorf("算式不完整")
	}
	return strconv.ParseFloat(p.s[start:p.i], 64)
}

func (p *parser) skip() {
	for p.i < len(p.s) && unicode.IsSpace(rune(p.s[p.i])) {
		p.i++
	}
}

func (p *parser) eat(c byte) bool {
	p.skip()
	if p.i < len(p.s) && p.s[p.i] == c {
		p.i++
		return true
	}
	return false
}
