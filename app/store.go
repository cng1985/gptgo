package app

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	defaultBcryptCost = 12
	sessionTTL        = 7 * 24 * time.Hour
)

var (
	ErrEmailTaken         = errors.New("email taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotFound           = errors.New("not found")
)

// User 是登录后可以展示的账号信息，不包含密码哈希。
type User struct {
	ID        int64
	Name      string
	Email     string
	CreatedAt time.Time
}

// Store 把账号和会话放在本地 SQLite 里。
type Store struct {
	db       *sql.DB
	cost     int
	dummy    []byte
	dummyErr error
	once     sync.Once
}

// Open 创建数据库文件（如果还没有）并建表。
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, cost: defaultBcryptCost}, nil
}

func sqliteDSN(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	u := url.URL{
		Scheme:   "file",
		Path:     abs,
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
	}
	return u.String()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL
);
`)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateUser(name, email, password string) (User, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)
	if name == "" || email == "" || password == "" {
		return User{}, fmt.Errorf("missing user fields")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO users (name, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		name, email, string(hash), now.Format(time.RFC3339Nano),
	)
	if isUnique(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Name: name, Email: email, CreatedAt: now}, nil
}

func (s *Store) Authenticate(email, password string) (User, error) {
	email = normalizeEmail(email)
	var user User
	var hash, created string
	err := s.db.QueryRow(
		`SELECT id, name, email, password_hash, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Name, &user.Email, &hash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		dummy, derr := s.dummyHash()
		if derr != nil {
			return User{}, derr
		}
		_ = bcrypt.CompareHashAndPassword(dummy, []byte(password))
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	user.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateSession(userID int64) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(sessionTTL)
	_, err = s.db.Exec(
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), userID, expires.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Store) UserByToken(token string) (User, error) {
	if !validToken(token) {
		return User{}, ErrNotFound
	}
	key := hashToken(token)
	var user User
	var created, expiresAt string
	err := s.db.QueryRow(`
SELECT u.id, u.name, u.email, u.created_at, s.expires_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ?`, key).Scan(&user.ID, &user.Name, &user.Email, &created, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return User{}, err
	}
	if !time.Now().Before(expires) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, key)
		return User{}, ErrNotFound
	}
	user.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) DeleteSession(token string) error {
	if !validToken(token) {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func (s *Store) dummyHash() ([]byte, error) {
	s.once.Do(func() {
		s.dummy, s.dummyErr = bcrypt.GenerateFromPassword([]byte("gptgo-dummy-password"), s.cost)
	})
	return s.dummy, s.dummyErr
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validToken(value string) bool {
	if len(value) < 40 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
