# Antigravity-Go 阶段二演进蓝图（以内核优先为核心）

## 战略目标
将系统从“工具驱动”升级为“内核托管”，实现可对标 Claude Code 的沉浸式交互、Gemini CLI 的长效记忆以及 Codex 的事务化编辑能力。

## 1. 记忆面：解决长效认知
- [ ] **接入 RAG 存储**：对接内核 `/UpdateCascadeMemory`，在任务结束时自动归纳并写回架构决策与避坑指南。
- [ ] **拉取语义上下文**：会话启动时通过 `/GetUserMemories` 自动注入与当前仓库相关的历史记忆片段。
- [ ] **补齐记忆管理工具**：为智能体封装 `memory_save` 和 `memory_query` 工具。

## 2. 轨迹面：解决任务溯源
- [ ] **构建时光机机制**：适配内核 `/GetAllCascadeTrajectories`，支持 TUI 与 Web 端回溯思考链路。
- [ ] **生成 Markdown 报告**：对接 `/ConvertTrajectoryToMarkdown`，支持一键导出结构化执行报告。
- [ ] **支持分支回滚**：探索 `/RevertToCascadeStep`，实现决策失误后的秒级恢复。

## 3. 执行面：解决编辑风险
- [ ] **事务化编辑**：将 `ApplyCodeEdit` 升级为“预览-确认”模式，对接 `/GetPatchAndCodeChange` 生成可视化差异。
- [ ] **增强自校正**：深度整合 `/GetDiagnostics` 与 `/GetCodeValidationStates`，实现补丁后的自动环境校验。
- [ ] **生成提交信息**：利用 `/GenerateCommitMessage` 提供半自动版本管理能力。

## 4. 视觉与 Web 操控面：解决交互边界
- [ ] **补齐自动控制**：从“截图”演进到“操控”，封装 `browser_click`、`browser_type`、`browser_scroll` 等原生内核动作。
- [ ] **增强智能调试**：打通 `/SmartOpenBrowser` 与 `/CaptureConsoleLogs` 闭环，支持智能体自动排查 Web 控制台报错。

## 下一步技术路线：收拢架构边界
- [ ] 创建 `internal/corecap/` 目录，按四个能力面拆分 RPC 调用逻辑。
- [ ] 逐步把 `internal/rpc/client.go` 的膨胀逻辑迁移到专门的能力模块中。
