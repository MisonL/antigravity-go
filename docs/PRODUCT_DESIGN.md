# Antigravity-Go (AGo) (V0.1.0) - 产品设计

## 1. 产品定义

Antigravity-Go (AGo) 是一个基于 Go 开发的工业级 Agent 运行环境与工作站，通过 `antigravity_core` 提供感知层能力，由本地宿主程序负责任务编排、闭环执行与系统自愈。

## 2. 核心目标

- **稳定宿主**: 提供可控的内核生命周期管理、任务执行与运行时隔离。
- **闭环执行**: 打通感知、决策、执行、验证和回滚的完整链路。
- **工程交付**: 为命令行、Web 控制台和部署场景提供一致的产品化入口。

## 3. 核心能力

### 3.1 宿主层 (Core Host)
负责内核的全生命周期管理：
- 启动编排与元数据注入（Stdin TLV 格式）。
- 端口自动发现（基于日志模式匹配）。
- 状态自检（WaitReady 与端口探测）。

### 3.2 代理运行时 (Agent Runtime)
实现基于外部 LLM（OpenAI/Gemini/iFlow）的决策循环：
- **工具调用**: 对接本地工具（读/写/搜索/运行命令）与内核 RPC。
- **权限管理**: 建立显式的工具审批策略（Approvals）。
- **上下文管理**: 实现摘要与修剪功能。

### 3.3 交互层 (Interfaces)
- **TUI**: 基于 Bubble Tea 的命令行对话界面。
- **Web**: 提供文件树预览、终端输出与实时对话功能。

## 4. 技术栈

- **后端**: Go 1.24+ (Gorilla WebSockets, Bubble Tea, Connect RPC)
- **前端**: React, Vite, TypeScript
- **外部依赖**: `antigravity_core` (二进制), `rg`, `fd`
