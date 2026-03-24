# Antigravity-Go - Phase 3 扩展蓝图 (Expansion)

## 战略目标
将 Phase 2 构建的六大核心面（Memory, Trajectory, Actuation, Visual, Workspace, Versioning）向用户层与业务层释放，打造全方位可视化的协同工作台，并探索高级的自主验证与协作模式。

## 1. 深度可观测性与 Dashboard (Observability) [优先级：高]
- [ ] **API 层暴露**: 在 `internal/server/server.go` 中新增 REST API 路由，将 `TrajectoryManager` 和 `MemoryManager` 的能力暴露给前端。
- [ ] **轨迹树可视化 (Trajectory UI)**: 在 Web 前端 (`frontend/`) 接入轨迹接口，以时间轴形式展示 Agent 的思考链路、工具调用记录与回滚节点。
- [ ] **记忆管理面板 (Memory UI)**: 允许用户在 Web 端查看、管理内核自动沉淀的“架构决策”与“项目知识”。

## 2. 交互式视觉自测 (Visual E2E Loop) [优先级：中]
- [ ] **Agent 驱动测试**: 编写端到端指令，让 Agent 启动自身的 Web 前端，利用 `browser_open`, `browser_click`, `browser_screenshot` 自主验证前端界面的可用性。

## 3. 人机协同与安全审批 (Human-in-the-loop) [优先级：中]
- [ ] **前端 Diff 审批**: 当 Agent 准备执行 `apply_core_edit` 时，拦截请求并将 `edit_preview` 发送至 Web 端，等待用户点击“Approve”后才真正落盘。

## 4. 智能体多轨编排 (Multi-Agent Orchestration) [优先级：低]
- [ ] **Maker & Checker**: 基于现有的 `specialist.go`，实现“生成者”与“审查者”的对弈。Coder 生成代码 -> Reviewer 调用 `get_validation_states` 审计 -> 失败则触发 `rollback_to_step`。

---
*注：我们将继续秉持 CSE 闭环工程理念，每一步扩展都必须有明确的测试支撑。*
