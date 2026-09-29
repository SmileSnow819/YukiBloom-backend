# COS 图片存储接入实施计划

> **For agentic workers:** 按任务顺序在当前 `codex/` 分支实施；步骤使用复选框跟踪。

**目标：** 为 YukiBloom Go 后端增加可配置的腾讯云 COS 图片存储，同时保留本地存储模式和现有 `/uploads/{key}` 图片地址。

**架构：** 在 `internal/media` 定义对象存储接口及本地、COS 两个实现。配置模块统一创建所选实现，服务和三个导入命令复用它；COS 公开图片请求校验数据库元数据后重定向到公有对象 URL。

**技术栈：** Go 1.27.1、Gin、pgx、腾讯云官方 `github.com/tencentyun/cos-go-sdk-v5@v0.7.75`。

**Spec：** `docs/superpowers/specs/2026-09-29-cos-media-storage-design.md`

## 全局约束

- API 路径、JSON 字段、Go 标识符、数据库字段和第三方协议名称按通用英文命名。
- 面向项目维护者的说明、报错提示、文档和接口错误信息使用中文。
- PostgreSQL 保存图片元数据，COS 保存图片对象；不新增数据库迁移。
- 本地模式为默认模式且无需 COS 配置；COS 密钥仅由服务端环境变量读取。
- 图片仍只接受有效 JPEG、PNG、WebP，单张不超过 10 MiB，宽高像素乘积不超过 2000 万。
- 公共图片继续使用 `/uploads/{key}`；COS 模式的对象匿名可读、匿名不可写。
- 导入工具默认仍为预检查；写入操作仍需显式 `-apply`。
- 不部署服务器、不迁移现有本地图片、不接入 CDN 或图片处理服务。

---

### Task 1：增加 COS 配置与存储后端选择

**文件：**
- 修改：`internal/config/config.go`
- 修改：`internal/config/config_test.go`（仅随实现同步现有配置结构，不新增测试用例）
- 修改：`cmd/server/main.go`
- 修改：`cmd/import-posts/main.go`
- 修改：`cmd/import-personal/main.go`
- 修改：`cmd/import-site/main.go`
- 修改：`compose.yaml`
- 修改：`.env.example`
- 修改：`go.mod`、`go.sum`

**接口：**
- 配置产出 `MediaStorage`、`COSBucket`、`COSRegion`、`COSSecretID`、`COSSecretKey`、`COSPublicBaseURL`。
- 增加 `media.NewStorage(config.Config) (media.ObjectStorage, error)`，由所有服务入口复用。

- [ ] `Load` 默认 `MEDIA_STORAGE=local`；只有 `cos` 可选。COS 模式要求 `COS_BUCKET`、`COS_REGION`、`COS_SECRET_ID`、`COS_SECRET_KEY`，可选 `COS_PUBLIC_BASE_URL`。错误只指出缺少变量名，不回显其值。
- [ ] 使用锁定版本 `github.com/tencentyun/cos-go-sdk-v5@v0.7.75`，用 region 与 bucket endpoint 创建客户端。
- [ ] 更新 API 与导入命令创建配置的调用点；所有命令使用同一图片存储选择逻辑。
- [ ] Compose 将 `MEDIA_STORAGE`、`COS_BUCKET`、`COS_REGION`、`COS_SECRET_ID`、`COS_SECRET_KEY`、`COS_PUBLIC_BASE_URL` 传给 API，默认本地模式且不要求填 COS 密钥。
- [ ] 运行 `gofmt` 和 `go build ./...`，再运行 `git diff --check`。

### Task 2：抽象对象存储并实现本地与 COS 操作

**文件：**
- 创建：`internal/media/storage.go`
- 创建：`internal/media/storage_local.go`
- 创建：`internal/media/storage_cos.go`
- 修改：`internal/media/store.go`

**接口：**
- `ObjectStorage` 提供 `Put(ctx, key, data, contentType) error`、`Delete(ctx, key) error`、`LocalPath(key) string` 和 `PublicURL(key) string`。
- `media.Store` 持有 `ObjectStorage`，并通过 `RemoveObject(ctx, key)` 清理尚未提交元数据的图片对象。
- `SaveInTx` 返回 `(Item, storageKey string, error)`；第二个结果用于事务回滚时删除对象。

- [ ] 将 `writeImageFile` 文件操作封装进本地实现，保留原子临时文件写入和文件权限。
- [ ] COS 实现将已校验字节写入生成的 key，设置正确 `Content-Type`，并提供安全拼接的公开 URL。
- [ ] `Store.save` 先存对象再写媒体元数据；元数据失败时调用后端 `Delete` 清理对象。
- [ ] 图片元数据读取仍通过 PostgreSQL；返回条目 URL 仍为 `/uploads/` 加 key。
- [ ] 对象存储失败只向调用层返回错误，由处理器记录详细错误并返回中文稳定消息。
- [ ] 运行 `gofmt`、`go build ./...` 和 `git diff --check`。

### Task 3：接入公开读取、删除和导入回滚

**文件：**
- 修改：`internal/media/store.go`
- 修改：`internal/media/handler.go`
- 修改：`cmd/import-posts/main.go`
- 修改：`cmd/import-personal/main.go`
- 修改：`cmd/import-site/main.go`

**接口：**
- `Store.PublicFile(ctx, key)` 返回存储 key 对应的 MIME、路径或外部公开 URL及错误。
- `Store.RemoveObject(ctx, key)` 删除未关联数据库记录的对象，仅供失败回滚。

- [ ] `PublicFile` 校验 key 后查询媒体记录；本地模式继续由 API 返回文件，COS 模式返回 302 到对象公开 URL。
- [ ] `DeleteUnused` 保持当前引用检查和数据库删除顺序，提交事务后用所选存储后端删除对象；对象删除失败时返回可记录的错误，不泄露内部信息。
- [ ] 文章导入失败时按 `SaveInTx` 返回的 storage key 调用 `RemoveObject`，不再对 COS key 使用 `os.Remove`。
- [ ] 个人内容与站点导入继续通过媒体 ID 调用 `DeleteUnused` 清理失败时新建的图片。
- [ ] 运行 `gofmt`、`go build ./...` 和 `git diff --check`。

### Task 4：补齐配置与运维说明

**文件：**
- 修改：`README.md`
- 修改：`docs/project-files.json`
- 生成：`docs/project-structure.md`

- [ ] README 说明本地默认模式、COS 环境变量、用户桶名 `yukibloom-1379189818`、地域 `ap-shanghai`、公有读私有写要求、凭证保管和上传/删除流程。
- [ ] 说明免费存储额度不等于流量与请求全免费，按腾讯云 COS 账单查看实际费用。
- [ ] 登记新增 Go 文件职责并运行 `go run ./cmd/project-map`。
- [ ] 最终执行 `git diff --check` 和 `go build ./...`，检查 `git status --short` 确认没有意外修改。
