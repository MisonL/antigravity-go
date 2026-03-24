# Phase 5 云原生交付架构方案

日期: 2026-03-23

## 目标

- 为 `agy deploy` 提供统一的交付入口。
- 在部署前自动生成容器交付物：`Dockerfile` 与 `.dockerignore`。
- 在执行部署前强制通过 `ReviewerAgent` 的生产预检。
- 为镜像构建、推送、部署记录落盘提供事务性语义，失败时自动回滚部署记录。

## 约束

- 保持 Debug-First，不允许静默跳过失败步骤。
- 不引入隐藏回退路径；失败必须显式返回。
- 当前阶段优先支持本地 Docker 构建与推送命令封装，真实云端发布保留为可扩展执行器。
- 不修改用户业务代码以外的无关文件；部署产物只生成在工作区根目录。

## 总体设计

`agy deploy` 采用三段式流水线：

1. 代码扫描与交付物生成
2. ReviewerAgent 预检
3. 构建/推送/记录提交

命令层位于 `cmd/agy/workflow_commands.go`，负责参数解析、Agent/Runtime 初始化和结果输出。

工具层位于 `internal/tools/deployment.go`，负责：

- 工作区扫描
- Dockerfile 与 `.dockerignore` 计划生成
- 部署记录管理
- Docker 命令封装
- 回滚记录落盘

## 事务性设计

部署流程采用“写前日志 + 状态机提交”模型，核心对象为 `DeploymentRecord`。

### 状态机

记录状态只允许按以下顺序推进：

`prepared -> checked -> built -> pushed -> committed`

失败时进入：

`rollback_pending -> rolled_back`

### 原子边界

- `prepared`: 先生成唯一部署 ID，并将工作区、镜像标签、目标环境、时间戳、上一个已知镜像引用写入记录文件。
- `checked`: 仅当 ReviewerAgent 返回 PASS，才允许推进。
- `built`: `docker build` 成功后写入镜像摘要或构建标签。
- `pushed`: `docker push` 成功后写入远端引用。
- `committed`: 所有步骤成功后一次性落最终状态。

任何阶段失败都必须：

1. 将记录置为 `rollback_pending`
2. 写入失败步骤、错误信息、可用的上一个镜像引用
3. 执行回滚动作
4. 将记录置为 `rolled_back`

### 回滚语义

当前阶段的“自动回滚部署记录”定义为：

- 若失败发生在 `checked` 之前：仅回滚记录状态，不生成已发布结论。
- 若失败发生在 `built` 之后、`committed` 之前：保留构建和推送证据，但最终记录必须以 `rolled_back` 结束，并明确“未完成发布”。
- 若存在 `previousImageRef`，回滚动作将记录“应恢复的上一版本镜像引用”；后续真实发布执行器可直接消费。

这保证部署记录本身具备事务性：

- 成功记录一定是 `committed`
- 非成功记录一定显式标识失败与回滚
- 不会出现“镜像已推送但记录仍显示成功待定”的悬空状态

## ReviewerAgent 预检设计

预检阶段复用 Phase 4 的 Specialist/Reviewer 机制，输入包括：

- 工作区路径
- 目标环境
- 生成的 Dockerfile 摘要
- 部署命令计划
- 可选的环境变量键名列表

ReviewerAgent 输出要求：

- 第一行必须是 `PASS` 或 `FAIL`
- 后续最多 5 条发现

若返回 `FAIL`，部署立即终止，不进入构建阶段。

## Docker 交付物生成策略

扫描逻辑遵循最小假设：

- 若存在 `go.mod`，生成 Go 服务型 Dockerfile
- 若存在 `package.json` 且存在前端构建脚本，可扩展生成 Node 多阶段 Dockerfile
- 当前仓库以 Go 为主，首版先稳定支持 Go

Go 版 Dockerfile 要求：

- 使用多阶段构建
- 先复制 `go.mod` / `go.sum` 以利用缓存
- 通过 `go build` 生成单一可执行文件
- 运行镜像优先使用轻量基础镜像
- 暴露可配置工作目录与默认启动命令

`.dockerignore` 至少排除：

- `.git`
- `target`
- `node_modules`
- `.go-cache`
- `.agy-doctor`
- `docs/reviews`
- 临时文件与日志

## 记录文件设计

记录目录：

`<dataDir>/deployments/<deployment-id>.json`

建议字段：

- `id`
- `workspaceRoot`
- `environment`
- `imageRepository`
- `imageTag`
- `imageRef`
- `previousImageRef`
- `status`
- `reviewStatus`
- `buildCommand`
- `pushCommand`
- `artifacts`
- `failureStage`
- `failureReason`
- `createdAt`
- `updatedAt`

## 执行器抽象

工具层暴露统一接口：

- 生成部署计划
- 落盘初始记录
- 执行构建
- 执行推送
- 标记提交
- 标记回滚

首版执行器直接调用本地 `docker` CLI，后续可扩展到：

- 远端 BuildKit
- Registry API
- Kubernetes / ECS / Fly / Render 发布器

## 风险与权衡

- 优先保证部署记录一致性，而不是一次性覆盖所有平台。
- 首版只实现 Docker 构建与推送，不直接操作生产编排系统，降低误发布风险。
- ReviewerAgent 只做预检门禁，不替代真实运行时健康检查。
- 生成 Dockerfile 采用规则驱动，不做不可解释的隐式猜测。

## 开发落地顺序

1. 提交现有 `resume` CLI 改动
2. 实现 `internal/tools/deployment.go`
3. 接入 `agy deploy`
4. 编写 Dockerfile 生成测试
5. 执行构建与测试
