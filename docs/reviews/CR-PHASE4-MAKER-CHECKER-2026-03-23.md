# Phase 4 Maker Checker 集成验证记录

日期: 2026-03-23

## 变更范围

- `internal/agent/specialist.go`
- `internal/agent/maker_checker.go`
- `internal/agent/approval.go`
- `internal/agent/agent.go`
- `internal/server/websocket.go`
- `internal/agent/agent_test.go`
- `internal/agent/maker_checker_test.go`

## 实现摘要

- 将 `apply_core_edit` 和 `write_file` 改为后置审批模型：先执行，再做机器预审，最后才发起人工确认。
- 机器预审阶段会调用 `get_validation_states`，并在可用时调用 `run_command` 执行 `go test ./...`。
- 若预审失败，`apply_core_edit` 优先执行 `rollback_to_step`；若无法使用轨迹回滚，则退回文件快照恢复。
- 人工拒绝时同样执行自动回滚，避免“已写入但未批准”的脏状态残留。
- WebSocket 审批弹窗会附带机器预审摘要，明确显示“机器预审通过，等待人工最终确认”。

## 回归测试

- 命令:
  `GOCACHE=$(pwd)/.go-cache go test ./internal/agent`
- 退出码:
  `0`

- 命令:
  `GOCACHE=$(pwd)/.go-cache go test ./...`
- 退出码:
  `0`

- 命令:
  `GOCACHE=$(pwd)/.go-cache go build -o agy cmd/agy/*.go`
- 退出码:
  `0`
