package chatbi

import (
	"strings"
	"testing"
)

func TestGuardReadOnlySQL(t *testing.T) {
	got, err := Guard("SELECT * FROM orders", "mysql", 200)
	if err != nil || !strings.Contains(got, "LIMIT 200") {
		t.Fatalf("mysql limit: %s %v", got, err)
	}
	got, err = Guard("select distinct city from orders", "sqlserver", 50)
	if err != nil || !strings.Contains(got, "TOP (50)") {
		t.Fatalf("sqlserver top: %s %v", got, err)
	}
	got, err = Guard("WITH c AS (SELECT 1 AS n) SELECT n FROM c", "postgres", 20)
	if err != nil || !strings.HasPrefix(strings.ToLower(got), "with") || !strings.Contains(got, "LIMIT 20") {
		t.Fatalf("with: %s %v", got, err)
	}
	for _, sql := range []string{
		"DROP TABLE orders",
		"SELECT * FROM orders; DELETE FROM orders",
		"SELECT * FROM orders INTO OUTFILE '/tmp/a'",
		"UPDATE orders SET city = 'x'",
		"SELECT * FROM orders FOR UPDATE",
		"SELECT /* DROP */ 1; DROP TABLE orders",
		"SELECT SLEEP(2)",
	} {
		if _, err := Guard(sql, "mysql", 10); err == nil {
			t.Fatalf("应该拒绝: %s", sql)
		}
	}
	if extractSQL("```sql\nSELECT 1\n```") != "SELECT 1" {
		t.Fatal("没有抽出 SQL")
	}
}

func TestPlanUsesSchema(t *testing.T) {
	columns := []Column{
		{Table: "orders", Name: "id", DataType: "int"},
		{Table: "orders", Name: "city", DataType: "varchar"},
		{Table: "orders", Name: "amount", DataType: "decimal"},
		{Table: "orders", Name: "created_at", DataType: "datetime"},
		{Table: "users", Name: "id", DataType: "int"},
		{Table: "users", Name: "name", DataType: "varchar"},
	}
	sql, _, err := Plan("orders 有多少条", "mysql", "app", columns)
	if err != nil || !strings.Contains(sql, "COUNT(*)") || !strings.Contains(sql, "`orders`") {
		t.Fatalf("count: %s %v", sql, err)
	}
	sql, _, err = Plan("orders 每个 city 的销售额", "mysql", "app", columns)
	if err != nil || !strings.Contains(sql, "SUM(") || !strings.Contains(sql, "`city`") || !strings.Contains(sql, "GROUP BY") {
		t.Fatalf("sum: %s %v", sql, err)
	}
	sql, _, err = Plan("列出 orders", "postgres", "app", columns)
	if err != nil || !strings.Contains(sql, `"orders"`) || !strings.Contains(sql, `"city"`) {
		t.Fatalf("list: %s %v", sql, err)
	}
	sql, _, err = Plan("每天的订单趋势", "sqlserver", "app", []Column{
		{Schema: "dbo", Table: "orders", Name: "id", DataType: "int"},
		{Schema: "dbo", Table: "orders", Name: "created_at", DataType: "datetime"},
	})
	if err != nil || !strings.Contains(sql, "[dbo].[orders]") || !strings.Contains(sql, "CAST(") {
		t.Fatalf("series: %s %v", sql, err)
	}
	if _, _, err := Plan("今天天气怎么样", "mysql", "app", columns); err == nil {
		t.Fatal("多表且对不上时应拒绝")
	}
	driver, err := CanonicalDriver("pgsql")
	if err != nil || driver != "postgres" {
		t.Fatalf("driver: %s %v", driver, err)
	}
}

func TestDSNKeepsPasswordOutOfShape(t *testing.T) {
	mysqlDSN, err := buildDSN(ConnInfo{Driver: "mysql", Host: "127.0.0.1", Port: 3306, Database: "app", Username: "reader", Password: "p@ss/word"})
	if err != nil || !strings.Contains(mysqlDSN, "tcp(127.0.0.1:3306)") || !strings.Contains(mysqlDSN, "charset=utf8mb4") {
		t.Fatalf("mysql dsn: %s %v", mysqlDSN, err)
	}
	pgDSN, err := buildDSN(ConnInfo{Driver: "postgres", Host: "db.internal", Port: 5432, Database: "app", Username: "reader", Password: "secret", SSLMode: "disable"})
	if err != nil || !strings.Contains(pgDSN, "sslmode=disable") || !strings.Contains(pgDSN, "db.internal:5432") {
		t.Fatalf("postgres dsn: %s %v", pgDSN, err)
	}
	msDSN, err := buildDSN(ConnInfo{Driver: "sqlserver", Host: "10.0.0.8", Port: 1433, Database: "app", Username: "reader", Password: "secret"})
	if err != nil || !strings.Contains(msDSN, "encrypt=disable") || !strings.Contains(msDSN, "10.0.0.8:1433") {
		t.Fatalf("sqlserver dsn: %s %v", msDSN, err)
	}
}

func TestCipherRoundTrip(t *testing.T) {
	cipher, err := OpenCipher(t.TempDir() + "/datasource.key")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := cipher.Encrypt("p@ss/word")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "p@ss/word") {
		t.Fatal("密文不应包含原文")
	}
	plain, err := cipher.Decrypt(encoded)
	if err != nil || plain != "p@ss/word" {
		t.Fatalf("解密失败: %s %v", plain, err)
	}
}
