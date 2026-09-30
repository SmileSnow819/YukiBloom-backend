# 内容 API 第一阶段实施计划

> **执行说明：**在 `codex/backend-foundation` 分支按任务顺序实施，使用 `executing-plans` 在当前会话内执行；每项提交前运行对应测试。

**目标：**让 Go 后端安全地管理文章和图片，公开接口只返回已发布内容，并能导入现有 Markdown。

**架构：**`internal/posts` 管理文章模型、SQL 和 HTTP；`internal/auth` 管理站长密码与会话；`internal/media` 管理图片元数据与磁盘文件；`cmd/import-posts` 执行旧文导入。Gin 路由注入数据库池和这些处理器，PostgreSQL 是唯一内容来源。

**技术栈：**Go 1.27、Gin、pgx/v5、PostgreSQL 17、bcrypt。

**设计依据：**`docs/superpowers/specs/2026-09-24-go-content-backend-design.md`。

## 全局约束

- 生产写接口全部要求管理员会话；公开接口只返回 `published` 文章。
- Markdown 原文存 `text` 字段，不在 Go 中渲染 HTML。
- 管理员会话令牌只以 SHA-256 摘要存数据库；Cookie 使用 `HttpOnly`、`SameSite=Lax`，线上要求 `Secure`。
- 修改请求要验证 CSRF 令牌；登录接口限流，不能公开注册。
- 请求限制图片类型与大小；文件名由服务生成，存放在持久化目录。
- 导入命令有预检查与正式导入模式，重复导入不产生重复文章。

## 文件结构

- `internal/database/migrations/0002_content.sql`：文章、管理员、会话、图片表与索引。
- `internal/posts/model.go`、`store.go`、`handler.go` 及测试：文章查询、写入、公开与管理 HTTP 接口。
- `internal/auth/password.go`、`session.go`、`handler.go` 及测试：密码、会话、登录、CSRF。
- `internal/media/store.go`、`handler.go` 及测试：校验并保存图片、列出资源。
- `internal/importer/markdown.go`、`cmd/import-posts/main.go` 及测试：解析 frontmatter 与导入。
- `internal/httpserver/router.go`：注入业务处理器并注册路由。
- `README.md`：接口、本地管理员初始化与导入说明。

## 任务 1：内容表及数据库约束

- [ ] 写数据库集成测试，使用 `TEST_DATABASE_URL` 检查 `posts(locale, slug)` 唯一、状态只能是 `draft/published`、删除被文章引用的图片时外键阻止。测试应先失败。
- [ ] 添加 `0002_content.sql`，至少包括 `admins`、`sessions`、`media`、`posts` 四张表；`posts` 有 `id UUID`、`locale`、`slug`、`title`、`description`、`body_markdown`、`status`、`published_at`、`created_at`、`updated_at`、`version`、`categories text[]`、`tags text[]`、`extra jsonb`、`cover_media_id`。索引支持已发布文章列表和 slug 查询。
- [ ] 运行测试、`go vet ./...`，提交表结构。

## 任务 2：站长登录

- [ ] 先写 `HashPassword/VerifyPassword` 的失败测试：正确密码成功、错误密码失败；会话令牌原文不能存数据库。
- [ ] 实现 bcrypt 密码处理、随机会话令牌、SHA-256 摘要，提供管理员初始化命令。
- [ ] 先写 Gin 处理器测试：错误密码为 401；登录成功设置安全 Cookie 和返回 CSRF；无会话或无 CSRF 的写请求为 401/403。
- [ ] 实现登录、登出、会话查询与认证中间件；使用数据库持久化会话，设置有效期，登录限流。
- [ ] 运行所有测试、`go vet ./...`，提交登录功能。

## 任务 3：文章 API

- [ ] 先写公开路由测试：草稿不能查询；已发布文章可按 slug 查询；列表有分页上限、语言与分类/标签筛选。
- [ ] 实现文章模型和 `pgx` SQL 查询；错误映射为 400/404/409/500，不泄露 SQL 详情。
- [ ] 先写管理路由测试：创建默认草稿；更新验证版本号；发布后公开可见；撤回后公开不可见。
- [ ] 实现 `GET /api/v1/posts`、`GET /api/v1/posts/:slug`、`GET/POST/PATCH /api/v1/admin/posts`、`POST /api/v1/admin/posts/:id/publish` 和 `.../unpublish`。管理路由接入任务 2 的会话与 CSRF 中间件。
- [ ] 运行测试与静态检查，提交文章 API。

## 任务 4：Markdown 导入

- [ ] 先写解析测试：读取已有文章样本，保留日期、标题、分类、标签和正文；无效 frontmatter 要报告文件路径；重复导入不重复写入。
- [ ] 用 Go YAML 解析 frontmatter，提供预检查和正式导入；兼容现有 `link` 与目录 slug 规则，冲突列入报告。
- [ ] 实现管理员 Markdown 上传预览，保存时与在线编辑走同一文章校验路径，默认草稿。
- [ ] 运行测试、在现有 55 篇文章上做预检查，提交导入功能。

## 任务 5：图片管理

- [ ] 先写上传校验测试：拒绝伪装图片、超大文件和 SVG，合法 JPEG/PNG/WebP 得到生成的路径；不允许路径穿越。
- [ ] 实现图片文件写入持久化目录与 `media` 元数据；写入失败清理临时文件。后台可上传、列出和删除未被引用图片，公开读取只提供已保存的安全文件。
- [ ] 运行测试、`go vet ./...` 和构建，更新 Compose 持久化图片 volume 与 README，提交图片功能。

## 完成标准

管理员可以登录并通过受保护 API 上传 Markdown、编辑与发布文章、上传图片。公开接口只显示已发布内容。对现有 55 篇 Markdown 的预检查给出可读报告，正式导入可重复执行。数据库和图片均持久化，所有可运行测试通过。
