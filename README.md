# Antigravity Go (v0.1.0)

> [!CAUTION]
> **这是一个实验性项目。**
> 本项目旨在解构 Antigravity 核心 AI 功能的工作原理，实现了一个具备自愈能力和通用 LLM 适配的独立 Agent 运行环境原型。

## 1. 项目定位
Antigravity Go 是一个利用 `antigravity_core` 作为感知层、通过 Go 宿主进程实现任务编排的实验性 IDE 工作台。

## 2. 核心架构与能力 (CSE Enabled)
- **内核宿主 (Host)**: 自动化管理 `antigravity_core` 生命周期，支持 RPC Heartbeat 主动探测。
- **自愈式索引 (Hybrid Indexer)**: 具备架构自感知能力，支持高性能 `fd` 与原生 `Walk` 动态切换。
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
配置与数据默认存储于 `~/.agy_go` 目录。您可以通过 Web UI 的 **⚙️ 设置** 按钮动态调整 AI 渠道。

如果您需要深入了解本项目的底层机制，请查阅以下文档：
- [产品设计 (Product Design)](docs/PRODUCT_DESIGN.md): 项目定义、核心目标与技术栈说明。
- [技术手册 (Technical Documentation)](docs/TECHNICAL_DOCUMENTATION.md): 内核研究成果、接口定义与具体的代码实现逻辑。

## 6. 免责声明
本项目仅用于技术研究。功能受限于闭源内核的协议稳定性，严禁用于处理机密代码。
