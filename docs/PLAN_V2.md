# Antigravity-Go - Phase 2 演进蓝图 (Core-First Evolution)

## 战略目标
将系统从“工具驱动”升级为“内核托管”，实现对标 Claude Code 的沉浸式交互、Gemini CLI 的长效记忆以及 Codex 的事务化编辑。

## 1. 记忆面 (Memory Plane) - 解决长效认知
- [ ] **RAG 存储集成**: 对接内核 `/UpdateCascadeMemory`，在任务结束时自动归纳并写回架构决策与避坑指南。
- [ ] **语义上下文拉取**: 会话启动时通过 `/GetUserMemories` 自动注入与当前 Repo 相关的历史记忆片段。
- [ ] **记忆管理工具**: 为 Agent 封装 `memory_save` 和 `memory_query` 工具。

## 2. 轨迹面 (Trajectory Plane) - 解决任务溯源
- [ ] **时光机机制**: 实现对内核 `/GetAllCascadeTrajectories` 的适配，支持 TUI/Web 端回溯思考链路。
- [ ] **Markdown 报告生成**: 对接 `/ConvertTrajectoryToMarkdown`，支持一键导出结构化的任务执行报告。
- [ ] **分支回滚**: 探索 `/RevertToCascadeStep`，实现决策失误时的秒级状态恢复。

## 3. 执行面 (Actuation Plane) - 解决编辑风险
- [ ] **事务化编辑**: 将 `ApplyCodeEdit` 升级为“预览-确认”模式，对接 `/GetPatchAndCodeChange` 生成可视化 Diff。
- [ ] **自校正增强**: 深度整合 `/GetDiagnostics` 与 `/GetCodeValidationStates`，实现自动化的补丁后环境校验。
- [ ] **提交生成**: 利用内核 `/GenerateCommitMessage` 实现改动后的半自动版本管理。

## 4. 视觉与 Web 操控 (Visual Plane) - 解决交互边界
- [ ] **全自动控制**: 从“截图”进化为“操控”，封装 `browser_click`, `browser_type`, `browser_scroll` 等原生内核动作。
- [ ] **智能调试**: 实现 `/SmartOpenBrowser` 与 `/CaptureConsoleLogs` 闭环，支持 Agent 自动排查 Web 控制台报错。

---
### 下一步技术路线：架构收口
- [ ] 创建 `internal/corecap/` 目录，按上述四个面拆分 RPC 调用逻辑。
- [ ] 逐步将 `internal/rpc/client.go` 的膨胀逻辑迁移至 specialized capabilities 模块。
