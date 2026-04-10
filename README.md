# Antigravity-Go（AGo）v0.2.1

## 1. 项目定位
Antigravity-Go（AGo）是一个基于 Go 实现的工业级智能体运行环境与本地工作台。系统通过集成 `antigravity_core` 提供的感知与执行能力，完成任务编排、闭环执行、验证回收与问题追溯。

## 2. 核心能力
- **内核宿主管理**：负责 `antigravity_core` 的启动、就绪探测、端口发现与日志回放。
- **显式兼容处理**：当旧版或缺失的内核 RPC 不可用时，系统返回可消费空态，不做静默伪造。
- **任务账本与可观测性**：统一提供任务摘要、运行轨迹、系统记忆、工具服务状态与回滚入口。
- **闭环执行**：支持“修改代码 -> 自动校验 -> 反馈修正”的工程闭环。
- **Web 工作台**：提供对话、文件树、代码查看、终端面板、执行记录与审批界面。

## 3. 支持的模型服务
系统支持以下主流模型接入方式，并允许在 Web 界面中动态切换：
- OpenAI 兼容接口（Chat v1 / Legacy）
- Anthropic 兼容接口
- Google Gemini
- Ollama / LM Studio 本地模型

## 4. 快速开始

### 4.1 提取内核
首次使用前需要从官方应用中提取内核组件：
```bash
make update-core
```
执行后会生成 `CORE_VERSION.json`，记录内核版本、架构和兼容性信息。

### 4.2 构建与运行
```bash
make build
make run
```
构建完成后会生成 `ago` 可执行文件。默认配置与数据位于 `~/.ago`。

### 4.3 前端开发
当前前端基线为 `Bun 1.3.9 + Vite 8 + React 19 + TypeScript 5.9`。
```bash
cd frontend && bun install
cd frontend && bun run lint
cd frontend && bun run build
```
`make build` 会先完成前端构建，再把产物同步到 `internal/server/dist` 供 Go 服务嵌入。

## 5. 相关文档
- [产品设计](docs/PRODUCT_DESIGN.md)：产品定位、目标与技术栈
- [技术手册](docs/TECHNICAL_DOCUMENTATION.md)：内核适配、接口事实与实现路径
- [审计记录](docs/reviews/)：阶段复核与验证证据
- [更新日志](CHANGELOG.md)：版本演进记录

## 6. 免责声明
本项目仅用于技术研究与本地工程辅助。由于依赖闭源内核协议，请勿将其直接用于处理高敏感机密代码。
