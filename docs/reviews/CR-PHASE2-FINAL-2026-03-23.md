# Phase 2 全量开发成果终极复核

日期: 2026-03-23

## 复核范围

- `internal/rpc/` 新增文件: `memory.go`, `trajectory.go`, `actuation.go`, `browser.go`, `versioning.go`
- `internal/corecap/` 所有 Manager 实现
- `internal/agent/agent.go`
- `internal/tools/core_v2.go` 及子文件
- `cmd/agy/subcommands.go`

## 审计结论

- 已确认 `internal/corecap/` 仅依赖 `rpc.Client`，未向上层泄漏 RPC 细节，分层关系保持为 `tools -> corecap -> rpc`。
- 已确认 `browser` 与 `trajectory` 相关 RPC 不再残留于 `internal/rpc/client.go`，重复定义已迁移到独立文件。
- 发现并修复 3 个 Phase 2 收口问题。

## 发现与修复

### 1. `FinalizeTask` 会把记忆写入失败升级为主流程失败

- 风险:
  `Run` / `RunStream` 在最终回答路径上同步返回 `FinalizeTask` 的错误，违反“记忆失败不得阻塞主流程”的要求。
- 修复:
  在 `internal/agent/agent.go` 中将 `FinalizeTask` 调整为 best-effort 语义，仅记录日志，不再向主流程返回错误；同时保留 `Run` 与 `RunStream` 两条最终回答路径上的触发点。
- 位置:
  `internal/agent/agent.go:335`
  `internal/agent/agent.go:387`
  `internal/agent/agent.go:470`

### 2. 工具层未完整暴露已实现的内核能力

- 风险:
  `FocusUserPage`、`GetCascadeTrajectory`、`AddTrackedWorkspace` 已在 `rpc/corecap` 层存在，但工具层没有暴露，导致浏览器焦点、单轨迹读取、增量工作区注册没有贯穿到 Agent 能力面。
- 修复:
  新增 `browser_focus`、`trajectory_get`、`workspace_track` 三个工具，并在 `buildBaseAgent` 中完成注册。
- 位置:
  `internal/tools/core_v2_browser.go:122`
  `internal/tools/core_v2_trajectory.go:15`
  `internal/tools/core_v2_workspace.go:15`
  `cmd/agy/subcommands.go:196`

### 3. 终结语义缺少回归测试

- 风险:
  旧测试把“记忆失败应中断主流程”当成正确行为，和目标语义相反。
- 修复:
  改写原测试，并新增 `RunStream` 路径用例，验证记忆失败时主流程仍返回最终答案。
- 位置:
  `internal/agent/agent_test.go:168`
  `internal/agent/agent_test.go:195`

## 一致性检查

- `browser` 相关 RPC 方法未在 `internal/rpc/client.go` 残留重复定义。
- `trajectory` 相关 RPC 方法未在 `internal/rpc/client.go` 残留重复定义。
- 工具参数字段与现有 RPC JSON 字段口径一致:
  - 浏览器: `page_id`, `selector`, `text`, `delta_x`, `delta_y`
  - 轨迹: `id`
  - 版本: `step_id`
  - 工作区: `root`
  - 原始透传请求: `request`
- `internal/corecap/` 中各 Manager 继续只做参数校验与 RPC 转发，无跨层耦合。

## 清理结果

- 目标范围内未发现 `TODO` / `FIXME` / `XXX` 残留。
- 未发现新增测试代码混入 `src` 类核心目录的问题；新增测试位于 `internal/agent/agent_test.go`。
- 未发现目标范围内重复 RPC 定义残留。

## 验证记录

- 命令:
  `GOCACHE=$(pwd)/.go-cache go build -o agy cmd/agy/*.go`
- 退出码:
  `0`

- 命令:
  `GOCACHE=$(pwd)/.go-cache go test ./...`
- 退出码:
  `0`

- 命令:
  `rg -n "TODO|todo|\\[ ]|FIXME|XXX" internal/rpc internal/corecap internal/agent/agent.go internal/tools/core_v2* cmd/agy/subcommands.go -g '*.go'`
- 结果:
  无命中
