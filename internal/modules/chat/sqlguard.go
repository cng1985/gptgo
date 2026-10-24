package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	identRe       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	fromRe        = regexp.MustCompile(`(?i)\b(?:from|join)\s+"?([A-Za-z_][A-Za-z0-9_]*)`)
	limitRe       = regexp.MustCompile(`(?i)\blimit\s+(\d+)`)
	selectStartRe = regexp.MustCompile(`(?i)^\s*(select|with)\b`)
	headLimitRe   = regexp.MustCompile(`前\s*(\d{1,3})`)
	writeSQLRe    = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^delete\s+from\b`),
		regexp.MustCompile(`(?i)^update\s+[A-Za-z_][A-Za-z0-9_]*\s+set\b`),
		regexp.MustCompile(`(?i)^insert\s+into\b`),
		regexp.MustCompile(`(?i)^drop\s+(table|view|index)\b`),
		regexp.MustCompile(`(?i)^alter\s+table\b`),
		regexp.MustCompile(`(?i)^pragma\s+`),
		regexp.MustCompile(`(?i)^attach\s+`),
	}
	forbiddenSQL = []string{
		"insert", "update", "delete", "drop", "alter", "attach", "detach",
		"pragma", "vacuum", "reindex", "truncate", "grant", "revoke", "into",
	}
)

func quoteIdent(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", fmt.Errorf("非法标识符")
	}
	return `"` + name + `"`, nil
}

func extractSQL(question string) (string, bool) {
	text := strings.TrimSpace(question)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	if idx := strings.Index(lower, "```sql"); idx >= 0 {
		rest := strings.TrimLeft(text[idx+6:], " \t\r\n")
		end := strings.Index(rest, "```")
		if end >= 0 {
			body := strings.TrimSpace(rest[:end])
			if body != "" {
				return body, true
			}
		}
	}
	if looksLikeSQL(text) {
		return text, true
	}
	return "", false
}

func looksLikeSQL(text string) bool {
	head := strings.TrimSpace(text)
	lower := strings.ToLower(head)
	if strings.HasPrefix(lower, "select ") || strings.HasPrefix(lower, "select\n") || strings.HasPrefix(lower, "select\t") {
		return true
	}
	if strings.HasPrefix(lower, "with ") && strings.Contains(lower, "select") {
		return true
	}
	for _, re := range writeSQLRe {
		if re.MatchString(head) {
			return true
		}
	}
	return false
}

func guardSelect(sql string, catalog []DataTable) (string, error) {
	raw := strings.TrimSpace(sql)
	raw = strings.TrimRightFunc(raw, unicode.IsSpace)
	raw = strings.TrimSuffix(raw, ";")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("问数只能执行一条只读 SELECT，不能修改数据")
	}
	stripped := stripSQLStrings(raw)
	if strings.Contains(stripped, ";") {
		return "", fmt.Errorf("问数只能执行一条只读 SELECT，不能修改数据")
	}
	if strings.Contains(stripped, "--") || strings.Contains(stripped, "/*") {
		return "", fmt.Errorf("问数只能执行一条只读 SELECT，不能修改数据")
	}
	if !selectStartRe.MatchString(stripped) {
		return "", fmt.Errorf("问数只能执行一条只读 SELECT，不能修改数据")
	}
	lower := strings.ToLower(stripped)
	for _, word := range forbiddenSQL {
		if hasWord(lower, word) {
			return "", fmt.Errorf("问数只能执行一条只读 SELECT，不能修改数据")
		}
	}
	known := map[string]bool{}
	for _, table := range catalog {
		known[strings.ToLower(table.Name)] = true
	}
	for _, ref := range fromRe.FindAllStringSubmatch(stripped, -1) {
		if len(ref) < 2 {
			continue
		}
		if !known[strings.ToLower(ref[1])] {
			return "", fmt.Errorf("数据表 %s 不存在，已改为读取当前库里的全部数据表", ref[1])
		}
	}
	if m := limitRe.FindStringSubmatch(lower); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		if n > 200 {
			raw = limitRe.ReplaceAllString(raw, "LIMIT 200")
		}
	} else {
		raw += " LIMIT 200"
	}
	return raw, nil
}

func stripSQLStrings(sql string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(sql); i++ {
		if sql[i] == '\'' {
			if in && i+1 < len(sql) && sql[i+1] == '\'' {
				i++
				continue
			}
			in = !in
			b.WriteByte(' ')
			continue
		}
		if in {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}

func hasWord(lower, word string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`).MatchString(lower)
}

func rowLimit(question string, fallback int) int {
	match := headLimitRe.FindStringSubmatch(question)
	if len(match) != 2 {
		return fallback
	}
	n, err := strconv.Atoi(match[1])
	if err != nil || n < 1 {
		return fallback
	}
	if n > 200 {
		return 200
	}
	return n
}
