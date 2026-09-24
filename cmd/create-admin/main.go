package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
)

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout); err != nil {
		log.Printf("创建管理员失败：%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, output io.Writer) error {
	username := strings.TrimSpace(getenv("ADMIN_USERNAME"))
	password := getenv("ADMIN_PASSWORD")
	if username == "" || password == "" {
		return errors.New("请设置 ADMIN_USERNAME 和 ADMIN_PASSWORD 环境变量")
	}
	if utf8.RuneCountInString(password) < 12 {
		return errors.New("管理员密码至少需要 12 个字符")
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}
	defer pool.Close()
	if err := database.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("数据库迁移失败：%w", err)
	}
	if _, err := auth.NewStore(pool).CreateAdmin(startupCtx, username, password); err != nil {
		return errors.New("创建管理员失败，请确认用户名尚未使用")
	}
	_, err = fmt.Fprintf(output, "管理员 %s 已创建\n", username)
	return err
}
