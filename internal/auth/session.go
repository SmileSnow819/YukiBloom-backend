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

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

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

func (s *Store) FindAdmin(ctx context.Context, username string) (string, string, error) {
	var id, passwordHash string
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM admins WHERE username = $1`, username).Scan(&id, &passwordHash)
	return id, passwordHash, err
}

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

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	tokenHash := sha256.Sum256([]byte(token))
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash[:])
	return err
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
