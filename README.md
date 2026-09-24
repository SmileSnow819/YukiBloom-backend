# YukiBloom 后端

YukiBloom 的独立 Go 服务，使用 Go、Gin 和 PostgreSQL。当前已提供健康检查、数据库迁移、管理员登录与会话接口，以及文章管理和公开查询接口。整体方案见[设计文档](docs/superpowers/specs/2026-09-24-go-content-backend-design.md)。

## 本地运行

需要 Go 1.27 和 PostgreSQL 17。可使用 Docker Compose：

```bash
cp .env.example .env
# 修改 .env 中的 POSTGRES_PASSWORD，使用随机生成的长密码
# 该密码会用于数据库连接地址，建议使用随机十六进制字符串
docker compose up --build -d
curl http://127.0.0.1:8080/api/v1/health
```

健康接口正常时返回 `{"status":"正常"}`。API 只绑定主机的 `127.0.0.1`，数据库不映射主机端口。数据库和图片分别保存在 Compose 持久化卷中。与 Astro 一起部署时，由统一的反向代理暴露网站入口。

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

## 文章接口

公开接口 `GET /api/v1/posts` 支持 `locale`、`category`、`tag`、`q`、`page` 和 `limit` 参数；单篇文章通过 `GET /api/v1/posts/{slug}?locale=zh-CN` 获取。公开接口只返回已发布文章。后台需要先登录，支持 `GET/POST /api/v1/admin/posts`、`GET/PATCH /api/v1/admin/posts/{id}`、`POST /api/v1/admin/posts/{id}/publish` 和 `.../unpublish`。修改文章时需把读取到的 `version` 一并提交，避免覆盖较新的编辑。

后台图片库支持 `GET/POST /api/v1/admin/media` 和 `DELETE /api/v1/admin/media/{id}`。只接受通过真实格式校验的 JPEG、PNG 和 WebP 图片，单张不超过 10 MiB，图片地址为 `/uploads/{生成的文件名}`。仍被文章正文或封面引用的图片不能删除。

后台 Markdown 导入支持 `POST /api/v1/admin/posts/markdown/preview` 预览，以及 `POST /api/v1/admin/posts/markdown` 保存为草稿；两者使用 multipart 字段 `file`，可选字段 `locale` 默认 `zh-CN`，文件上限为 2 MiB。旧文章迁移工具默认为只读预检查；检查通过后，使用 `DATABASE_URL=... go run ./cmd/import-posts -apply` 执行导入。重复运行会跳过已有文章，不覆盖后台编辑内容。

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
