# Antigravity-Go 阶段四多智能体编排与自愈循环

## 战略目标
从“单体智能体”向“多智能体协作系统”演进。引入生成者与审查者内部对弈机制，并在正式向人工发起审批前，实现自动化代码质量把关与测试自愈。

## 1. 多智能体编排 [优先级：高]
- [ ] **重构 Specialist 体系**：在 `internal/agent/specialist.go` 中抽象 `CoderAgent` 与 `ReviewerAgent` 两类角色。
- [ ] **建立内部对弈机制**：
  - Coder 生成代码并调用 `apply_core_edit`
  - 系统内部拦截该编辑，先交给 Reviewer 调用 `get_validation_states` 审计
  - 若验证失败或 Reviewer 发现逻辑漏洞，则触发 `rollback_to_step` 并反馈给 Coder 重试
  - 只有 Reviewer 通过后，才向 Web 端抛出 `approval_request` 等待人工最终审批

## 2. 自动化测试闭环 [优先级：高]
- [ ] **强化本地执行沙箱**：增强 `run_shell_command` 工具的隔离与验证能力，让智能体能安全执行 `go test` 或 `npm test`。
- [ ] **测试驱动自愈**：强制智能体在提交功能代码时同步生成对应测试；若测试不通过，则自动进入“生成 -> 测试 -> 自愈”循环，直至全绿。

## 3. 工作流命令扩展 [优先级：中]
- [ ] **新增子命令**：
  - `ago review`：仅执行代码审查，不直接修改代码
  - `ago auto-fix`：扫描当前目录编译错误或 Lint 警告，并自动进入“生成 -> 测试 -> 自愈”循环

*注：本项目坚持 CSE 闭环工程理念，引入多智能体的目的是降低人工审批负担，提高自动补丁的一次通过率。*
