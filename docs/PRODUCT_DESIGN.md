# Antigravity Go (V0.1.0) - 产品设计

## 1. 产品定义

Antigravity Go 是一个利用 `antigravity_core` 作为感知引擎、由本地宿主程序实现任务编排的实验性 Agent 工作台原型。

## 2. 核心目标

- **机制探索**: 深入理解 `antigravity_core` 的 Connect RPC 协议与 Agent 工作流。
- **解耦感知层**: 验证内核是否可以脱离原本的 IDE 环境独立运行。
- **实现补丁系统**: 探索基于内核原生接口（`ApplyCodeEdit`）的稳健代码修改方案。

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
