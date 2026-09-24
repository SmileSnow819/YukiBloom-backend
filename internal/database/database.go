package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("数据库连接地址 DATABASE_URL 无效")
	}
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New("无法创建数据库连接池")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("无法连接数据库")
	}
	return pool, nil
}
