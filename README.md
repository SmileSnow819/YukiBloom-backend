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

健康接口正常时返回 `{"status":"正常"}`。API 只绑定主机的 `127.0.0.1`，数据库不映射主机端口。PostgreSQL 使用 Compose 持久化卷；图片统一保存在 COS。与 Astro 一起部署时，Compose 会把 API 同时接入 backend 私网和 `yukibloom-public` 网络；PostgreSQL 只留在 backend 私网，Astro 与统一反向代理通过 `api:8080` 访问 Go API。

### 自动构建与部署

`.github/workflows/deploy.yml` 在 PR 指向 `main` 时运行 `go test ./...` 和 `go vet ./...`；PR 合并到 `main` 后，检查通过才会构建并推送 GHCR 镜像，然后通过 SSH 在服务器更新 API。镜像同时使用 `main` 和 Git 提交 SHA 标签，实际部署使用 SHA 标签，便于确认版本和手动回滚。手动触发 workflow 时需要选择 `main` 分支。

服务器首次准备时，将仓库的 `compose.yaml` 放到 `/srv/YukiBloom-backend`，在该目录创建仅服务器可读的 `.env`，填入数据库密码和 COS 凭证，然后运行 `docker compose up -d db` 初始化 PostgreSQL。部署用户需要能执行 Docker 命令。GHCR 镜像如果设为私有，还要在服务器配置只读 GHCR 登录凭据；也可以在首次发布后将 Container package 设为公开。

在 GitHub 仓库的 **Settings → Secrets and variables → Actions** 设置以下 Secrets：

| Secret | 内容 |
| --- | --- |
| `SERVER_HOST` | 服务器 IP 或域名 |
| `SERVER_USER` | 允许 SSH 登录并执行 Docker 命令的部署用户 |
| `SERVER_SSH_KEY` | 对应部署用户公钥的 SSH 私钥 |
| `SERVER_FINGERPRINT` | 服务器 SSH 主机公钥的 SHA256 指纹 |

SSH 公钥需预先安装到服务器部署用户的 `authorized_keys`；主机指纹应通过可信的服务器控制台或已有安全连接核对。不要把私钥、数据库密码或 COS 密钥写进 workflow 文件。

自动部署只更新 `api` 容器，不会停止 PostgreSQL 或删除其持久化卷。服务启动时会执行数据库迁移。若需回滚，在服务器目录用已知的旧提交 SHA 执行：

```bash
IMAGE_TAG=<旧提交SHA> docker compose pull api
IMAGE_TAG=<旧提交SHA> docker compose up -d --no-build api
```

### 手动更新 Docker 中的代码

代码更新到本机或服务器后，在**后端仓库根目录**重新构建并启动 API 容器：

```bash
cd /path/to/YukiBloom-backend
git pull
docker compose up -d --build api
docker compose ps
curl http://127.0.0.1:8080/api/v1/health
```

`git pull` 只适用于用 Git 部署的工作副本；如果代码已通过其他方式更新，从 `docker compose` 命令开始即可。`--build` 会用当前源码重新构建镜像，`up -d` 会在需要时重建 API 容器；服务启动时会自动执行数据库迁移。无需先运行 `docker compose down`，以免同时停止数据库。

前端代码更新后，在**前端仓库 `YukiBloom` 的根目录**执行其 Compose 命令：

```bash
cd /path/to/YukiBloom
git pull
docker compose --env-file ./.env -f docker/docker-compose.yml up -d --build
docker compose --env-file ./.env -f docker/docker-compose.yml ps
```

前端首次启动前，先启动上面的后端 Compose 服务，以创建双方共用的 `yukibloom-public` 网络。前端的 `.env` 需按其仓库说明配置；这组命令会重建 Astro 与 Nginx 镜像。两套 Compose 分别在各自的仓库目录运行，不要对数据库卷执行删除操作。

## 图片统一存储至 COS

后台上传和内容导入的图片统一写入腾讯云 COS，不再保存到本地目录。创建通用存储桶，建议选择与 CVM 相同的地域和标准存储；目前配置的桶为 `yukibloom-1379189818`，地域为 `ap-shanghai`。桶内图片供博客访客读取时，设置为“公有读私有写”，禁止匿名写入。所有桶对象都可通过 URL 读取，因此只将公开内容图片放在这个桶中。

复制 `.env.example` 后保留或填写以下配置，并在服务器 `.env` 中填入 COS 服务端凭证：

```dotenv
COS_BUCKET=yukibloom-1379189818
COS_REGION=ap-shanghai
COS_PUBLIC_BASE_URL=https://yukibloom-1379189818.cos.ap-shanghai.myqcloud.com
COS_SECRET_ID=服务器专用的SecretId
COS_SECRET_KEY=服务器专用的SecretKey
```

建议在腾讯云访问管理中创建仅对此桶对象有上传、删除权限的服务身份；不要使用主账号密钥，不要将真实凭证提交到仓库。COS 配置缺失时，API 和导入命令会启动失败。上传仍由 Go API 校验图片并保存 PostgreSQL 元数据；JPEG、PNG 在保存前转为 WebP，已有 WebP 原样保存。转换后的图片以 `.webp` 对象键和 `image/webp` 类型上传，数据库记录转换后的大小和尺寸；JPEG 的 EXIF 方向会应用到图片像素。上传和图片库响应中的 `url` 是可直接使用的 COS 地址。公开文章列表与详情中的 `coverUrl` 也直接返回 COS 地址。文章、个人内容和站点内容导入命令使用同一组转换流程和 COS 配置。已上传到 COS 的旧图片不会自动转换；旧 `/uploads/{key}` 地址不再提供服务，已有内容如仍引用该路径需改为 COS URL。

COS 免费额度仅覆盖对应额度内的标准存储容量，不代表外网下行流量、请求等项目均免费。请在腾讯云费用中心查看 COS 实际账单；不要因启用 COS 而在服务器环境中暴露或提交密钥。

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

前端可按 `code` 判断错误类别，并直接展示 `message`，无需硬编码错误文案。公开图片地址会重定向到 COS；上传和图片库 API 直接返回 COS 图片 URL。

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

后台图片库支持 `GET/POST /api/v1/admin/media` 和 `DELETE /api/v1/admin/media/{id}`。只接受通过真实格式校验的 JPEG、PNG 和 WebP 图片，原文件单张不超过 10 MiB；新上传图片均以 WebP 保存，响应中的 `url` 为 COS 公开地址。仍被文章正文或封面引用的图片不能删除。

后台 Markdown 导入支持 `POST /api/v1/admin/posts/markdown/preview` 预览，以及 `POST /api/v1/admin/posts/markdown` 保存为草稿；两者使用 multipart 字段 `file`，可选字段 `locale` 默认 `zh-CN`，文件上限为 2 MiB。预览会返回原文件的 `coverPath`。若文件声明了封面，先通过图片接口上传，再在保存请求中传 `coverMediaId`，避免导入时无声丢失封面。旧文章迁移工具默认为只读预检查；检查通过后，使用 `DATABASE_URL=... go run ./cmd/import-posts -apply` 执行导入。整批文章与封面元数据在同一数据库事务中提交，失败时回滚并清理本次写入的图片文件。重复运行会跳过已有文章，不覆盖后台编辑内容。

个人内容接口提供 `GET /api/v1/footprints` 和 `GET /api/v1/timeline`，响应分别包含 `version` 与内容列表。管理员通过对应的 `PUT /api/v1/admin/footprints`、`PUT /api/v1/admin/timeline` 整体保存地点、停留、路线与实习经历；请求必须提交读取时的 `version`，旧版本返回 409，其他写入失败会回滚。

迁移旧足迹与实习经历时，`go run ./cmd/import-personal` 默认只检查 YAML 和图片。检查通过后添加 `-apply` 写入；若数据库已有足迹或实习经历，需明确加 `-replace` 才会替换现有数据。迁移会把路线图片上传到 COS 并更新图片地址。

站点内容公开接口为 `GET /api/v1/site-content`，后台通过 `GET/PUT /api/v1/admin/site-content` 管理站点资料、社交链接、分类、精选系列、导航、公告、友链、音乐列表和内容翻译。每条分类记录统一保存名称、slug、首页展示状态、封面、描述及顺序；文章继续通过分类名称关联分类。整体保存时需提交读取到的 `version`，旧版本会返回冲突提示。“关于我”等独立页面通过 `GET /api/v1/pages/{slug}?locale=zh-CN` 公开读取，后台支持页面列表、草稿保存、发布、撤回和删除。技术开关、主题设置、评论与统计服务配置继续保留在代码和环境变量中。

旧站点资料迁移使用 `go run ./cmd/import-site`，默认只读检查 `config/site.yaml`、`config/i18n-content.yaml`、“关于我”和歌单页面。预检查通过后添加 `-apply` 才会写入数据库；已有站点内容时还需显式添加 `-replace`。导入工具会把头像与精选入口图片上传到 COS。Docker 镜像也包含 `/import-posts`、`/import-personal`、`/import-site`，可挂载旧前端仓库后在容器内运行。

## 本地接口验证

请使用独立的测试数据库和 COS 测试桶，不要把测试数据写入正式数据库或生产图片桶。准备测试数据库后，在一个终端创建临时管理员并启动服务：

```bash
createdb yukibloom_test
export DATABASE_URL='postgres://user:password@127.0.0.1:5432/yukibloom_test?sslmode=disable'
ADMIN_USERNAME='api-test' ADMIN_PASSWORD='请换成至少 12 位的临时密码' go run ./cmd/create-admin
COOKIE_SECURE=false go run ./cmd/server
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
