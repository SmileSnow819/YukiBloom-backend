package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnauthenticated = errors.New("请先登录")
	ErrCSRF            = errors.New("请求校验失败，请刷新页面后重试")
)

type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建管理员及会话存储。
// 参数：pool 是数据库连接池。
// 返回：使用该连接池的 Store。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateAdmin 创建管理员并保存密码哈希。
// 参数：s 是管理员及会话存储；ctx 控制数据库操作；username 是管理员用户名；password 是明文密码。
// 返回：新管理员 ID；哈希或写入失败时返回错误。
func (s *Store) CreateAdmin(ctx context.Context, username, password string) (string, error) {
	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	var id string
	if err := s.pool.QueryRow(ctx, `INSERT INTO admins (username, password_hash) VALUES ($1, $2) RETURNING id`, username, passwordHash).Scan(&id); err != nil {
		return "", fmt.Errorf("创建管理员失败：%w", err)
	}
	return id, nil
}

// FindAdmin 按用户名查询管理员凭据。
// 参数：s 是管理员及会话存储；ctx 控制数据库操作；username 是待查询的管理员用户名。
// 返回：管理员 ID、密码哈希，以及查询错误。
func (s *Store) FindAdmin(ctx context.Context, username string) (string, string, error) {
	var id, passwordHash string
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM admins WHERE username = $1`, username).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

// CreateSession 为管理员建立限时登录会话。
// 参数：s 是管理员及会话存储；ctx 控制数据库操作；adminID 是管理员 ID。
// 返回：会话令牌、CSRF 令牌，以及生成或保存失败时的错误。
func (s *Store) CreateSession(ctx context.Context, adminID string) (string, string, error) {
	token, err := randomToken()
	if err != nil {
		return "", "", err
	}
	csrf, err := randomToken()
	if err != nil {
		return "", "", err
	}
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrf))
	_, err = s.pool.Exec(ctx, `INSERT INTO sessions (token_hash, admin_id, csrf_hash, expires_at) VALUES ($1, $2, $3, $4)`, tokenHash[:], adminID, csrfHash[:], time.Now().Add(7*24*time.Hour))
	if err != nil {
		return "", "", fmt.Errorf("创建登录会话失败：%w", err)
	}
	return token, csrf, nil
}

// Authenticate 校验会话令牌，并按需验证 CSRF 令牌。
// 参数：s 是管理员及会话存储；ctx 控制数据库操作；token 是会话令牌；csrf 是请求中的 CSRF 令牌；mutation 表示是否为写操作。
// 返回：已认证的管理员 ID；令牌无效或查询失败时返回错误。
func (s *Store) Authenticate(ctx context.Context, token, csrf string, mutation bool) (string, error) {
	if token == "" {
		return "", ErrUnauthenticated
	}
	tokenHash := sha256.Sum256([]byte(token))
	var adminID string
	var storedCSRF []byte
	err := s.pool.QueryRow(ctx, `SELECT admin_id, csrf_hash FROM sessions WHERE token_hash = $1 AND expires_at > now()`, tokenHash[:]).Scan(&adminID, &storedCSRF)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", fmt.Errorf("查询登录会话失败：%w", err)
	}
	if mutation {
		csrfHash := sha256.Sum256([]byte(csrf))
		if subtle.ConstantTimeCompare(storedCSRF, csrfHash[:]) != 1 {
			return "", ErrCSRF
		}
	}
	return adminID, nil
}

// DeleteSession 删除指定令牌对应的登录会话。
// 参数：s 是管理员及会话存储；ctx 控制数据库操作；token 是待删除的会话令牌。
// 返回：数据库操作错误；空令牌直接返回 nil。
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	tokenHash := sha256.Sum256([]byte(token))
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash[:])
	return err
}

// randomToken 生成安全的 URL 可用随机令牌。
// 参数：无。
// 返回：随机令牌；随机数读取失败时返回错误。
func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
