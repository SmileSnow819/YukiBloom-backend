# Go 后端基础服务实施计划

> **执行说明：**按任务逐项实施并核验。使用 `executing-plans` 在当前会话内执行；每项都有可独立检查的结果。

**目标：**在独立仓库中提供可运行的 Gin 服务、PostgreSQL 连接、版本化数据库迁移和本地 Docker Compose 环境，为后续登录与内容接口提供基础。

**架构：**`cmd/server` 负责启动；`internal/config` 读取环境变量；`internal/httpserver` 配置路由；`internal/database` 创建连接池并执行 SQL 迁移。服务和 PostgreSQL 在 Compose 内通信，数据库不映射公网端口。

**技术栈：**Go、Gin、pgx/v5、PostgreSQL、Docker Compose。

**设计依据：**`docs/superpowers/specs/2026-09-24-go-content-backend-design.md`。

## 全局约束

- 只开发后端仓库，不修改 YukiBloom 前端仓库。
- Go 模块路径为 `github.com/SmileSnow819/YukiBloom-backend`。
- 所有配置从环境变量读取；密钥和数据库密码不提交到 Git。
- 公开健康接口为 `GET /api/v1/health`；成功返回 HTTP 200 和 `{"status":"ok"}`。
- PostgreSQL 不映射到主机公网端口；本地运行可用 `localhost` 连接自行启动的数据库。
- 错误在服务启动时清晰报告；运行时日志不输出数据库连接串或密码。

## 文件结构

- `go.mod`、`go.sum`：Go 模块及依赖。
- `cmd/server/main.go`：装载配置、建立数据库连接、执行迁移、启动 HTTP 服务与优雅退出。
- `internal/config/config.go`、`config_test.go`：环境变量解析与校验。
- `internal/httpserver/router.go`、`router_test.go`：Gin 路由与健康响应。
- `internal/database/database.go`、`database_test.go`：pgx 连接池和数据库探测。
- `internal/database/migrations/0001_initial.sql`：迁移记录表的初始迁移样例。
- `internal/database/migrate.go`、`migrate_test.go`：按文件名顺序、事务化执行未运行的迁移。
- `Dockerfile`、`compose.yaml`、`.env.example`、`.gitignore`、`README.md`：本地运行和部署基础。

## 任务 1：配置与健康接口

**产物：**不依赖数据库也能构建并测试的 Gin 路由，以及可验证的运行配置。

- [ ] 执行 `go mod init github.com/SmileSnow819/YukiBloom-backend`，添加 Gin 与 pgx/v5 依赖。编写 `config_test.go`：`DATABASE_URL` 为空返回错误，`PORT` 为空默认 `8080`，非法端口返回错误。先执行 `go test ./internal/config`，确认因缺少实现而失败。
- [ ] 实现 `Load(getenv func(string) string) (Config, error)`，`Config` 包含 `Port string` 与 `DatabaseURL string`。使用 `strconv.Atoi` 校验端口在 `1..65535` 范围内，并返回带字段名的错误。再次执行测试确认通过。
- [ ] 编写 `router_test.go`：`NewRouter()` 的 `GET /api/v1/health` 返回 200 和 JSON `{"status":"ok"}`，未知路径返回 404。先运行测试确认失败。
- [ ] 实现 `NewRouter() *gin.Engine`，使用 `gin.New()`、`gin.Recovery()` 和一个只返回固定 JSON 的健康处理器。运行 `go test ./...` 与 `go vet ./...`。
- [ ] 提交配置与路由代码。

## 任务 2：数据库连接与迁移

**产物：**可连接 PostgreSQL，且重复启动不会重复执行迁移。

- [ ] 为 `database.go` 编写测试：无效 URL 返回错误；连接池初始化后可通过 `Ping(ctx)` 检查数据库可用。集成测试使用 `TEST_DATABASE_URL`，没有该变量时明确跳过。
- [ ] 实现 `Open(ctx context.Context, url string) (*pgxpool.Pool, error)`：解析配置、设置连接超时、创建池、执行 Ping，失败时关闭池并返回不含密码的错误。
- [ ] 编写迁移测试：对测试数据库运行 `Migrate` 两次，确认 `schema_migrations` 中同一版本仅有一条记录；失败的 SQL 不写入版本记录。
- [ ] 实现 `Migrate(ctx context.Context, pool *pgxpool.Pool) error`：内嵌 `migrations/*.sql`，先建立 `schema_migrations(version text primary key, applied_at timestamptz not null default now())`，按文件名排序，对每个未应用文件在事务里执行 SQL 并记录版本。`0001_initial.sql` 建立一个供后续内容表扩展的初始版本，内容为 `SELECT 1;`。
- [ ] 运行 `go test ./...`、`go vet ./...`，并在有 `TEST_DATABASE_URL` 的环境运行集成测试。提交数据库代码。

## 任务 3：可运行服务与本地部署

**产物：**`docker compose up --build` 可启动后端和数据库，健康接口可访问。

- [ ] 在 `main.go` 中加载配置、以带超时的 context 打开数据库、执行迁移，再启动带读写超时的 `http.Server`；收到 SIGINT/SIGTERM 时调用 `Shutdown` 并关闭数据库池。用编译和启动失败场景检查错误输出。
- [ ] 编写多阶段 `Dockerfile`：Go 构建二进制，运行阶段使用非 root 用户。编写 `compose.yaml`：`api` 和 `db` 两个服务，数据库 volume 持久化，数据库端口不映射主机，API 仅供本机访问以便日后由统一反向代理接入。
- [ ] 编写 `.env.example` 和 `.gitignore`，确保真实 `.env` 不会提交；README 用中文写本地启动、测试、健康检查、迁移方式和环境变量。
- [ ] 执行 `go test ./...`、`go vet ./...`、`go build -o /tmp/yukibloom-server ./cmd/server`；若 Docker 可用，再执行 `docker compose config` 和启动后的健康检查。记录不能运行的环境门槛。提交运行与部署文件。

## 本计划完成标准

`go test ./...` 和 `go vet ./...` 通过；服务在配置错误时清晰退出，在数据库可用时启动；健康接口返回指定 JSON；重复启动不重复迁移；仓库没有提交密钥；Compose 不公开 PostgreSQL 端口。
