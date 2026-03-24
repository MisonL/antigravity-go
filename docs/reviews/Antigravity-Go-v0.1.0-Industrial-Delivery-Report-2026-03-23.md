# Antigravity-Go v0.1.0 工业级交付报告

日期: 2026-03-23

## 1. 交付结论

本次 Phase 6E 已完成以下工业化收口：

- 部署执行器补齐：`internal/tools/deployment.go` 现可基于当前工作区生成 production-ready `Dockerfile`、`.dockerignore`、`docker-compose.yml`，并保留 GitHub Actions 工作流导出能力。
- Agent 部署工具补齐：新增 `deploy_project` 工具，默认写入部署工件，可选写入 `.github/workflows/deploy.yml`。
- 前端大 Diff 性能收口：`frontend/src/components/ApprovalModal.tsx` 对超过 2000 行的 Diff 默认折叠未修改上下文，并对预览区和 chunk 列表启用分批渲染。
- 启动自愈补齐：`cmd/ago/main.go` 增加环境自检；检测到配置文件损坏或数据目录异常时，显式阻断并提供 `--safe-start` / `--auto-repair` 两条恢复路径。
- Self-Evolving 回归补齐：新增 `internal/server/self_evolving_test.go`，验证 Agent 可通过并行 worker 生成两处修改，并通过 chunk approval 完成最终落盘。

## 2. 核心性能指标

- 大 Diff 预览折叠阈值：`2000` 行。
- 大 Diff 首屏渲染预算：`600` 行。
- 大 Diff 增量加载步长：`400` 行。
- Chunk 列表首屏批次：`20` 项。
- 前端生产构建：`npm run build` 通过，Vite 构建耗时 `19.88s`。
- 关键产物体积：
  - `dist/assets/index-DXiXlUqt.js`: `81.59 kB`，gzip `22.63 kB`
  - `dist/assets/index-C9MgsFvQ.css`: `17.10 kB`，gzip `3.65 kB`
  - `dist/assets/react-vendor-CsfCmJYk.js`: `192.49 kB`，gzip `60.35 kB`
- Self-Evolving 专项测试：`go test -run TestSelfEvolvingParallelWorkersSupportChunkApproval ./internal/server` 通过，耗时 `0.027s`。
- 全量 Go 回归：`go test ./...` 通过。

## 3. 安全隔离边界

- 工作区文件边界：
  - `deploy_project`、`write_file`、`apply_core_edit` 仍受工作区路径约束，路径解析走 `WorkspaceContext` / `ResolvePathWithinWorkspace`。
- 人工审批边界：
  - 敏感写操作继续保留人工确认。
  - Maker-Checker 顺序恢复为“先自动预审，再人工最终确认”。
  - Web 审批面继续支持 chunk approval，且审批预览在自动预审后仍能基于修改前快照重建 diff。
- Web 暴露边界：
  - 服务端仍默认只允许回环地址监听。
  - 若监听 `0.0.0.0` / `::`，必须显式提供 token。
  - 生成的 `docker-compose.yml` 强制要求 `AGO_WEB_TOKEN`。
- 容器运行边界：
  - 生成的 compose 将 `antigravity_core` 以只读挂载方式注入：`./deploy/runtime/antigravity_core:/opt/antigravity/bin/antigravity_core:ro`。
  - 运行数据卷独立挂载到 `ago-data:/var/lib/ago`，避免容器内状态与工作区源代码混写。
- 自愈边界：
  - `--safe-start` 使用隔离数据目录启动，不直接触碰原损坏数据。
  - `--auto-repair` 先备份损坏路径到 `.bak-时间戳`，再重建最小可运行配置与目录结构。

## 4. 验证证据

- 前端构建：
  - `cd frontend && npm run build`
- 部署工具链定向测试：
  - `go test ./internal/tools`
- 启动自愈与 CLI 定向测试：
  - `go test ./cmd/ago`
- 审批与 Self-Evolving 定向测试：
  - `go test ./internal/server`
  - `go test -run TestSelfEvolvingParallelWorkersSupportChunkApproval ./internal/server`
- 最终全量回归：
  - `go test ./...`

## 5. 最终状态

- 部署配置导出：已完成
- 大 Diff 审批性能收口：已完成
- 启动自愈：已完成
- Self-Evolving 并行修改 + chunk approval：已完成
- 全量测试：已通过
