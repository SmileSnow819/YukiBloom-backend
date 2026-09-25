// @title YukiBloom 后端接口
// @version 1.0
// @description YukiBloom 网站的文章、图片、个人内容和站点页面 API。JSON 响应统一使用 code、message、data；成功 code 为 0。错误码 10001、10002、10003、10004、10005、10006、10007、10008、50000 分别表示参数错误、未登录、无权限、资源不存在、内容冲突、请求过大、请求过频、不支持请求方法和服务错误。
// @BasePath /
// @schemes http https
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/SmileSnow819/YukiBloom-backend/internal/httpserver"
	"github.com/SmileSnow819/YukiBloom-backend/internal/importer"
	"github.com/SmileSnow819/YukiBloom-backend/internal/media"
	"github.com/SmileSnow819/YukiBloom-backend/internal/personal"
	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	"github.com/SmileSnow819/YukiBloom-backend/internal/sitecontent"
	"github.com/gin-gonic/gin"
)

// main 启动 API 服务，并在收到系统信号时触发平滑关闭。
// 参数：无。
// 返回：无。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		log.Printf("服务停止：%v", err)
		os.Exit(1)
	}
}

// run 加载配置、初始化数据库和路由，并运行 HTTP 服务直到退出。
// 参数：ctx 接收系统关闭信号；getenv 读取服务环境变量。
// 返回：error；配置、数据库迁移、监听或平滑关闭失败时返回错误。
func run(ctx context.Context, getenv func(string) string) error {
	gin.SetMode(gin.ReleaseMode)
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}

	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库启动失败：%w", err)
	}
	defer pool.Close()
	if err := database.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("数据库迁移失败：%w", err)
	}

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpserver.NewRouter(
			auth.NewHandler(auth.NewStore(pool), cfg.CookieSecure),
			posts.NewHandler(posts.NewStore(pool)),
			media.NewHandler(media.NewStore(pool, cfg.UploadDir)),
			importer.NewHandler(posts.NewStore(pool)),
			personal.NewHandler(personal.NewStore(pool)),
			sitecontent.NewHandler(sitecontent.NewStore(pool)),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.ListenAndServe() }()
	log.Printf("API 已在端口 %s 启动", cfg.Port)

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP 服务失败：%w", err)
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP 服务关闭失败：%w", err)
		}
		return nil
	}
}
