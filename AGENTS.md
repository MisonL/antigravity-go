# 仓库指南

## 项目结构
- `cmd/ago/`：CLI 入口、`doctor`/`mcp`/`resume` 等子命令，以及工作流编排。
- `internal/`：核心后端代码。重点目录包括 `core/`（内核宿主）、`rpc/`（Connect RPC 适配）、`server/`（Web API 与嵌入前端）、`tools/`（Agent 工具）、`corecap/`（内核能力门控）、`tui/`、`session/`。
- `frontend/src/`：React + TypeScript WebUI；测试放在同级 `*.test.ts(x)` 与 `src/test/`。
- `internal/server/dist/`、`frontend/dist/`：前端构建产物，只能由构建流程生成。
- `docs/`：设计文档、计划与复核记录；发布相关事实以 `CHANGELOG.md` 为准。

## 构建、测试与开发命令
- `make build`：安装前端依赖、构建前端、同步 `internal/server/dist`，再编译 `ago`。
- `make run`：本地启动 Web 模式，等价于 `./ago --web --no-tui`。
- `make update-core`：用脚本更新仓库内 `antigravity_core`。
- `go test ./...`：执行全部 Go 测试。
- `go vet ./...`：执行 Go 静态检查。
- `cd frontend && bun run test`：执行 Vitest。
- `cd frontend && bun run lint && bun run build`：执行 ESLint、TypeScript 与 Vite 构建。

## 代码风格与命名
- Go 版本为 `1.24`；提交前执行 `gofmt -w`。
- Go 导出符号用 `PascalCase`，内部函数用 `camelCase`，包名保持短小写。
- React 组件文件使用 `PascalCase.tsx`，Hook 使用 `useXxx`。
- 禁止静默降级、假成功路径和硬编码密钥；错误必须显式返回或展示。
- 不手改 `dist/`、日志、缓存、临时二进制。

## 测试与验证
- 后端改动至少跑最小相关测试，提交前必须跑 `go test ./...`、`go vet ./...`。
- 前端改动至少跑 `bun run test`、`bun run lint`、`bun run build`。
- 涉及 WebUI 的改动，需要确认内嵌产物已通过 `make build` 同步到 `internal/server/dist/`。

## 提交与合并
- 提交风格遵循现有历史：`feat(...)`、`fix(...)`、`refactor(...)`、`docs:`、`chore:`、`release:`、`ci:`。
- 一个提交只做一个清晰主题；不要把无关修复混入同一提交。
- PR 或交付说明应包含：变更目的、影响路径、验证命令、结果；界面改动补截图或等效运行证据。
