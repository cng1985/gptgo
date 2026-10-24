package chatbi

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

type QueryResult struct {
	Columns   []string
	Rows      [][]any
	Truncated bool
}

type QueryEngine interface {
	Ping(ctx context.Context, info ConnInfo) error
	Schema(ctx context.Context, info ConnInfo) ([]Column, error)
	Query(ctx context.Context, info ConnInfo, query string, maxRows int) (QueryResult, error)
}

type ConnInfo struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

type SQLEngine struct{}

func NewSQLEngine() *SQLEngine { return &SQLEngine{} }

func (e *SQLEngine) Ping(ctx context.Context, info ConnInfo) error {
	db, err := openDB(info)
	if err != nil {
		return err
	}
	defer db.Close()
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return db.PingContext(cctx)
}

func (e *SQLEngine) Schema(ctx context.Context, info ConnInfo) ([]Column, error) {
	db, err := openDB(info)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := db.Conn(cctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	setReadOnly(cctx, info.Driver, conn)
	rows, err := conn.QueryContext(cctx, schemaSQL(info), schemaArgs(info)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Column, 0, 64)
	for rows.Next() {
		var schemaName, tableName, columnName, dataType sql.NullString
		switch info.Driver {
		case "mysql":
			if err := rows.Scan(&tableName, &columnName, &dataType); err != nil {
				return nil, err
			}
			schemaName = sql.NullString{String: info.Database, Valid: true}
		default:
			if err := rows.Scan(&schemaName, &tableName, &columnName, &dataType); err != nil {
				return nil, err
			}
		}
		out = append(out, Column{
			Schema:   strings.TrimSpace(schemaName.String),
			Table:    strings.TrimSpace(tableName.String),
			Name:     strings.TrimSpace(columnName.String),
			DataType: strings.TrimSpace(dataType.String),
		})
	}
	return out, rows.Err()
}

func (e *SQLEngine) Query(ctx context.Context, info ConnInfo, query string, maxRows int) (QueryResult, error) {
	var result QueryResult
	if maxRows < 1 {
		maxRows = MaxRows
	}
	db, err := openDB(info)
	if err != nil {
		return result, err
	}
	defer db.Close()
	cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	conn, err := db.Conn(cctx)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	setReadOnly(cctx, info.Driver, conn)
	rows, err := conn.QueryContext(cctx, query)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return result, err
	}
	result.Columns = names
	result.Rows = make([][]any, 0)
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}
		dest := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return result, err
		}
		line := make([]any, len(dest))
		for i, value := range dest {
			line[i] = normalizeCell(value)
		}
		result.Rows = append(result.Rows, line)
	}
	return result, rows.Err()
}

func openDB(info ConnInfo) (*sql.DB, error) {
	dsn, err := buildDSN(info)
	if err != nil {
		return nil, err
	}
	driver, err := sqlDriver(info.Driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

func sqlDriver(driver string) (string, error) {
	switch driver {
	case "mysql":
		return "mysql", nil
	case "postgres":
		return "pgx", nil
	case "sqlserver":
		return "sqlserver", nil
	default:
		return "", fmt.Errorf("不支持的数据源")
	}
}

func buildDSN(info ConnInfo) (string, error) {
	switch info.Driver {
	case "mysql":
		cfg := mysqldriver.NewConfig()
		cfg.User = info.Username
		cfg.Passwd = info.Password
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
		cfg.DBName = info.Database
		cfg.ParseTime = true
		cfg.Timeout = 8 * time.Second
		cfg.ReadTimeout = 12 * time.Second
		cfg.Params = map[string]string{"charset": "utf8mb4"}
		if info.SSLMode == "require" {
			cfg.TLSConfig = "true"
		}
		return cfg.FormatDSN(), nil
	case "postgres":
		ssl := info.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(info.Username, info.Password),
			Host:   net.JoinHostPort(info.Host, strconv.Itoa(info.Port)),
			Path:   "/" + info.Database,
		}
		q := u.Query()
		q.Set("sslmode", ssl)
		q.Set("connect_timeout", "8")
		u.RawQuery = q.Encode()
		return u.String(), nil
	case "sqlserver":
		q := url.Values{}
		q.Set("database", info.Database)
		q.Set("connection timeout", "8")
		if info.SSLMode == "require" {
			q.Set("encrypt", "true")
			q.Set("TrustServerCertificate", "true")
		} else {
			q.Set("encrypt", "disable")
		}
		u := &url.URL{
			Scheme:   "sqlserver",
			User:     url.UserPassword(info.Username, info.Password),
			Host:     net.JoinHostPort(info.Host, strconv.Itoa(info.Port)),
			RawQuery: q.Encode(),
		}
		return u.String(), nil
	default:
		return "", fmt.Errorf("不支持的数据源")
	}
}

func schemaSQL(info ConnInfo) string {
	switch info.Driver {
	case "mysql":
		return `SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, ORDINAL_POSITION
LIMIT 400`
	case "postgres":
		return `SELECT table_schema, table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name, ordinal_position
LIMIT 400`
	default:
		return `SELECT TOP 400 TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA NOT IN ('sys', 'INFORMATION_SCHEMA')
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`
	}
}

func schemaArgs(info ConnInfo) []any {
	if info.Driver == "mysql" {
		return []any{info.Database}
	}
	return nil
}

func setReadOnly(ctx context.Context, driver string, conn *sql.Conn) {
	switch driver {
	case "mysql":
		_, _ = conn.ExecContext(ctx, "SET SESSION transaction_read_only = 1")
	case "postgres":
		_, _ = conn.ExecContext(ctx, "SET default_transaction_read_only = on")
	}
}

func normalizeCell(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return clipText(string(typed))
	case string:
		return clipText(typed)
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	case int64, int32, int, float64, float32, bool:
		return typed
	default:
		return clipText(fmt.Sprint(typed))
	}
}

func clipText(text string) string {
	runes := []rune(text)
	if len(runes) <= 500 {
		return text
	}
	return string(runes[:500]) + "…"
}
