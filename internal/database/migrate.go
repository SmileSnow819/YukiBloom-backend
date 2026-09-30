package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrationNames 列出嵌入的 SQL 迁移文件并按名称排序。
// 参数：无。
// 返回：排序后的文件名列表；列举失败时返回错误。
func migrationNames() ([]string, error) {
	paths, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, strings.TrimPrefix(path, "migrations/"))
	}
	sort.Strings(names)
	return names, nil
}

// Migrate 创建迁移记录表并依次执行未应用的迁移。
// 参数：ctx 控制数据库操作；pool 是数据库连接池。
// 返回：迁移过程中的错误，成功时为 nil。
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("创建数据库迁移记录表失败：%w", err)
	}

	names, err := migrationNames()
	if err != nil {
		return fmt.Errorf("列出数据库迁移文件失败：%w", err)
	}
	for _, name := range names {
		contents, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("读取数据库迁移文件 %s 失败：%w", name, err)
		}
		if err := runMigration(ctx, pool, name, string(contents)); err != nil {
			return fmt.Errorf("执行数据库迁移 %s 失败：%w", name, err)
		}
	}
	return nil
}

// runMigration 在事务中检查并执行指定版本的迁移。
// 参数：ctx 控制数据库操作；pool 是数据库连接池；version 是迁移版本名；sql 是待执行的 SQL 内容。
// 返回：事务或 SQL 执行错误，已应用或成功时为 nil。
func runMigration(ctx context.Context, pool *pgxpool.Pool, version, sql string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(20260924)`); err != nil {
		return err
	}
	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return nil
	}
	if _, err := tx.Exec(ctx, sql, pgx.QueryExecModeSimpleProtocol); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
