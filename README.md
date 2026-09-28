# YukiBloom 后端

YukiBloom 的独立 Go 服务，使用 Go、Gin 和 PostgreSQL。当前已提供健康检查、数据库迁移、管理员登录、文章与图片管理、足迹与实习经历管理、站点内容与独立页面管理，以及旧内容导入命令。整体方案见[设计文档](docs/superpowers/specs/2026-09-24-go-content-backend-design.md)。Astro 按需渲染、线上管理界面和腾讯云部署仍待接入。

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

登录接口为 `POST /api/v1/admin/login`，请求 JSON 包含 `username` 和 `password`。成功后服务设置 `HttpOnly` 会话 Cookie，并在 `data.csrfToken` 返回后续写请求使用的令牌。后台会话查询为 `GET /api/v1/admin/session`；退出为 `POST /api/v1/admin/logout`，写请求需在 `X-CSRF-Token` 请求头传入 CSRF 令牌。连续登录失败会触发限流。

所有 JSON 接口统一返回 `{ "code": 0, "message": "成功", "data": ... }`。失败时 `message` 是后端返回的中文说明，`data` 为 `null`；`code` 使用稳定的业务错误码，HTTP 状态码继续表达请求状态：

| 业务码 | 含义 | HTTP 状态 |
| --- | --- | --- |
| `10001` | 参数或请求内容错误 | 400 |
| `10002` | 未登录或会话无效 | 401 |
| `10003` | 无权限或 CSRF 校验失败 | 403 |
| `10004` | 资源不存在 | 404 |
| `10005` | 内容版本冲突或标识重复 | 409 |
| `10006` | 请求内容过大 | 413 |
| `10007` | 请求过频 | 429 |
| `10008` | 不支持的请求方法 | 405 |
| `50000` | 服务内部错误 | 500 |

前端可按 `code` 判断错误类别，并直接展示 `message`，无需硬编码错误文案。图片文件成功读取时返回原始二进制内容。

## 接口文档与 Apifox

接口说明写在 Gin 处理器的注释中，使用 [Swaggo](https://github.com/swaggo/swag) 生成 Swagger 2.0 文档。生成器是本地开发工具，不会作为 API 服务的运行依赖。首次使用时安装并在仓库根目录生成 JSON 和 YAML：

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.6
swag init --dir cmd/server,internal/apiresponse,internal/auth,internal/database,internal/httpserver,internal/importer,internal/media,internal/personal,internal/posts,internal/sitecontent --generalInfo main.go --parseInternal --output api-docs --outputTypes json,yaml
```

在 Apifox 项目中选择“导入数据”，导入 `api-docs/swagger.json`。设置环境前置 URL 为 `http://127.0.0.1:8080`。先调用管理员登录接口；Apifox 会保存响应中的 `yb_session` Cookie，后续请求会自动携带。后台写请求还要把登录响应的 `csrfToken` 填入 `X-CSRF-Token` 请求头。[Apifox Cookie 说明](https://docs.apifox.com/create-and-send-cookie)

接口或数据结构变化后重新运行生成命令，再将更新的 JSON 导入 Apifox。生成文件保存在 `api-docs/swagger.json` 和 `api-docs/swagger.yaml`，可提交到仓库供维护者和 Apifox 共用。

## 文章接口

公开接口 `GET /api/v1/posts` 支持 `locale`、`category`、`tag`、`q`、`page` 和 `limit` 参数；单篇文章通过 `GET /api/v1/posts/{slug}?locale=zh-CN` 获取。公开接口只返回已发布文章。后台需要先登录，支持 `GET/POST /api/v1/admin/posts`、`GET/PATCH/DELETE /api/v1/admin/posts/{id}`、`POST /api/v1/admin/posts/{id}/publish` 和 `.../unpublish`。修改文章时需把读取到的 `version` 一并提交，避免覆盖较新的编辑。

后台图片库支持 `GET/POST /api/v1/admin/media` 和 `DELETE /api/v1/admin/media/{id}`。只接受通过真实格式校验的 JPEG、PNG 和 WebP 图片，单张不超过 10 MiB，图片地址为 `/uploads/{生成的文件名}`。仍被文章正文或封面引用的图片不能删除。

后台 Markdown 导入支持 `POST /api/v1/admin/posts/markdown/preview` 预览，以及 `POST /api/v1/admin/posts/markdown` 保存为草稿；两者使用 multipart 字段 `file`，可选字段 `locale` 默认 `zh-CN`，文件上限为 2 MiB。预览会返回原文件的 `coverPath`。若文件声明了封面，先通过图片接口上传，再在保存请求中传 `coverMediaId`，避免导入时无声丢失封面。旧文章迁移工具默认为只读预检查；检查通过后，使用 `DATABASE_URL=... go run ./cmd/import-posts -apply` 执行导入。整批文章与封面元数据在同一数据库事务中提交，失败时回滚并清理本次写入的图片文件。重复运行会跳过已有文章，不覆盖后台编辑内容。

个人内容接口提供 `GET /api/v1/footprints` 和 `GET /api/v1/timeline`，响应分别包含 `version` 与内容列表。管理员通过对应的 `PUT /api/v1/admin/footprints`、`PUT /api/v1/admin/timeline` 整体保存地点、停留、路线与实习经历；请求必须提交读取时的 `version`，旧版本返回 409，其他写入失败会回滚。

迁移旧足迹与实习经历时，`go run ./cmd/import-personal` 默认只检查 YAML 和图片。检查通过后添加 `-apply` 写入；若数据库已有足迹或实习经历，需明确加 `-replace` 才会替换现有数据。迁移会把路线图片复制到后端图片目录并更新图片地址。

站点内容公开接口为 `GET /api/v1/site-content`，后台通过 `GET/PUT /api/v1/admin/site-content` 管理站点资料、社交链接、分类映射、首页精选、导航、公告、友链、音乐列表和内容翻译。整体保存时需提交读取到的 `version`，旧版本会返回冲突提示。“关于我”等独立页面通过 `GET /api/v1/pages/{slug}?locale=zh-CN` 公开读取，后台支持页面列表、草稿保存、发布、撤回和删除。技术开关、主题设置、评论与统计服务配置继续保留在代码和环境变量中。

旧站点资料迁移使用 `go run ./cmd/import-site`，默认只读检查 `config/site.yaml`、`config/i18n-content.yaml`、“关于我”和歌单页面。预检查通过后添加 `-apply` 才会写入数据库；已有站点内容时还需显式添加 `-replace`。导入工具会把头像与精选入口图片复制到后端图片目录。Docker 镜像也包含 `/import-posts`、`/import-personal`、`/import-site`，可挂载旧前端仓库后在容器内运行。

## 本地接口验证

请使用独立的测试数据库和图片目录，不要把测试数据写入正式数据库。准备测试数据库后，在一个终端创建临时管理员并启动服务：

```bash
createdb yukibloom_test
export DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom_test?sslmode=disable'
ADMIN_USERNAME='api-test' ADMIN_PASSWORD='请换成至少 12 位的临时密码' go run ./cmd/create-admin
COOKIE_SECURE=false UPLOAD_DIR=/tmp/yukibloom-api-test-uploads go run ./cmd/server
```

在另一个终端检查服务和管理员登录：

```bash
curl -i http://127.0.0.1:8080/api/v1/health
curl -i -c /tmp/yukibloom-api-test-cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username":"api-test","password":"你的临时密码"}' \
  http://127.0.0.1:8080/api/v1/admin/login
```

登录成功会在 `data.csrfToken` 返回 CSRF 令牌并写入 Cookie。后台的新增、修改、发布、撤回、上传和删除请求需要携带 Cookie，并在请求头中传入 `X-CSRF-Token: 登录响应中的 data.csrfToken`。例如：

```bash
curl -i -b /tmp/yukibloom-api-test-cookies.txt \
  -H 'X-CSRF-Token: 登录响应中的 data.csrfToken' \
  http://127.0.0.1:8080/api/v1/admin/posts
```

**2026-09-24 本地接口检查：44 项全部通过。**检查使用临时 PostgreSQL 数据库与本机服务，覆盖了健康检查和公开读取、登录保护、文章草稿/编辑/发布/撤回/搜索、图片上传/读取/引用保护、Markdown 预览与导入、足迹和实习经历保存、站点内容版本冲突，以及独立页面的创建/编辑/发布/撤回/删除。检查后已删除临时数据库和图片文件。

其中几项关键预期结果：草稿文章和未发布页面公开读取返回 `404`；发布后公开读取返回 `200`；撤回后再次返回 `404`；图片仍被文章引用时删除返回 `409`；使用过期的站点内容版本保存返回 `409`；未登录访问后台返回 `401`。

## 项目结构图

目录树和逐文件用途见[项目结构图](docs/project-structure.md)。文件用途由 `docs/project-files.json` 维护；新增、删除或移动文件后更新清单，再运行下面的命令即可重生成图表：

```bash
go run ./cmd/project-map
```

生成命令会对照实际目录检查清单。项目文件没有用途说明，或清单仍引用已删除文件时，命令会报告对应路径并停止生成。

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
