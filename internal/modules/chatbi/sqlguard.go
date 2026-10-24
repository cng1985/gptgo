package chatbi

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
)

var (
	identPattern = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_$]{0,62}$`)
	forbiddenSQL = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|truncate|grant|revoke|exec|execute|merge|call|attach|detach|pragma|outfile|dumpfile|load_file|into|shutdown|backup|restore|rename|handler|openrowset|openquery|sp_configure|xp_|sleep|benchmark|pg_sleep|waitfor|dbcc)\b`)
	forUpdateSQL = regexp.MustCompile(`(?i)\bfor\s+update\b`)
	limitSQL     = regexp.MustCompile(`(?i)\blimit\s+\d+`)
	topSQL       = regexp.MustCompile(`(?i)\btop\s*(\(|\d)`)
	selectHead   = regexp.MustCompile(`(?i)^(\s*select\s+(?:distinct\s+)?)`)
)

func CanonicalDriver(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql", "pgsql":
		return "postgres", nil
	case "sqlserver", "mssql":
		return "sqlserver", nil
	default:
		return "", apperr.Invalid("数据源只支持 mysql、pgsql、sqlserver")
	}
}

func QuoteIdent(driver, name string) (string, error) {
	if !identPattern.MatchString(name) {
		return "", fmt.Errorf("标识符无效")
	}
	switch driver {
	case "mysql":
		return "`" + name + "`", nil
	case "postgres":
		return `"` + name + `"`, nil
	case "sqlserver":
		return "[" + name + "]", nil
	default:
		return "", fmt.Errorf("不支持的数据源")
	}
}

func Guard(sql, driver string, maxRows int) (string, error) {
	cleaned, err := prepareReadOnly(sql)
	if err != nil {
		return "", err
	}
	if maxRows < 1 {
		maxRows = MaxRows
	}
	if maxRows > 1000 {
		maxRows = 1000
	}
	return applyLimit(cleaned, driver, maxRows), nil
}

func prepareReadOnly(sql string) (string, error) {
	raw := strings.TrimSpace(sql)
	raw = strings.TrimRight(raw, "; \t\r\n")
	if raw == "" {
		return "", apperr.Invalid("只允许执行单条只读查询")
	}
	if strings.Contains(raw, ";") {
		return "", apperr.Invalid("只允许执行单条只读查询")
	}
	if utf8.RuneCountInString(raw) > 4000 {
		return "", apperr.Invalid("查询过长")
	}
	cleaned := strings.TrimSpace(stripComments(raw))
	if cleaned == "" {
		return "", apperr.Invalid("只允许执行单条只读查询")
	}
	lower := strings.ToLower(cleaned)
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return "", apperr.Invalid("只允许执行单条只读查询")
	}
	if forbiddenSQL.MatchString(cleaned) || forUpdateSQL.MatchString(cleaned) {
		return "", apperr.Invalid("只允许执行单条只读查询")
	}
	return cleaned, nil
}

func applyLimit(sql, driver string, maxRows int) string {
	switch driver {
	case "sqlserver":
		if topSQL.MatchString(sql) || strings.HasPrefix(strings.ToLower(strings.TrimSpace(sql)), "with") {
			return sql
		}
		if selectHead.MatchString(sql) {
			return selectHead.ReplaceAllString(sql, fmt.Sprintf("${1}TOP (%d) ", maxRows))
		}
		return sql
	default:
		if limitSQL.MatchString(sql) {
			return sql
		}
		return sql + fmt.Sprintf(" LIMIT %d", maxRows)
	}
}

func stripComments(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))
	inLine := false
	inBlock := false
	for i := 0; i < len(sql); i++ {
		if inLine {
			if sql[i] == '\n' {
				inLine = false
				b.WriteByte('\n')
			}
			continue
		}
		if inBlock {
			if sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				inBlock = false
				i++
				b.WriteByte(' ')
			}
			continue
		}
		if sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		if sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}

func extractSQL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if start := strings.Index(text, "```"); start >= 0 {
		rest := text[start+3:]
		rest = strings.TrimPrefix(rest, "sql")
		rest = strings.TrimPrefix(rest, "SQL")
		rest = strings.TrimPrefix(rest, "\n")
		if end := strings.Index(rest, "```"); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	lower := strings.ToLower(text)
	selectAt := strings.Index(lower, "select")
	withAt := strings.Index(lower, "with")
	start := -1
	if selectAt >= 0 {
		start = selectAt
	}
	if withAt >= 0 && (start < 0 || withAt < start) {
		start = withAt
	}
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(text[start:])
}
