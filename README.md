# Antigravity-Go (AGo) (v0.1.0)

## 1. 项目定位
Antigravity-Go (AGo) 是一个基于 Go 开发的工业级 Agent 运行环境与工作站。它通过集成感知层能力，实现任务编排、闭环执行与系统自愈，旨在为开发者提供稳定、极简的自动化辅助。

## 2. 核心架构与能力 (CSE Enabled)
- **内核宿主 (Host)**: 自动化管理 `antigravity_core` 生命周期，支持 RPC Heartbeat 主动探测、端口发现与日志回放。
- **工业级 RPC 鲁棒性**: 数据面接口对废弃或缺失的 Plane RPC 走显式兼容分支，Web 控制台可稳定返回空态而不是静默崩溃。
- **任务账本与可观测性**: 统一暴露任务摘要、轨迹、记忆、MCP 状态与回滚入口，便于产品化验收和问题追溯。
- **闭环执行 (Feedback Loop)**: 实现“应用补丁 -> 自动诊断 -> 决策修正”的闭环，Agent 具备自校正能力。
- **视觉与项目感知**: 适配内核原生视觉接口（Screenshot）与项目级元数据统计。

## 3. 多渠道 LLM 适配 (Universal Interface)
系统支持六种主流 AI 渠道，具备 Base URL 智能适配与零重启配置更新能力：
- **OpenAI 兼容 (Chat v1 / Legacy)**
- **Anthropic 兼容**
- **Google Gemini**
- **Ollama / LM Studio (本地私有化)**

## 4. 快速开始

### 核心提取
在使用前需要从官方应用中提取内核组件：
```bash
make update-core
```
该脚本会自动生成 `CORE_VERSION.json` 以记录内核版本、硬件架构及环境兼容性。

### 运行与配置
```bash
# 启动 Web 控制台 (默认端口 8888)
make run
```
执行 `make build` 后会生成 `ago` 可执行文件。配置与数据默认存储于 `~/.ago` 目录。您可以通过 Web UI 的“设置”入口动态调整 AI 渠道。

### 前端环境
当前前端基线为 `Bun 1.3.9 + Vite 8 + React 19 + TypeScript 5.9`。标准构建链路如下：
```bash
bun install
bun run build
```
`make build` 会先执行上述前端构建，再将产物同步到 `internal/server/dist` 供 Go 服务嵌入。

### Web UI 规范
- 控制台遵循 `Commander Paradigm 2.0`。
- 所有面板、按钮、徽标与输入框统一使用 `0px` 圆角设计语言。
- Data Plane / Control Plane / Observability 三类面板共享同一套状态反馈与空态组件。

如果您需要深入了解本项目的底层机制，请查阅以下文档：
- [产品设计 (Product Design)](docs/PRODUCT_DESIGN.md): 项目定义、核心目标与技术栈说明。
- [技术手册 (Technical Documentation)](docs/TECHNICAL_DOCUMENTATION.md): 内核研究成果、接口定义与具体的代码实现逻辑。
- [审计记录 (Audit Records)](docs/reviews/): 历次版本审计与结项报告。
- [更新日志 (Changelog)](CHANGELOG.md): 项目版本演进记录。

## 6. 免责声明
本项目仅用于技术研究。功能受限于闭源内核的协议稳定性，严禁用于处理机密代码。
