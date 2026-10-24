package chatbi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

func (r *Resource) Ask(ctx context.Context, request *AskRequest) (response.Response[AskResponse], error) {
	var zero response.Response[AskResponse]
	if request == nil || request.DatasourceID <= 0 {
		return zero, apperr.NotFound("数据源不存在")
	}
	question := strings.TrimSpace(request.Question)
	if question == "" || utf8.RuneCountInString(question) > 500 {
		return zero, apperr.Invalid("问题需要是 1 到 500 个字")
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = "builtin"
	}
	if _, ok := Models[model]; !ok {
		return zero, apperr.Invalid("不支持的模型")
	}
	entity, err := r.repo.FindSource(ctx, request.DatasourceID)
	if err != nil {
		return zero, err
	}
	password, err := r.passwordOf(entity, "")
	if err != nil {
		return zero, err
	}
	balance, err := r.repo.EnsureWallet(ctx)
	if err != nil {
		return zero, err
	}
	if balance < AskCost {
		return zero, apperr.Forbidden("余额不足，请到账单页充值")
	}
	info := connFrom(entity, password)
	started := time.Now()
	columns, err := r.engine.Schema(ctx, info)
	if err != nil {
		r.remember(ctx, entity, question, "", "error", 0, started, 0, 0, redact(err.Error(), password))
		return zero, connectErr(err, password)
	}
	if len(groupTables(columns)) == 0 {
		r.remember(ctx, entity, question, "", "error", 0, started, 0, 0, "这个数据源里没有可读的表")
		return zero, apperr.Invalid("这个数据源里没有可读的表")
	}
	if SchemaQuestion(question) {
		return r.finishSchema(ctx, entity, question, columns, started)
	}
	sqlText, note, err := r.proposeSQL(ctx, question, model, info, columns)
	if err != nil {
		r.remember(ctx, entity, question, "", "error", 0, started, estimateTokens(question+schemaPrompt(columns)), 0, err.Error())
		return zero, err
	}
	result, err := r.engine.Query(ctx, info, sqlText, MaxRows)
	if err != nil {
		r.remember(ctx, entity, question, sqlText, "error", 0, started, estimateTokens(question+schemaPrompt(columns)), estimateTokens(sqlText), redact(err.Error(), password))
		return zero, queryFailed(err, password)
	}
	if result.Columns == nil {
		result.Columns = []string{}
	}
	if result.Rows == nil {
		result.Rows = [][]any{}
	}
	promptTokens := estimateTokens(question + schemaPrompt(columns))
	completionTokens := estimateTokens(sqlText)
	record := usageFrom(entity, question, sqlText, "ok", int64(len(result.Rows)), started, promptTokens, completionTokens, AskCost, "")
	balance, err = r.repo.SaveUsage(ctx, record)
	if err != nil {
		return zero, err
	}
	summary := fmt.Sprintf("%s，返回 %d 行。", note, len(result.Rows))
	if result.Truncated {
		summary += fmt.Sprintf("最多展示 %d 行。", MaxRows)
	}
	summary += fmt.Sprintf(" 本次扣费 %d，余额 %d。", AskCost, balance)
	return response.OK(AskResponse{
		UsageID:          record.ID,
		SQL:              sqlText,
		Columns:          result.Columns,
		Rows:             result.Rows,
		Chart:            detectChart(result.Columns, result.Rows),
		Summary:          summary,
		RowCount:         len(result.Rows),
		Truncated:        result.Truncated,
		DurationMs:       time.Since(started).Milliseconds(),
		Cost:             AskCost,
		Balance:          balance,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}), nil
}

func (r *Resource) finishSchema(ctx context.Context, entity Datasource, question string, columns []Column, started time.Time) (response.Response[AskResponse], error) {
	var zero response.Response[AskResponse]
	names := []string{"表", "字段", "类型"}
	rows := make([][]any, 0, len(columns))
	truncated := false
	for _, column := range columns {
		if len(rows) >= MaxRows {
			truncated = true
			break
		}
		table := column.Table
		if column.Schema != "" && !strings.EqualFold(column.Schema, entity.DatabaseName) {
			table = column.Schema + "." + column.Table
		}
		rows = append(rows, []any{table, column.Name, column.DataType})
	}
	note := fmt.Sprintf("当前数据源共 %d 张表", len(groupSchema(columns)))
	record := usageFrom(entity, question, "", "ok", int64(len(rows)), started, estimateTokens(question), estimateTokens(note), AskCost, "")
	balance, err := r.repo.SaveUsage(ctx, record)
	if err != nil {
		return zero, err
	}
	summary := fmt.Sprintf("%s，返回 %d 行。", note, len(rows))
	if truncated {
		summary += fmt.Sprintf("最多展示 %d 行。", MaxRows)
	}
	summary += fmt.Sprintf(" 本次扣费 %d，余额 %d。", AskCost, balance)
	return response.OK(AskResponse{
		UsageID:          record.ID,
		SQL:              "",
		Columns:          names,
		Rows:             rows,
		Chart:            "table",
		Summary:          summary,
		RowCount:         len(rows),
		Truncated:        truncated,
		DurationMs:       time.Since(started).Milliseconds(),
		Cost:             AskCost,
		Balance:          balance,
		PromptTokens:     record.PromptTokens,
		CompletionTokens: record.CompletionTokens,
	}), nil
}

func (r *Resource) proposeSQL(ctx context.Context, question, model string, info ConnInfo, columns []Column) (string, string, error) {
	if model != "builtin" && r.completer != nil {
		system := "你是 ChatBI。根据表结构为用户问题生成一条只读 SQL。只输出 SQL，不要解释。禁止写入、DDL 和多条语句。数据库类型：" + info.Driver
		user := "表结构:\n" + schemaPrompt(columns) + "\n问题:\n" + question
		raw, err := r.completer.Complete(ctx, system, user, model)
		if err == nil {
			if guarded, gerr := Guard(extractSQL(raw), info.Driver, MaxRows); gerr == nil {
				return guarded, "由模型生成的只读查询", nil
			}
		}
	}
	planned, note, err := Plan(question, info.Driver, info.Database, columns)
	if err != nil {
		return "", "", err
	}
	guarded, err := Guard(planned, info.Driver, MaxRows)
	if err != nil {
		return "", "", err
	}
	return guarded, note, nil
}

func (r *Resource) remember(ctx context.Context, entity Datasource, question, sqlText, status string, rows int64, started time.Time, prompt, completion int64, message string) {
	record := usageFrom(entity, question, sqlText, status, rows, started, prompt, completion, 0, message)
	_, _ = r.repo.SaveUsage(ctx, record)
}

func usageFrom(entity Datasource, question, sqlText, status string, rows int64, started time.Time, prompt, completion, cost int64, message string) *UsageRecord {
	return &UsageRecord{
		DatasourceID:     entity.ID,
		DatasourceName:   entity.Name,
		Question:         question,
		SQLText:          sqlText,
		Status:           status,
		RowCount:         rows,
		DurationMs:       time.Since(started).Milliseconds(),
		PromptTokens:     prompt,
		CompletionTokens: completion,
		Cost:             cost,
		ErrorMessage:     clipText(message),
		CreatedAt:        time.Now().UTC(),
	}
}

func schemaPrompt(columns []Column) string {
	var b strings.Builder
	last := ""
	tables := 0
	first := true
	for _, column := range columns {
		key := column.Table
		if column.Schema != "" {
			key = column.Schema + "." + column.Table
		}
		if key != last {
			if tables == 80 || b.Len() > 12000 {
				break
			}
			if last != "" {
				b.WriteString(")\n")
			}
			b.WriteString(key)
			b.WriteString("(")
			last = key
			tables++
			first = true
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(column.Name)
		b.WriteByte(' ')
		b.WriteString(column.DataType)
	}
	if last != "" {
		b.WriteString(")")
	}
	return b.String()
}

func detectChart(columns []string, rows [][]any) string {
	if len(rows) == 0 || len(columns) == 0 {
		return "table"
	}
	if len(rows) == 1 && (len(columns) == 1 || (len(columns) == 2 && isNumberCell(rows[0][1]))) {
		return "metric"
	}
	if len(columns) >= 2 && len(rows) >= 2 && isNumberCell(rows[0][len(columns)-1]) {
		if looksDateCell(rows[0][0]) {
			return "line"
		}
		return "bar"
	}
	return "table"
}

func isNumberCell(value any) bool {
	switch typed := value.(type) {
	case int, int32, int64, float32, float64:
		return true
	case string:
		if typed == "" {
			return false
		}
		_, err := strconv.ParseFloat(typed, 64)
		return err == nil
	default:
		return false
	}
}

func looksDateCell(value any) bool {
	text := strings.TrimSpace(fmt.Sprint(value))
	if len(text) < 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", text[:10])
	return err == nil
}

func estimateTokens(text string) int64 {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	return int64(n+1) / 2
}
