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

// main 执行管理员创建命令，并在失败时输出原因后退出。
// 参数：无。
// 返回：无。
func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout); err != nil {
		log.Printf("创建管理员失败：%v", err)
		os.Exit(1)
	}
}

// run 校验管理员凭据、连接数据库并创建管理员账号。
// 参数：ctx 控制启动过程的取消；getenv 读取环境变量；output 接收成功提示。
// 返回：error；配置、数据库、迁移、写入或输出失败时返回错误。
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
