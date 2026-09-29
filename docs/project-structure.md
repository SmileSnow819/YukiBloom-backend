<!-- 此文件由 go run ./cmd/project-map 自动生成，请修改 docs/project-files.json 后重新运行。 -->

# 项目结构图

以下目录树和文件职责表由仓库文件清单自动生成。新增、删除或移动文件后，先更新 `docs/project-files.json`，再运行 `go run ./cmd/project-map`。

## 目录树

```text
YukiBloom-backend/
├── .dockerignore
├── .env.example
├── .gitignore
├── AGENTS.md
├── Dockerfile
├── PRODUCT.md
├── README.md
├── api-docs/
│   ├── swagger.json
│   └── swagger.yaml
├── cmd/
│   ├── create-admin/
│   │   ├── main.go
│   │   └── main_test.go
│   ├── import-personal/
│   │   └── main.go
│   ├── import-posts/
│   │   ├── main.go
│   │   └── main_test.go
│   ├── import-site/
│   │   └── main.go
│   ├── project-map/
│   │   └── main.go
│   └── server/
│       ├── main.go
│       └── main_test.go
├── compose.yaml
├── docs/
│   ├── project-files.json
│   ├── project-structure.md
│   └── superpowers/
│       ├── plans/
│       │   ├── 2026-09-24-backend-foundation.md
│       │   ├── 2026-09-24-content-api.md
│       │   └── 2026-09-25-code-comments-and-project-map.md
│       └── specs/
│           ├── 2026-09-24-go-content-backend-design.md
│           └── 2026-09-29-cos-media-storage-design.md
├── go.mod
├── go.sum
└── internal/
    ├── apiresponse/
    │   ├── errors.go
    │   └── response.go
    ├── auth/
    │   ├── handler.go
    │   ├── handler_test.go
    │   ├── password.go
    │   ├── password_test.go
    │   ├── session.go
    │   └── session_test.go
    ├── config/
    │   ├── config.go
    │   └── config_test.go
    ├── database/
    │   ├── database.go
    │   ├── database_test.go
    │   ├── migrate.go
    │   ├── migrate_test.go
    │   └── migrations/
    │       ├── 0001_initial.sql
    │       ├── 0002_content.sql
    │       ├── 0003_personal_content.sql
    │       ├── 0004_site_content.sql
    │       ├── 0005_site_content_revision.sql
    │       └── 0006_personal_content_revision.sql
    ├── httpserver/
    │   ├── auth_routes_test.go
    │   ├── router.go
    │   └── router_test.go
    ├── importer/
    │   ├── handler.go
    │   ├── handler_test.go
    │   └── markdown.go
    ├── media/
    │   ├── handler.go
    │   └── store.go
    ├── personal/
    │   ├── handler.go
    │   ├── handler_test.go
    │   ├── model.go
    │   ├── store.go
    │   └── store_test.go
    ├── posts/
    │   ├── handler.go
    │   ├── handler_test.go
    │   ├── model.go
    │   └── store.go
    └── sitecontent/
        ├── handler.go
        ├── legacy.go
        ├── model.go
        ├── store.go
        └── store_test.go
```

## 文件职责

| 文件 | 用途 |
| --- | --- |
| `.dockerignore` | Docker 镜像构建时忽略的本地文件和目录。 |
| `.env.example` | 本地及 Compose 部署需要的环境变量示例。 |
| `.gitignore` | Git 忽略规则，防止提交本地配置和生成文件。 |
| `AGENTS.md` | 提供给 AI 助手和维护者的协作、代码、安全和验证约定。 |
| `Dockerfile` | 构建并打包 Go API 服务和内容导入命令。 |
| `PRODUCT.md` | 记录产品目标、当前阶段、已确定决策和暂缓范围。 |
| `README.md` | 说明本地运行、接口、内容迁移和接口验证方法。 |
| `api-docs/swagger.json` | 由 Swaggo 根据 Go 接口注释生成、可导入 Apifox 的 Swagger JSON 文档。 |
| `api-docs/swagger.yaml` | 由 Swaggo 根据 Go 接口注释生成的 Swagger YAML 文档。 |
| `cmd/create-admin/main.go` | 连接数据库并创建首个站点管理员账号。 |
| `cmd/create-admin/main_test.go` | 验证管理员创建命令的参数和错误处理。 |
| `cmd/import-personal/main.go` | 预检查或迁移旧 YAML 中的足迹、路线、图片和实习经历。 |
| `cmd/import-posts/main.go` | 预检查或迁移旧 Markdown 文章及封面图片。 |
| `cmd/import-posts/main_test.go` | 验证文章批量导入失败时回滚文章和封面图片。 |
| `cmd/import-site/main.go` | 预检查或迁移旧站点配置、翻译、独立页面和内容图片。 |
| `cmd/project-map/main.go` | 扫描项目文件并根据职责清单生成目录结构图和文件用途表。 |
| `cmd/server/main.go` | 加载配置、连接 PostgreSQL、运行迁移并启动 Gin API。 |
| `cmd/server/main_test.go` | 验证 API 服务启动配置的错误信息不会泄露敏感值。 |
| `compose.yaml` | 定义 PostgreSQL 和 Go API 的本地 Compose 服务与持久化卷。 |
| `docs/project-files.json` | 维护项目每个文件的用途说明，供结构图生成命令读取。 |
| `docs/project-structure.md` | 由生成命令产出的目录树和逐文件职责索引。 |
| `docs/superpowers/plans/2026-09-24-backend-foundation.md` | 记录后端基础架构实施步骤和验证安排。 |
| `docs/superpowers/plans/2026-09-24-content-api.md` | 记录文章、图片和内容 API 的实施步骤。 |
| `docs/superpowers/plans/2026-09-25-code-comments-and-project-map.md` | 记录 Go 注释规范和自动生成项目结构图的实施计划。 |
| `docs/superpowers/specs/2026-09-24-go-content-backend-design.md` | 记录网站内容后端的整体设计、迁移范围和阶段验收标准。 |
| `docs/superpowers/specs/2026-09-29-cos-media-storage-design.md` | 记录 COS 图片存储接入方案、兼容约定和验收范围。 |
| `go.mod` | 声明 Go 模块路径、语言版本和直接依赖。 |
| `go.sum` | 锁定 Go 模块依赖及其校验值。 |
| `internal/apiresponse/errors.go` | 集中定义 API 错误类别、业务码和 HTTP 状态的映射。 |
| `internal/apiresponse/response.go` | 定义 JSON API 统一响应结构，以及成功、失败和中止响应的写入方法。 |
| `internal/auth/handler.go` | 处理管理员登录、会话校验、CSRF 校验、退出和登录限流。 |
| `internal/auth/handler_test.go` | 验证登录、会话保护、CSRF 校验和重复失败限流。 |
| `internal/auth/password.go` | 使用安全哈希算法创建并验证管理员密码。 |
| `internal/auth/password_test.go` | 验证管理员密码哈希和密码校验行为。 |
| `internal/auth/session.go` | 保存管理员账号和会话，并校验会话令牌及 CSRF 令牌。 |
| `internal/auth/session_test.go` | 使用临时 PostgreSQL 验证管理员和会话数据访问。 |
| `internal/config/config.go` | 从环境变量读取并校验服务运行配置。 |
| `internal/config/config_test.go` | 验证环境变量配置的默认值和非法输入。 |
| `internal/database/database.go` | 创建带连接超时的 PostgreSQL 连接池。 |
| `internal/database/database_test.go` | 验证数据库连接配置与连接失败处理。 |
| `internal/database/migrate.go` | 按文件名顺序、事务化地执行 SQL 数据库迁移。 |
| `internal/database/migrate_test.go` | 验证迁移顺序、重复执行和迁移失败回滚。 |
| `internal/database/migrations/0001_initial.sql` | 创建管理员、会话、图片和文章基础数据表。 |
| `internal/database/migrations/0002_content.sql` | 增加文章封面图片关联及文章查询索引。 |
| `internal/database/migrations/0003_personal_content.sql` | 创建足迹地点、停留、路线和实习经历数据表。 |
| `internal/database/migrations/0004_site_content.sql` | 创建站点资料、页面、导航、友链、翻译和音乐数据表。 |
| `internal/database/migrations/0005_site_content_revision.sql` | 保存站点内容版本，用于检测并发编辑冲突。 |
| `internal/database/migrations/0006_personal_content_revision.sql` | 保存足迹与实习经历各自的版本号，用于检测并发编辑冲突。 |
| `internal/httpserver/auth_routes_test.go` | 验证后台路由在未登录时拒绝访问。 |
| `internal/httpserver/router.go` | 注册健康检查、公开内容、管理员和上传文件路由。 |
| `internal/httpserver/router_test.go` | 验证健康检查、未知路由和代理来源识别。 |
| `internal/importer/handler.go` | 接收 Markdown 上传，提供文章预览并保存为草稿。 |
| `internal/importer/handler_test.go` | 验证 Markdown 封面未关联时会得到明确提示。 |
| `internal/importer/markdown.go` | 解析 Markdown frontmatter、文章正文和日期字段。 |
| `internal/media/handler.go` | 处理图片上传、列表、删除和公开文件读取请求。 |
| `internal/media/store.go` | 校验图片格式与大小，并管理图片元数据和持久化文件。 |
| `internal/personal/handler.go` | 处理公开足迹和时间线查询，以及管理员整体保存请求。 |
| `internal/personal/handler_test.go` | 验证个人内容接口的版本参数、成功保存和冲突响应。 |
| `internal/personal/model.go` | 定义足迹地点、停留、路线和实习经历的数据结构。 |
| `internal/personal/store.go` | 读取或事务化替换足迹、路线和实习经历数据。 |
| `internal/personal/store_test.go` | 验证足迹和实习经历的旧版本写入会被拒绝。 |
| `internal/posts/handler.go` | 处理文章公开查询、后台编辑、发布、撤回、删除和参数校验。 |
| `internal/posts/handler_test.go` | 验证删除文章接口会拒绝无效的文章 ID。 |
| `internal/posts/model.go` | 定义文章记录、编辑输入和分页响应结构。 |
| `internal/posts/store.go` | 执行文章的公开查询、后台增改删、发布和冲突检测 SQL。 |
| `internal/sitecontent/handler.go` | 处理站点内容与独立页面的公开读取及后台管理请求。 |
| `internal/sitecontent/legacy.go` | 解析旧站点 YAML、关于页和歌单页面数据。 |
| `internal/sitecontent/model.go` | 定义站点资料、导航、友链、翻译、音乐和独立页面模型。 |
| `internal/sitecontent/store.go` | 读取、事务化替换站点内容，并管理独立页面数据。 |
| `internal/sitecontent/store_test.go` | 验证站点内容读写、版本冲突、导入事务和页面发布。 |
