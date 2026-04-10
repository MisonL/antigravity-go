# 仓库指南

## 项目结构与模块组织
- `cmd/ago/`：CLI 入口、工作流命令与脚手架逻辑。
- `internal/`：后端核心模块，包含 `agent`、`server`、`llm`、`core`、`tools`、`rpc`、`session` 等。
- `frontend/src/`：React + TypeScript Web 界面；构建产物会复制到 `internal/server/dist/` 供 Go 服务嵌入。
- `docs/`：产品设计、技术手册、阶段计划与复核记录。
- `scripts/`：内核提取、更新与维护脚本。
- `frontend/dist/` 与 `internal/server/dist/` 均为构建产物，禁止手工修改。

## 构建、测试与开发命令
- `make update-core`：提取或更新 `antigravity_core`，并刷新版本元数据。
- `make build`：先构建前端，再编译后端二进制 `ago`。
- `make run`：以本地 Web 模式运行（`./ago --web --no-tui`）。
- `go test ./...`：执行全部 Go 测试。
- `go vet ./...`：执行 Go 静态检查。
- `cd frontend && bun install && bun run dev`：启动前端开发服务器。
- `cd frontend && bun run lint && bun run build`：执行前端检查并生成生产构建。

## 代码风格与命名约定
- Go 目标版本为 `1.24`，提交前统一执行 `gofmt -w`。
- Go 包名使用小写；导出符号使用 `PascalCase`，内部辅助函数使用 `camelCase`。
- React 组件文件使用 `PascalCase`，例如 `TerminalPanel.tsx`；Hook 统一使用 `useXxx` 命名。
- 核心流程必须显式暴露错误，禁止静默回退或伪造成功路径。
- 遵循 `.gitignore`，不要提交日志、缓存、临时截图或本地调试残留。

## 测试要求
- Go 测试文件使用 `*_test.go` 并与对应包同目录放置。
- 开发阶段优先执行最小相关验证，例如 `go test ./internal/server -run TestTasks`，提交前必须补跑 `go test ./...`。
- 前端相关改动至少通过 `bun run lint` 与 `bun run build`。
- 行为变更必须同步补测试或更新复核记录，不能只靠人工判断。

## 提交与合并要求
- 提交信息遵循当前仓库历史风格：`feat(...)`、`fix(...)`、`refactor(...)`、`docs:`、`chore:`。
- 每个提交只承载一个清晰变更主题，必要时补 scope，例如 `feat(server): ...`。
- 合并请求需说明改动目的、涉及路径、验证命令与结果；前端改动需附界面截图或等效证据。
- 若存在破坏性变更、版本升级或核心兼容性调整，必须在描述中显式说明。
