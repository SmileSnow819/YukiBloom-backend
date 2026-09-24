# YukiBloom 后端

YukiBloom 的独立 Go 服务，使用 Go、Gin 和 PostgreSQL。当前已提供健康检查、数据库迁移、管理员登录与会话接口；下一步实现文章管理和图片上传。整体方案见[设计文档](docs/superpowers/specs/2026-09-24-go-content-backend-design.md)。

## 本地运行

需要 Go 1.27 和 PostgreSQL 17。可使用 Docker Compose：

```bash
cp .env.example .env
# 修改 .env 中的 POSTGRES_PASSWORD，使用随机生成的长密码
# 该密码会用于数据库连接地址，建议使用随机十六进制字符串
docker compose up --build -d
curl http://127.0.0.1:8080/api/v1/health
```

健康接口正常时返回 `{"status":"ok"}`。API 只绑定主机的 `127.0.0.1`，数据库不映射主机端口。与 Astro 一起部署时，由统一的反向代理暴露网站入口。

已有 PostgreSQL 时，也可以直接运行：

```bash
export DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom?sslmode=disable'
go run ./cmd/server
```

可选配置项：`PORT` 默认 `8080`；`COOKIE_SECURE` 默认 `true`，正式环境应启用 HTTPS 并保持 `true`。如果本地直接通过 HTTP 调试登录，可在 `.env` 中临时设为 `false`。服务启动时会连接数据库并运行尚未执行的迁移文件；迁移失败时服务不会开始监听。

## 管理员登录

首次启动后，通过环境变量创建管理员。用户名和密码不会写入仓库；密码至少 12 个字符：

```bash
docker compose exec \
  -e ADMIN_USERNAME='你的用户名' \
  -e ADMIN_PASSWORD='至少十二位的随机密码' \
  api /create-admin
```

登录接口为 `POST /api/v1/admin/login`，请求 JSON 包含 `username` 和 `password`。成功后服务设置 `HttpOnly` 会话 Cookie，并返回后续写请求使用的 `csrfToken`。后台会话查询为 `GET /api/v1/admin/session`；退出为 `POST /api/v1/admin/logout`，写请求需在 `X-CSRF-Token` 请求头传入 CSRF 令牌。连续登录失败会触发限流。

## 测试

```bash
go test ./...
go vet ./...
```

数据库集成测试需要单独的测试库：

```bash
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom_test?sslmode=disable' go test ./internal/database -v
```

不设置 `TEST_DATABASE_URL` 时，数据库集成测试会跳过；其余测试仍会运行。
