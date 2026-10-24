package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Column struct {
	Name string
	Type string
	PK   bool
}

type DataTable struct {
	Name    string
	Columns []Column
}

func (r *Repository) loadCatalog(ctx context.Context) ([]DataTable, error) {
	var names []string
	err := r.db.WithContext(ctx).Raw(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`).Scan(&names).Error
	if err != nil {
		return nil, err
	}
	tables := make([]DataTable, 0, len(names))
	for _, name := range names {
		if !identRe.MatchString(name) {
			continue
		}
		quoted, err := quoteIdent(name)
		if err != nil {
			continue
		}
		var info []pragmaColumn
		if err := r.db.WithContext(ctx).Raw("PRAGMA table_info(" + quoted + ")").Scan(&info).Error; err != nil {
			return nil, err
		}
		table := DataTable{Name: name, Columns: make([]Column, 0, len(info))}
		for _, column := range info {
			if !identRe.MatchString(column.Name) {
				continue
			}
			table.Columns = append(table.Columns, Column{
				Name: column.Name,
				Type: column.Type,
				PK:   column.PK > 0,
			})
		}
		tables = append(tables, table)
	}
	return tables, nil
}

type pragmaColumn struct {
	CID     int    `gorm:"column:cid"`
	Name    string `gorm:"column:name"`
	Type    string `gorm:"column:type"`
	NotNull int    `gorm:"column:notnull"`
	PK      int    `gorm:"column:pk"`
}

func (r *Repository) countTables(ctx context.Context, catalog []DataTable) (map[string]int64, string, error) {
	counts := map[string]int64{}
	if len(catalog) == 0 {
		return counts, "", nil
	}
	sql, err := countSQL(catalog)
	if err != nil {
		return nil, "", err
	}
	_, rows, err := r.runQuery(ctx, sql, len(catalog)+8)
	if err != nil {
		return nil, "", err
	}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		counts[fmt.Sprint(row[0])] = asInt64(row[1])
	}
	return counts, sql, nil
}

func countSQL(catalog []DataTable) (string, error) {
	var b strings.Builder
	for i, table := range catalog {
		quoted, err := quoteIdent(table.Name)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString(" UNION ALL ")
		}
		fmt.Fprintf(&b, "SELECT '%s' AS \"表\", COUNT(*) AS \"条数\" FROM %s", table.Name, quoted)
	}
	return b.String(), nil
}

func (r *Repository) runQuery(ctx context.Context, sql string, max int) ([]string, [][]any, error) {
	if max <= 0 {
		max = 200
	}
	if max > 1000 {
		max = 1000
	}
	rows, err := r.db.WithContext(ctx).Raw(sql).Rows()
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	out := make([][]any, 0, 16)
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		record := make([]any, len(columns))
		for i, value := range values {
			record[i] = maskCell(columns[i], normalizeCell(value))
		}
		out = append(out, record)
		if len(out) >= max {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return columns, out, nil
}

func normalizeCell(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return coerceScalar(string(typed))
	case string:
		return coerceScalar(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	case bool:
		if typed {
			return int64(1)
		}
		return int64(0)
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return coerceScalar(fmt.Sprint(typed))
	}
}

func coerceScalar(text string) any {
	if text == "" {
		return ""
	}
	if len(text) < 16 && !strings.ContainsAny(text, "eE+") {
		negative := strings.HasPrefix(text, "-")
		digits := text
		if negative {
			digits = text[1:]
		}
		if digits != "" && !strings.Contains(digits, ".") && isDigits(digits) {
			if len(digits) > 1 && digits[0] == '0' {
				return text
			}
			if n, err := strconv.ParseInt(text, 10, 64); err == nil {
				return n
			}
		}
		if strings.Count(digits, ".") == 1 && isDecimal(digits) {
			if n, err := strconv.ParseFloat(text, 64); err == nil {
				return n
			}
		}
	}
	return text
}

func isDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isDecimal(text string) bool {
	seen := false
	digits := false
	for _, r := range text {
		switch {
		case r == '.':
			if seen {
				return false
			}
			seen = true
		case r >= '0' && r <= '9':
			digits = true
		default:
			return false
		}
	}
	return seen && digits
}

func maskCell(column string, value any) any {
	if isSecret(column) && value != nil && value != "" {
		return "已隐藏"
	}
	text, ok := value.(string)
	if !ok {
		return value
	}
	runes := []rune(text)
	if len(runes) > 160 {
		return string(runes[:160]) + "…"
	}
	return text
}

func isSecret(name string) bool {
	switch strings.ToLower(name) {
	case "password_hash", "password", "token", "secret", "api_key":
		return true
	default:
		return false
	}
}

func asInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		n, _ := strconv.ParseInt(typed, 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(typed), 10, 64)
		return n
	}
}

func findCol(table DataTable, name string) (Column, bool) {
	for _, column := range table.Columns {
		if strings.EqualFold(column.Name, name) {
			return column, true
		}
	}
	return Column{}, false
}
