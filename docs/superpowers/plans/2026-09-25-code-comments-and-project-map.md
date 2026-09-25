# Go 代码注释与项目结构图实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为后端生产 Go 代码补齐中文函数说明，并提供能检查文件说明完整性、自动生成目录结构图和文件职责表的命令。

**Architecture:** 函数注释直接写在 Go 源码声明前，说明用途、每个参数和返回值。文件职责只记录在 JSON 清单中，`cmd/project-map` 扫描仓库实际文件并生成 Markdown 目录树和职责表；新增文件没有说明时命令报错。

**Tech Stack:** Go 标准库、JSON、Markdown。

**Spec:** `PRODUCT.md` 与 `AGENTS.md` 中的中文说明要求、代码清晰原则和当前后端范围。

## Global Constraints

- 面向项目维护者的说明、报错提示、文档和接口错误信息使用中文。
- 后端使用 Go、Gin、PostgreSQL 和 pgx；不为文档工具引入第三方依赖。
- 不改动文章、接口、数据库或部署行为；本次只补说明和文档生成工具。
- 生成的结构图必须由仓库内命令重复生成，文件清单变化时不得静默遗漏文件。
- 命令只处理本地文件，不连接数据库或线上服务。

---

### Task 1: 实现项目结构图生成器

**Files:**
- Create: `cmd/project-map/main.go`
- Create: `docs/project-files.json`
- Create: `docs/project-structure.md`
- Modify: `README.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Produces: `go run ./cmd/project-map`；扫描项目文件、校验职责清单并更新结构图。

- [ ] 扫描仓库文件并忽略 `.git`；读取 JSON 文件说明，按稳定路径排序。对漏项、多余路径和重复路径返回中文错误。
- [ ] 生成含自动更新标记、目录树、逐文件职责表和更新命令的 Markdown。生成文档本身也登记职责。
- [ ] 运行 `go run ./cmd/project-map`，检查所有项目文件均有说明，且图表生成成功。
- [ ] 在 README 写明生成命令，在 AGENTS 写明新增、删除或移动文件后需更新清单并重生成图表。

### Task 2: 为生产 Go 源码补齐中文注释

**Files:**
- Modify: `cmd/**/*.go`
- Modify: `internal/**/*.go`
- Modify: `docs/project-files.json`

**Interfaces:**
- Consumes: 现有函数签名和调用关系。
- Produces: 生产函数声明前的中文用途、参数和返回值说明，以及每个源文件的职责说明。

- [ ] 覆盖服务启动、管理员创建、导入工具、配置、数据库、认证、文章、图片、足迹和站点内容模块的生产函数。无返回值时明确写“返回：无”；测试文件的测试框架参数不重复解释。
- [ ] 逐项核对 `docs/project-files.json` 的 Go 文件用途，职责说明与源码责任相符，避免在源码注释和清单重复维护相同描述。
- [ ] 执行 `gofmt` 和结构图生成命令；抽查多返回值、上下文参数、事务方法和 HTTP 处理函数的参数及返回说明。

### Task 3: 核对最终图表和改动

**Files:**
- Modify: `docs/project-files.json`
- Modify: `docs/project-structure.md`

**Interfaces:**
- Produces: 完整、稳定排序的项目目录图与逐文件职责表。

- [ ] 再次运行 `go run ./cmd/project-map` 并确认重复生成不产生差异。
- [ ] 运行 `git diff --check`，确认没有格式错误或 API 行为改动。
