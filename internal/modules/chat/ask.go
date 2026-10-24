package chat

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type TableView struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Rows    int64    `json:"rows"`
}

type AskChart struct {
	Type    string      `json:"type"`
	Title   string      `json:"title"`
	SQL     string      `json:"sql,omitempty"`
	Columns []string    `json:"columns"`
	Rows    [][]any     `json:"rows"`
	X       string      `json:"x,omitempty"`
	Y       string      `json:"y,omitempty"`
	Note    string      `json:"note,omitempty"`
	Tables  []TableView `json:"tables"`
}

type AskOutcome struct {
	Thinking string
	Answer   string
	Chart    *AskChart
}

var tableAliases = map[string][]string{
	"account":           {"账号", "帐户", "用户表"},
	"conversation":      {"对话", "会话记录"},
	"message":           {"消息"},
	"attachment":        {"附件"},
	"login_session":     {"登录会话", "登录记录"},
	"schema_migrations": {"迁移版本", "迁移记录"},
}

var strongAskKeys = []string{
	"问数", "数据表", "哪些表", "所有表", "全部表", "全部数据表", "所有数据表",
	"表结构", "系统表", "柱状图", "折线图", "饼图", "可视化", "图形化",
	"数据量", "各表", "每个表", "每张表", "全库", "多少张表",
}

var weakAskKeys = []string{
	"统计", "趋势", "分布", "占比", "明细", "多少", "查询", "列出", "列表",
	"图表", "画图", "展示", "字段", "结构", "条数", "总数", "求和", "合计",
	"平均", "分组", "汇总", "数量", "有哪些", "查看", "读取", "柱状", "折线", "饼图",
}

func (r *Repository) Ask(ctx context.Context, question, mode string) (AskOutcome, bool, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return AskOutcome{}, false, nil
	}
	if err := ctx.Err(); err != nil {
		return AskOutcome{}, false, err
	}
	catalog, err := r.loadCatalog(ctx)
	if err != nil {
		if !mightAsk(question) {
			return AskOutcome{}, false, nil
		}
		return AskOutcome{}, true, err
	}
	if !isDataQuestion(question, catalog) {
		return AskOutcome{}, false, nil
	}
	counts, countQuery, err := r.countTables(ctx, catalog)
	if err != nil {
		return AskOutcome{}, true, err
	}
	views := viewsFrom(catalog, counts)
	note := secretNote(catalog)
	chart, lead, reason, err := r.plan(ctx, question, catalog, views, countQuery)
	if err != nil {
		if ctx.Err() != nil {
			return AskOutcome{}, true, ctx.Err()
		}
		chart = overviewChart(views, countQuery, note)
		chart.Title = "查询没有完成"
		lead = err.Error() + "下面改为展示每张数据表的条数。"
		reason = "原查询没有执行。"
	}
	if chart == nil {
		chart = overviewChart(views, countQuery, note)
		lead = "已按每张表的条数汇总。"
		reason = "没有定位到单表，改为汇总全部数据表。"
	}
	chart.Tables = views
	if chart.Note == "" {
		chart.Note = note
	}
	if chart.Columns == nil {
		chart.Columns = []string{}
	}
	if chart.Rows == nil {
		chart.Rows = [][]any{}
	}
	return AskOutcome{
		Thinking: buildThinking(mode, views, reason, chart.SQL),
		Answer:   buildAnswer(chart, lead),
		Chart:    chart,
	}, true, nil
}

func mightAsk(question string) bool {
	if _, ok := extractSQL(question); ok {
		return true
	}
	return containsAny(question, strongAskKeys) || containsAny(question, weakAskKeys)
}

func isDataQuestion(question string, catalog []DataTable) bool {
	if _, ok := extractSQL(question); ok {
		return true
	}
	if containsAny(question, strongAskKeys) {
		return true
	}
	score, ties := bestTableScore(question, catalog)
	if score > 0 && ties <= 1 && containsAny(question, weakAskKeys) {
		return true
	}
	if score >= 6 && ties <= 1 && utf8.RuneCountInString(strings.TrimSpace(question)) <= 48 {
		return true
	}
	if distinctiveColumn(question, catalog) && containsAny(question, weakAskKeys) {
		return true
	}
	return false
}

func (r *Repository) plan(ctx context.Context, question string, catalog []DataTable, views []TableView, countQuery string) (*AskChart, string, string, error) {
	note := secretNote(catalog)
	if sqlText, ok := extractSQL(question); ok {
		safe, err := guardSelect(sqlText, catalog)
		if err != nil {
			chart := overviewChart(views, countQuery, note)
			return chart, err.Error() + "下面改为展示每张数据表的条数。", "写入语句已拒绝。", nil
		}
		chart, err := r.queryChart(ctx, catalog, safe, "查询结果", question, "")
		if err != nil {
			return nil, "", "", err
		}
		chart.Note = note
		return chart, "已执行只读查询，结果通过图表事件返回。", "使用了消息里的 SELECT。", nil
	}
	if wantsOverview(question) {
		chart := overviewChart(views, countQuery, note)
		return chart, "系统里的每张数据表都已计数，并按条数画成柱状图。", "问题要看全部数据表，所以对每张表做了 COUNT。", nil
	}
	table, score := bestTable(question, catalog)
	if score == 0 {
		chart := overviewChart(views, countQuery, note)
		return chart, "没有定位到单独一张表，已汇总全部数据表的条数。", "没有足够把握选择单表。", nil
	}
	chart, lead, reason, err := r.planTable(ctx, question, catalog, table)
	if err != nil {
		return nil, "", "", err
	}
	chart.Note = note
	return chart, lead, reason, nil
}

func (r *Repository) planTable(ctx context.Context, question string, catalog []DataTable, table DataTable) (*AskChart, string, string, error) {
	quoted, err := quoteIdent(table.Name)
	if err != nil {
		return nil, "", "", err
	}
	switch {
	case wantsDescribe(question):
		chart := describeChart(table)
		return chart, table.Name + " 的字段如下。其他表的条数也已经读过，列在下方。", "读取 " + table.Name + " 的字段定义。", nil
	case wantsTrend(question, table):
		sql, err := trendSQL(question, table, quoted)
		if err != nil {
			return nil, "", "", err
		}
		chart, err := r.queryChart(ctx, catalog, sql, table.Name+" 的趋势", question, "line")
		if err != nil {
			return nil, "", "", err
		}
		return chart, table.Name + " 已按时间汇总，并用折线图展示。", "按时间汇总 " + table.Name + "。", nil
	case wantsAggregate(question):
		if column, ok := groupColumn(question, table); ok {
			sql, err := groupSQL(question, table, quoted, column)
			if err != nil {
				return nil, "", "", err
			}
			chart, err := r.queryChart(ctx, catalog, sql, table.Name+" 按 "+column.Name, question, "")
			if err != nil {
				return nil, "", "", err
			}
			return chart, table.Name + " 已按 " + column.Name + " 汇总。", "分组字段是 " + column.Name + "。", nil
		}
	}
	if wantsCount(question) {
		sql := fmt.Sprintf("SELECT COUNT(*) AS \"条数\" FROM %s", quoted)
		chart, err := r.queryChart(ctx, catalog, sql, table.Name+" 的条数", question, "metric")
		if err != nil {
			return nil, "", "", err
		}
		return chart, table.Name + " 的条数如下。", "对 " + table.Name + " 做了计数。", nil
	}
	sql, hidden, err := listSQL(question, table, quoted)
	if err != nil {
		return nil, "", "", err
	}
	chart, err := r.queryChart(ctx, catalog, sql, table.Name+" 的数据", question, "table")
	if err != nil {
		return nil, "", "", err
	}
	lead := "已列出 " + table.Name + " 的数据。"
	if hidden {
		lead += "敏感字段的值已隐藏。"
	}
	return chart, lead, "读取 " + table.Name + " 的明细。", nil
}

func (r *Repository) queryChart(ctx context.Context, catalog []DataTable, sql, title, question, forced string) (*AskChart, error) {
	safe, err := guardSelect(sql, catalog)
	if err != nil {
		return nil, err
	}
	columns, rows, err := r.runQuery(ctx, safe, 200)
	if err != nil {
		return nil, err
	}
	kind, x, y := pickChart(question, columns, rows, forced)
	return &AskChart{
		Type:    kind,
		Title:   title,
		SQL:     safe,
		Columns: columns,
		Rows:    rows,
		X:       x,
		Y:       y,
	}, nil
}

func overviewChart(views []TableView, sql, note string) *AskChart {
	rows := make([][]any, 0, len(views))
	for _, view := range views {
		rows = append(rows, []any{view.Name, view.Rows})
	}
	return &AskChart{
		Type:    "bar",
		Title:   "各数据表条数",
		SQL:     sql,
		Columns: []string{"表", "条数"},
		Rows:    rows,
		X:       "表",
		Y:       "条数",
		Note:    note,
		Tables:  views,
	}
}

func describeChart(table DataTable) *AskChart {
	rows := make([][]any, 0, len(table.Columns))
	for _, column := range table.Columns {
		kind := column.Type
		if kind == "" {
			kind = "ANY"
		}
		pk := "否"
		if column.PK {
			pk = "是"
		}
		rows = append(rows, []any{column.Name, kind, pk})
	}
	return &AskChart{
		Type:    "table",
		Title:   table.Name + " 的字段",
		Columns: []string{"字段", "类型", "主键"},
		Rows:    rows,
	}
}

func trendSQL(question string, table DataTable, quoted string) (string, error) {
	when, ok := timeColumn(table)
	if !ok {
		return "", fmt.Errorf("%s 没有可按时间汇总的字段", table.Name)
	}
	timeQuoted, err := quoteIdent(when.Name)
	if err != nil {
		return "", err
	}
	width := 10
	label := "日期"
	if strings.Contains(question, "按月") || strings.Contains(question, "每月") || strings.Contains(question, "月份") {
		width = 7
		label = "月份"
	}
	valueSQL := `COUNT(*) AS "数量"`
	if column, sum, avg, ok := measureColumn(question, table); ok {
		quotedColumn, err := quoteIdent(column.Name)
		if err != nil {
			return "", err
		}
		switch {
		case avg:
			valueSQL = fmt.Sprintf(`AVG(%s) AS "平均值"`, quotedColumn)
		case sum:
			valueSQL = fmt.Sprintf(`SUM(%s) AS "合计"`, quotedColumn)
		}
	}
	where := ""
	if filter := timeFilter(question); filter != "" {
		where = fmt.Sprintf(" WHERE %s >= datetime('now', '%s')", timeQuoted, filter)
	}
	return fmt.Sprintf(
		`SELECT substr(%s, 1, %d) AS "%s", %s FROM %s%s GROUP BY 1 ORDER BY 1 LIMIT 200`,
		timeQuoted, width, label, valueSQL, quoted, where,
	), nil
}

func groupSQL(question string, table DataTable, quoted string, column Column) (string, error) {
	groupQuoted, err := quoteIdent(column.Name)
	if err != nil {
		return "", err
	}
	valueSQL := `COUNT(*) AS "数量"`
	if measure, sum, avg, ok := measureColumn(question, table); ok && !strings.EqualFold(measure.Name, column.Name) {
		measureQuoted, err := quoteIdent(measure.Name)
		if err != nil {
			return "", err
		}
		switch {
		case avg:
			valueSQL = fmt.Sprintf(`AVG(%s) AS "平均值"`, measureQuoted)
		case sum:
			valueSQL = fmt.Sprintf(`SUM(%s) AS "合计"`, measureQuoted)
		default:
			valueSQL = fmt.Sprintf(`SUM(%s) AS "合计"`, measureQuoted)
		}
	}
	limit := rowLimit(question, 50)
	return fmt.Sprintf(
		`SELECT %s AS "%s", %s FROM %s GROUP BY %s ORDER BY 2 DESC LIMIT %d`,
		groupQuoted, column.Name, valueSQL, quoted, groupQuoted, limit,
	), nil
}

func listSQL(question string, table DataTable, quoted string) (string, bool, error) {
	parts := make([]string, 0, len(table.Columns))
	hidden := false
	for _, column := range table.Columns {
		if isSecret(column.Name) {
			hidden = true
			continue
		}
		item, err := quoteIdent(column.Name)
		if err != nil {
			return "", false, err
		}
		parts = append(parts, item)
	}
	if len(parts) == 0 {
		parts = append(parts, `COUNT(*) AS "条数"`)
	}
	limit := rowLimit(question, 50)
	order := ""
	if column := orderColumn(table); column != "" {
		quotedColumn, err := quoteIdent(column)
		if err != nil {
			return "", false, err
		}
		order = " ORDER BY " + quotedColumn + " DESC"
	}
	return fmt.Sprintf("SELECT %s FROM %s%s LIMIT %d", strings.Join(parts, ", "), quoted, order, limit), hidden, nil
}

func wantsOverview(question string) bool {
	keys := []string{
		"哪些表", "所有表", "全部表", "全部数据表", "所有数据表", "系统表",
		"各表", "每个表", "每张表", "数据表", "全库", "多少张", "系统全部",
	}
	if !containsAny(question, keys) {
		return false
	}
	if containsAny(question, []string{"所有", "全部", "各表", "每个", "每张", "哪些", "系统", "全库", "多少张"}) {
		return true
	}
	return strings.Contains(question, "数据表")
}

func wantsDescribe(question string) bool {
	if strings.Contains(question, "项目结构") {
		return false
	}
	return strings.Contains(question, "表结构") || strings.Contains(question, "字段") || strings.Contains(question, "哪些列") || strings.Contains(question, "什么列") || strings.Contains(question, "结构")
}

func wantsTrend(question string, table DataTable) bool {
	if _, ok := timeColumn(table); !ok {
		return false
	}
	if !(strings.Contains(question, "趋势") || strings.Contains(question, "折线") || strings.Contains(question, "按天") || strings.Contains(question, "按日") || strings.Contains(question, "每天") || strings.Contains(question, "按月") || strings.Contains(question, "每月") || strings.Contains(question, "按日期")) {
		return false
	}
	if column, ok := groupColumn(question, table); ok && !isTimeName(column.Name) {
		if strings.Contains(question, "按天") || strings.Contains(question, "按月") || strings.Contains(question, "每天") || strings.Contains(question, "每月") || strings.Contains(question, "按日") || strings.Contains(question, "按日期") {
			return true
		}
		return false
	}
	return true
}

func wantsAggregate(question string) bool {
	return containsAny(question, []string{"统计", "分布", "占比", "分组", "聚合", "饼", "柱", "图表", "图形", "多少", "求和", "合计", "平均", "金额", "汇总", "数量"})
}

func wantsCount(question string) bool {
	return containsAny(question, []string{"多少", "几条", "总数", "条数", "数据量", "有几", "几个"})
}

func bestTable(question string, catalog []DataTable) (DataTable, int) {
	var found DataTable
	score, ties := 0, 0
	for _, table := range catalog {
		current := scoreTable(question, table)
		switch {
		case current > score:
			found = table
			score = current
			ties = 1
		case current == score && current > 0:
			ties++
		}
	}
	if ties > 1 {
		return DataTable{}, 0
	}
	return found, score
}

func bestTableScore(question string, catalog []DataTable) (int, int) {
	score, ties := 0, 0
	for _, table := range catalog {
		current := 0
		if containsIdent(question, table.Name) {
			current += 6
		}
		for _, alias := range tableAliases[table.Name] {
			if strings.Contains(question, alias) {
				current += 5
			}
		}
		switch {
		case current > score:
			score = current
			ties = 1
		case current == score && current > 0:
			ties++
		}
	}
	return score, ties
}

func scoreTable(question string, table DataTable) int {
	score := 0
	if containsIdent(question, table.Name) {
		score += 6
	}
	for _, alias := range tableAliases[table.Name] {
		if strings.Contains(question, alias) {
			score += 5
		}
	}
	for _, column := range table.Columns {
		if !distinctiveName(column.Name) {
			continue
		}
		if containsIdent(question, column.Name) {
			score += 2
		}
	}
	return score
}

func distinctiveColumn(question string, catalog []DataTable) bool {
	for _, table := range catalog {
		for _, column := range table.Columns {
			if distinctiveName(column.Name) && containsIdent(question, column.Name) {
				return true
			}
		}
	}
	return false
}

func distinctiveName(name string) bool {
	if len(name) < 4 {
		return false
	}
	switch strings.ToLower(name) {
	case "name", "title", "type", "status", "content", "model", "role", "data", "user", "text", "value", "path", "created", "updated":
		return false
	default:
		return true
	}
}

func groupColumn(question string, table DataTable) (Column, bool) {
	for _, column := range table.Columns {
		if isIDName(column.Name) || isSecret(column.Name) || isNumericType(column.Type) || isTimeName(column.Name) {
			continue
		}
		if containsIdent(question, column.Name) {
			return column, true
		}
	}
	hints := []struct {
		keys []string
		name string
	}{
		{[]string{"模型"}, "model"},
		{[]string{"角色"}, "role"},
		{[]string{"状态"}, "status"},
		{[]string{"思考模式"}, "thinking_mode"},
		{[]string{"附件类型", "文件类型"}, "media_type"},
		{[]string{"用户名"}, "username"},
		{[]string{"标题"}, "title"},
		{[]string{"名称"}, "name"},
		{[]string{"区域", "地区"}, "region"},
	}
	for _, hint := range hints {
		for _, key := range hint.keys {
			if strings.Contains(question, key) {
				if column, ok := findCol(table, hint.name); ok {
					return column, true
				}
			}
		}
	}
	cats := categorical(table)
	if len(cats) == 1 {
		return cats[0], true
	}
	return Column{}, false
}

func measureColumn(question string, table DataTable) (Column, bool, bool, bool) {
	sumHint := containsAny(question, []string{"求和", "总和", "合计", "金额", "销售额", "sum"})
	avg := strings.Contains(question, "平均")
	for _, column := range numericMeasures(table) {
		if containsIdent(question, column.Name) {
			return column, !avg, avg, true
		}
	}
	if sumHint || avg || strings.Contains(question, "金额") {
		for _, name := range []string{"amount", "total", "price", "money", "sales", "fee"} {
			if column, ok := findCol(table, name); ok && isNumericType(column.Type) {
				return column, true, avg, true
			}
		}
		measures := numericMeasures(table)
		if len(measures) == 1 {
			return measures[0], true, avg, true
		}
	}
	return Column{}, false, false, false
}

func numericMeasures(table DataTable) []Column {
	out := make([]Column, 0, 2)
	for _, column := range table.Columns {
		if isIDName(column.Name) || !isNumericType(column.Type) {
			continue
		}
		out = append(out, column)
	}
	return out
}

func categorical(table DataTable) []Column {
	out := make([]Column, 0, 4)
	for _, column := range table.Columns {
		if isIDName(column.Name) || isSecret(column.Name) || isNumericType(column.Type) || isTimeName(column.Name) {
			continue
		}
		switch strings.ToLower(column.Name) {
		case "content", "thinking", "password_hash", "token":
			continue
		}
		out = append(out, column)
	}
	return out
}

func timeColumn(table DataTable) (Column, bool) {
	for _, name := range []string{"created_at", "updated_at", "applied_at", "expires_at"} {
		if column, ok := findCol(table, name); ok {
			return column, true
		}
	}
	for _, column := range table.Columns {
		upper := strings.ToUpper(column.Type)
		if strings.Contains(upper, "DATE") || strings.Contains(upper, "TIME") {
			return column, true
		}
	}
	return Column{}, false
}

func orderColumn(table DataTable) string {
	for _, name := range []string{"created_at", "updated_at", "applied_at", "expires_at", "id"} {
		if _, ok := findCol(table, name); ok {
			return name
		}
	}
	if len(table.Columns) > 0 {
		return table.Columns[0].Name
	}
	return ""
}

func isNumericType(kind string) bool {
	upper := strings.ToUpper(kind)
	return strings.Contains(upper, "INT") || strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") || strings.Contains(upper, "NUM") || strings.Contains(upper, "DEC") || strings.Contains(upper, "DOUB")
}

func isIDName(name string) bool {
	lower := strings.ToLower(name)
	return lower == "id" || strings.HasSuffix(lower, "_id")
}

func isTimeName(name string) bool {
	switch strings.ToLower(name) {
	case "created_at", "updated_at", "applied_at", "expires_at":
		return true
	default:
		return false
	}
}

func timeFilter(question string) string {
	recent := strings.Contains(question, "最近") || strings.Contains(question, "过去") || strings.Contains(question, "近7") || strings.Contains(question, "近30") || strings.Contains(question, "近90") || strings.Contains(question, "近一")
	if !recent {
		return ""
	}
	switch {
	case strings.Contains(question, "7天") || strings.Contains(question, "七天") || strings.Contains(question, "一周") || strings.Contains(question, "近7"):
		return "-7 day"
	case strings.Contains(question, "90天") || strings.Contains(question, "三个月"):
		return "-90 day"
	case strings.Contains(question, "一年") || strings.Contains(question, "12个月"):
		return "-1 year"
	default:
		return "-30 day"
	}
}

func pickChart(question string, columns []string, rows [][]any, forced string) (string, string, string) {
	x, y := chartAxes(columns, rows)
	if forced == "table" {
		return "table", "", ""
	}
	chartWords := containsAny(question, []string{"柱", "饼", "折线", "趋势"})
	if forced == "metric" && !chartWords {
		if len(columns) > 0 {
			return "metric", columns[0], columns[0]
		}
		return "metric", "", ""
	}
	if len(columns) == 1 && len(rows) == 1 && isNumber(rows[0][0]) && !chartWords {
		return "metric", columns[0], columns[0]
	}
	if forced == "line" && y != "" {
		return "line", x, y
	}
	if forced == "bar" && y != "" {
		return "bar", x, y
	}
	if y == "" {
		return "table", "", ""
	}
	switch {
	case strings.Contains(question, "饼"):
		return "pie", x, y
	case strings.Contains(question, "折线") || strings.Contains(question, "趋势"):
		return "line", x, y
	case strings.Contains(question, "柱") || strings.Contains(question, "条形"):
		return "bar", x, y
	case strings.Contains(question, "表格") || strings.Contains(question, "明细") || strings.Contains(question, "列出") || strings.Contains(question, "列表"):
		return "table", "", ""
	}
	if looksDate(rows, indexOf(columns, x)) {
		return "line", x, y
	}
	if (strings.Contains(question, "占比") || strings.Contains(question, "分布")) && len(rows) > 0 && len(rows) <= 8 {
		return "pie", x, y
	}
	return "bar", x, y
}

func chartAxes(columns []string, rows [][]any) (string, string) {
	yIndex := -1
	for i := range columns {
		if isIDName(columns[i]) {
			continue
		}
		if columnMostlyNumeric(rows, i) {
			yIndex = i
			break
		}
	}
	if yIndex < 0 {
		return "", ""
	}
	if len(columns) == 1 {
		return columns[0], columns[0]
	}
	xIndex := -1
	for i := range columns {
		if i == yIndex {
			continue
		}
		if !columnMostlyNumeric(rows, i) {
			xIndex = i
			break
		}
	}
	if xIndex < 0 {
		return "", ""
	}
	return columns[xIndex], columns[yIndex]
}

func columnMostlyNumeric(rows [][]any, index int) bool {
	seen := 0
	numeric := 0
	for _, row := range rows {
		if index >= len(row) || row[index] == nil || row[index] == "" {
			continue
		}
		seen++
		if isNumber(row[index]) {
			numeric++
		}
	}
	return seen > 0 && numeric*2 >= seen
}

func isNumber(value any) bool {
	switch value.(type) {
	case int, int64, int32, float64, float32:
		return true
	default:
		return false
	}
}

func looksDate(rows [][]any, index int) bool {
	if index < 0 {
		return false
	}
	seen := 0
	for _, row := range rows {
		if index >= len(row) {
			continue
		}
		text := fmt.Sprint(row[index])
		if len(text) < 7 {
			continue
		}
		seen++
		if text[4] != '-' {
			return false
		}
	}
	return seen > 0
}

func indexOf(columns []string, name string) int {
	for i, column := range columns {
		if column == name {
			return i
		}
	}
	return -1
}

func viewsFrom(catalog []DataTable, counts map[string]int64) []TableView {
	views := make([]TableView, 0, len(catalog))
	for _, table := range catalog {
		columns := make([]string, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, column.Name)
		}
		views = append(views, TableView{Name: table.Name, Columns: columns, Rows: counts[table.Name]})
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Rows == views[j].Rows {
			return views[i].Name < views[j].Name
		}
		return views[i].Rows > views[j].Rows
	})
	return views
}

func secretNote(catalog []DataTable) string {
	for _, table := range catalog {
		for _, column := range table.Columns {
			if isSecret(column.Name) {
				return "密码哈希和登录令牌已隐藏，结果里不会出现原文。"
			}
		}
	}
	return ""
}

func buildThinking(mode string, tables []TableView, reason, sql string) string {
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, table.Name)
	}
	head := "已读取当前库全部 " + strconv.Itoa(len(tables)) + " 张数据表：" + strings.Join(names, "、") + "。"
	if mode == "light" || mode == "deep" {
		text := "问数不会使用写死的表白名单，而是读取 sqlite_master 里的每张业务表。\n" + head
		if reason != "" {
			text += "\n" + reason
		}
		if mode == "deep" && sql != "" {
			text += "\n查询：\n" + sql
		}
		return text
	}
	if reason == "" {
		return head
	}
	return head + reason
}

func buildAnswer(chart *AskChart, lead string) string {
	var b strings.Builder
	b.WriteString("## 结论\n\n")
	b.WriteString(lead)
	b.WriteString("\n\n已读取系统全部 ")
	b.WriteString(strconv.Itoa(len(chart.Tables)))
	b.WriteString(" 张数据表：\n\n")
	for _, table := range chart.Tables {
		columns := strings.Join(table.Columns, "、")
		if columns == "" {
			columns = "无字段"
		}
		fmt.Fprintf(&b, "- `%s`：%d 条，字段 %s\n", table.Name, table.Rows, columns)
	}
	b.WriteString("\n")
	if len(chart.Rows) == 0 {
		b.WriteString("这次查询没有返回行。\n")
	} else {
		b.WriteString("### 查询结果\n\n")
		b.WriteString(markdownTable(chart.Columns, chart.Rows, 20))
		if len(chart.Rows) > 20 {
			fmt.Fprintf(&b, "\n图表里保留了全部 %d 行，这里只列出前 20 行。\n", len(chart.Rows))
		}
	}
	if chart.Note != "" {
		b.WriteString("\n")
		b.WriteString(chart.Note)
		b.WriteString("\n")
	}
	return b.String()
}

func markdownTable(columns []string, rows [][]any, limit int) string {
	if len(columns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| ")
	b.WriteString(strings.Join(columns, " | "))
	b.WriteString(" |\n| ")
	for i := range columns {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("---")
	}
	b.WriteString(" |\n")
	shown := rows
	if len(shown) > limit {
		shown = shown[:limit]
	}
	for _, row := range shown {
		b.WriteString("| ")
		for i := range columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			if i < len(row) {
				b.WriteString(cellText(row[i]))
			}
		}
		b.WriteString(" |\n")
	}
	return b.String()
}

func cellText(value any) string {
	if value == nil {
		return ""
	}
	text := fmt.Sprint(value)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "|", "\\|")
	return text
}

func containsAny(text string, keys []string) bool {
	for _, key := range keys {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func containsIdent(text, ident string) bool {
	ident = strings.ToLower(ident)
	if len(ident) < 3 {
		return false
	}
	lower := strings.ToLower(text)
	start := 0
	for start < len(lower) {
		index := strings.Index(lower[start:], ident)
		if index < 0 {
			return false
		}
		index += start
		before := index == 0 || !isIdentByte(lower[index-1])
		after := index+len(ident) >= len(lower) || !isIdentByte(lower[index+len(ident)])
		if before && after {
			return true
		}
		start = index + len(ident)
	}
	return false
}

func isIdentByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_'
}
