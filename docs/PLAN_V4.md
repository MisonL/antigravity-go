# Antigravity-Go - Phase 4 多智能体编排与自愈循环 (Orchestration & Auto-Healing)

## 战略目标
从“单体 Agent”向“多智能体协作（Multi-Agent System）”演进。引入生成者（Maker）与审查者（Checker）的内部对弈机制，并在正式向人类发起审批前，实现自动化的代码质量把关与测试自愈。

## 1. 智能体多轨编排 (Maker & Checker) [优先级：高]
- [ ] **重构 Specialist 体系**: 在 `internal/agent/specialist.go` 中，抽象出 `CoderAgent` (负责写代码) 和 `ReviewerAgent` (负责跑验证和审查)。
- [ ] **对弈机制 (Internal Dialogue)**: 
  - Coder 生成代码并调用 `apply_core_edit`。
  - 拦截该编辑（在系统内部），先交由 Reviewer 调用 `get_validation_states` 审计。
  - 如果验证失败或 Reviewer 发现逻辑漏洞，触发内部 `rollback_to_step` 并将错误反馈给 Coder 进行重试。
  - 只有 Reviewer 通过后，才向 Web 端抛出 `approval_request` 请求人类最终审批。

## 2. 自动化测试闭环 (Auto-Test Loop) [优先级：高]
- [ ] **本地执行沙箱**: 强化 `run_shell_command` 工具的隔离与验证能力，让 Agent 能够安全地执行 `go test` 或 `npm test`。
- [ ] **测试驱动自愈**: 强制 Agent 在提交功能代码的同时，必须生成对应的单元测试。若测试不通过，Agent 会自我循环修复，直至测试全绿。

## 3. 工作流指令扩展 (Workflow CLI) [优先级：中]
- [ ] **新增子命令**: 
  - `ago review`：仅执行代码审查任务，不直接修改代码。
  - `ago auto-fix`：扫描当前目录的编译错误或 Lint 警告，并自动进入“生成->测试->自愈”循环。

---
*注：本项目坚持 CSE 闭环工程理念，引入多智能体是为了降低人类审批时的负担，提高自动化补丁的首次通过率（First-time Pass Rate）。*
