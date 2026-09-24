# YukiBloom 后端

YukiBloom 的独立 Go 服务。当前分支先提供 Gin 健康接口、PostgreSQL 连接与版本化迁移，文章和后台接口会在后续任务中加入。整体内容方案见 [设计文档](docs/superpowers/specs/2026-09-24-go-content-backend-design.md)。

## 本地运行

需要 Go 1.27 和 PostgreSQL 17。可使用 Docker Compose：

```bash
cp .env.example .env
# 修改 .env 中的 POSTGRES_PASSWORD，使用随机生成的长密码
# 密码用于数据库 URL，若包含 @、:、/ 等字符，需先做 URL 编码
docker compose up --build -d
curl http://127.0.0.1:8080/api/v1/health
```

健康接口正常时返回 `{"status":"ok"}`。API 只绑定主机的 `127.0.0.1`，数据库不映射主机端口。日后与 Astro 一起部署时，由统一的反向代理暴露网站入口。

已有 PostgreSQL 时，也可以直接运行：

```bash
export DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom?sslmode=disable'
go run ./cmd/server
```

可选的 `PORT` 默认值是 `8080`。服务启动时会连接数据库并依次运行未执行的 `internal/database/migrations` SQL 文件；`schema_migrations` 记录执行过的版本。迁移失败时服务不会开始监听。生产环境部署前应先备份数据库。

## 测试

```bash
go test ./...
go vet ./...
```

数据库集成测试需要单独的测试库：

```bash
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom_test?sslmode=disable' go test ./internal/database -v
```

不设置 `TEST_DATABASE_URL` 时，集成测试会跳过；其余单元测试仍会运行。
