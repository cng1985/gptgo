package chatbi

import (
	"fmt"
	"strings"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
)

type Column struct {
	Schema   string
	Table    string
	Name     string
	DataType string
}

type tableDef struct {
	Schema  string
	Name    string
	Columns []Column
}

func Plan(question, driver, database string, columns []Column) (string, string, error) {
	tables := groupTables(columns)
	if len(tables) == 0 {
		return "", "", apperr.Invalid("这个数据源里没有可读的表")
	}
	table := matchTable(question, tables)
	if table == nil {
		return "", "", apperr.Invalid("没有在问题里找到对应的表。当前表：" + joinNames(tables, 12))
	}
	ref, err := tableRef(driver, database, table.Schema, table.Name)
	if err != nil {
		return "", "", apperr.Invalid("表名无法用于查询")
	}
	agg := detectAgg(question)
	grouped := wantsGroup(question)
	series := wantsSeries(question)
	if agg == "" && !grouped && !series {
		sql, err := selectList(driver, ref, table.Columns)
		if err != nil {
			return "", "", err
		}
		return sql, "按「" + table.Name + "」列出字段", nil
	}
	if agg == "" {
		agg = "count"
	}
	measure, measureOK := pickMeasure(question, table.Columns)
	dimension := pickDimension(question, table.Columns, series)
	if (agg == "sum" || agg == "avg" || agg == "max" || agg == "min") && !measureOK {
		agg = "count"
	}
	expr := "COUNT(*)"
	switch agg {
	case "sum":
		quoted, qerr := QuoteIdent(driver, measure.Name)
		if qerr != nil {
			return "", "", apperr.Invalid("字段名无法用于查询")
		}
		expr = "SUM(" + quoted + ")"
	case "avg":
		quoted, qerr := QuoteIdent(driver, measure.Name)
		if qerr != nil {
			return "", "", apperr.Invalid("字段名无法用于查询")
		}
		expr = "AVG(" + quoted + ")"
	case "max":
		quoted, qerr := QuoteIdent(driver, measure.Name)
		if qerr != nil {
			return "", "", apperr.Invalid("字段名无法用于查询")
		}
		expr = "MAX(" + quoted + ")"
	case "min":
		quoted, qerr := QuoteIdent(driver, measure.Name)
		if qerr != nil {
			return "", "", apperr.Invalid("字段名无法用于查询")
		}
		expr = "MIN(" + quoted + ")"
	}
	if dimension == nil {
		return "SELECT " + expr + " AS total FROM " + ref, "统计「" + table.Name + "」", nil
	}
	dimSQL, err := dimensionExpr(driver, dimension)
	if err != nil {
		return "", "", err
	}
	label := dimension.Name
	if strings.EqualFold(label, "total") {
		label = "bucket"
	}
	dimName, err := QuoteIdent(driver, label)
	if err != nil {
		return "", "", apperr.Invalid("字段名无法用于查询")
	}
	sql := "SELECT " + dimSQL + " AS " + dimName + ", " + expr + " AS total FROM " + ref + " GROUP BY " + dimSQL + " ORDER BY total DESC"
	return sql, "按「" + dimension.Name + "」汇总「" + table.Name + "」", nil
}

func SchemaQuestion(question string) bool {
	text := strings.ToLower(strings.TrimSpace(question))
	for _, key := range []string{"有哪些表", "有什么表", "表结构", "哪些表", "什么字段", "有哪些字段", "数据结构", "schema"} {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func groupTables(columns []Column) []tableDef {
	order := make([]string, 0)
	index := map[string]int{}
	for _, column := range columns {
		if !identPattern.MatchString(column.Table) || !identPattern.MatchString(column.Name) {
			continue
		}
		if column.Schema != "" && !identPattern.MatchString(column.Schema) {
			continue
		}
		key := column.Schema + "\x00" + column.Table
		if _, ok := index[key]; !ok {
			index[key] = len(order)
			order = append(order, key)
		}
	}
	tables := make([]tableDef, len(order))
	for key, pos := range index {
		parts := strings.SplitN(key, "\x00", 2)
		tables[pos] = tableDef{Schema: parts[0], Name: parts[1]}
	}
	for _, column := range columns {
		if !identPattern.MatchString(column.Table) || !identPattern.MatchString(column.Name) {
			continue
		}
		if column.Schema != "" && !identPattern.MatchString(column.Schema) {
			continue
		}
		key := column.Schema + "\x00" + column.Table
		pos, ok := index[key]
		if !ok {
			continue
		}
		tables[pos].Columns = append(tables[pos].Columns, column)
	}
	return tables
}

func matchTable(question string, tables []tableDef) *tableDef {
	q := strings.ToLower(question)
	var best *tableDef
	bestLen := 0
	for i := range tables {
		name := strings.ToLower(tables[i].Name)
		if name != "" && strings.Contains(q, name) && utf8.RuneCountInString(name) > bestLen {
			best = &tables[i]
			bestLen = utf8.RuneCountInString(name)
		}
	}
	if best != nil {
		return best
	}
	scores := make([]int, len(tables))
	bestScore := 0
	bestAt := -1
	for i := range tables {
		for _, column := range tables[i].Columns {
			n := strings.ToLower(column.Name)
			if utf8.RuneCountInString(n) < 3 || isIDName(n) {
				continue
			}
			if strings.Contains(q, n) {
				scores[i]++
			}
		}
		if scores[i] > bestScore {
			bestScore = scores[i]
			bestAt = i
		}
	}
	if bestScore > 0 {
		return &tables[bestAt]
	}
	if len(tables) == 1 {
		return &tables[0]
	}
	return nil
}

func tableRef(driver, database, schema, table string) (string, error) {
	quoted, err := QuoteIdent(driver, table)
	if err != nil {
		return "", err
	}
	if schema == "" || driver == "mysql" || strings.EqualFold(schema, database) {
		return quoted, nil
	}
	prefix, err := QuoteIdent(driver, schema)
	if err != nil {
		return "", err
	}
	return prefix + "." + quoted, nil
}

func selectList(driver, ref string, columns []Column) (string, error) {
	parts := make([]string, 0, 8)
	for _, column := range columns {
		if len(parts) == 8 {
			break
		}
		quoted, err := QuoteIdent(driver, column.Name)
		if err != nil {
			continue
		}
		parts = append(parts, quoted)
	}
	if len(parts) == 0 {
		return "", apperr.Invalid("表里没有可查询的字段")
	}
	return "SELECT " + strings.Join(parts, ", ") + " FROM " + ref, nil
}

func dimensionExpr(driver string, column *Column) (string, error) {
	quoted, err := QuoteIdent(driver, column.Name)
	if err != nil {
		return "", apperr.Invalid("字段名无法用于查询")
	}
	if columnKind(column.DataType) != "time" {
		return quoted, nil
	}
	return "CAST(" + quoted + " AS DATE)", nil
}

func pickMeasure(question string, columns []Column) (Column, bool) {
	q := strings.ToLower(question)
	for _, column := range columns {
		if columnKind(column.DataType) != "number" || isIDName(column.Name) {
			continue
		}
		if strings.Contains(q, strings.ToLower(column.Name)) {
			return column, true
		}
	}
	for _, column := range columns {
		if columnKind(column.DataType) == "number" && !isIDName(column.Name) {
			return column, true
		}
	}
	return Column{}, false
}

func pickDimension(question string, columns []Column, series bool) *Column {
	q := strings.ToLower(question)
	if series {
		for i := range columns {
			if columnKind(columns[i].DataType) == "time" {
				return &columns[i]
			}
		}
	}
	for i := range columns {
		name := strings.ToLower(columns[i].Name)
		if utf8.RuneCountInString(name) < 2 || isIDName(name) {
			continue
		}
		if strings.Contains(q, name) && columnKind(columns[i].DataType) != "number" {
			return &columns[i]
		}
	}
	for i := range columns {
		if columnKind(columns[i].DataType) == "text" && !isIDName(columns[i].Name) {
			return &columns[i]
		}
	}
	for i := range columns {
		if columnKind(columns[i].DataType) == "time" {
			return &columns[i]
		}
	}
	return nil
}

func detectAgg(question string) string {
	q := strings.ToLower(question)
	switch {
	case containsAny(q, "平均", "均值", "avg"):
		return "avg"
	case containsAny(q, "最大", "最高", "max"):
		return "max"
	case containsAny(q, "最小", "最低", "min"):
		return "min"
	case containsAny(q, "求和", "总额", "合计", "销售额", "金额", "sum"):
		return "sum"
	case containsAny(q, "数量", "多少", "总数", "几条", "条数", "count"):
		return "count"
	default:
		return ""
	}
}

func wantsGroup(question string) bool {
	return containsAny(strings.ToLower(question), "每个", "各个", "按", "分组", "分布", "排名", "占比")
}

func wantsSeries(question string) bool {
	return containsAny(strings.ToLower(question), "趋势", "每天", "每日", "每月", "按日", "按月", "时间")
}

func containsAny(text string, keys ...string) bool {
	for _, key := range keys {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func columnKind(dataType string) string {
	t := strings.ToLower(strings.TrimSpace(dataType))
	switch {
	case strings.Contains(t, "interval"):
		return "time"
	case strings.Contains(t, "date"), strings.Contains(t, "time"):
		return "time"
	case strings.Contains(t, "char"), strings.Contains(t, "text"), strings.Contains(t, "string"), strings.Contains(t, "json"), t == "uuid", strings.Contains(t, "xml"):
		return "text"
	case strings.Contains(t, "int"), strings.Contains(t, "decimal"), strings.Contains(t, "numeric"), strings.Contains(t, "float"), strings.Contains(t, "double"), strings.Contains(t, "real"), strings.Contains(t, "money"), strings.Contains(t, "number"), strings.Contains(t, "serial"):
		return "number"
	default:
		return "other"
	}
}

func isIDName(name string) bool {
	n := strings.ToLower(name)
	return n == "id" || strings.HasSuffix(n, "_id")
}

func joinNames(tables []tableDef, limit int) string {
	names := make([]string, 0, limit)
	for _, table := range tables {
		if len(names) == limit {
			break
		}
		names = append(names, table.Name)
	}
	text := strings.Join(names, "、")
	if len(tables) > limit {
		text += fmt.Sprintf(" 等 %d 张表", len(tables))
	}
	return text
}
