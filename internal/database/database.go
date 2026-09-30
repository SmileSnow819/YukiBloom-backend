package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open 建立并验证 PostgreSQL 连接池。
// 参数：ctx 控制连接操作；databaseURL 是数据库连接地址。
// 返回：可用的连接池；地址或连接无效时返回错误。
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
