# Antigravity-Go - Phase 5 演进蓝图 (Productization & Lifecycle)

## 战略目标
将 Agent 打造为具备“产品化思维”的全栈工程师，支持外部数据源（MCP）、长效任务续传（Session Persistence）以及自动化部署（Deployment）。

## 1. 外部数据面 (MCP Ecosystem) [优先级：高]
- [ ] **动态服务器加载**: 在 Web UI 支持动态添加/配置新的 MCP 服务器（如数据库查询、网络搜索）。
- [ ] **多源上下文感知**: 让 Agent 具备同时调用多个 MCP 工具来解决复杂业务逻辑的能力（如：查库 -> 分析 -> 写代码 -> 发通知）。

## 2. 任务续传面 (Session Persistence) [优先级：高]
- [ ] **全状态序列化**: 对接内核 `/GetAllCascadeTrajectories` 与 `/AddTrackedWorkspace`，实现会话级别的状态快照。
- [ ] **热启动增强**: 支持 `ago resume --id [UUID]` 彻底恢复包括环境变量、未提交 Diff 和 LLM 思考链路在内的所有现场。

## 3. 部署与交付面 (Deployment) [优先级：中]
- [ ] **新增子命令 `ago deploy`**:
  - 自动生成 Dockerfile 与 CI 配置。
  - 集成云端发布工具，支持一键部署至测试/生产环境。
- [ ] **发布预检**: 在部署前强制触发 Phase 4 的 ReviewerAgent 执行“上线前审计”。

## 4. 全栈脚手架 (Project Scaffolding) [优先级：低]
- [ ] **内置 Boilerplates**: 提供 Go + React + SQLite 的标准化全栈模板，Agent 可快速初始化业务项目。

---
*注：我们将继续秉持 CSE 闭环工程理念，每一项外部集成都必须通过‘机器预审’确保安全与一致性。*
