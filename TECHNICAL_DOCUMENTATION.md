# Antigravity Go (V0.1.0) - 技术手册

## 1. 核心架构: 宿主与内核

本项目将 `antigravity_core` 定位为感知基础设施，通过宿主程序 `Host` 管理其生命周期并调用其提供的服务。

### 1.1 宿主管理 (Core Host)
- **启动协议**: 通过 `stdin` 注入 TLV 格式的 Protobuf 启动元数据。
- **端口识别**: 通过正则表达式从内核的标准输出（stdout/stderr）中提取监听端口（HTTP/HTTPS/LSP）。
- **就绪探测**: 通过 TCP Dial 进行端口存活检测。

### 1.2 通信接口 (Connect RPC)
宿主程序封装了 `rpc.Client` 以便调用内核的 Connect RPC 服务：
- `/exa.language_server_pb.LanguageServerService/ApplyCodeEdit`: 应用代码补丁。
- `/exa.language_server_pb.LanguageServerService/GetMcpServerStates`: 查看内核管理的 MCP 插件。
- `/exa.language_server_pb.LanguageServerService/GetStaticExperimentStatus`: 获取实验性功能状态。

## 2. 运行时机制 (Agent)

### 2.1 决策循环 (Reasoning Loop)
Agent 基于消息队列实现多轮对话：
1. **输入**: 用户指令。
2. **推理**: 调用 LLM (OpenAI / Anthropic / Gemini / Ollama 等)。
3. **工具调用**: 根据 LLM 返回的 Tool Calls 执行本地或内核工具。
4. **反馈**: 将工具执行结果回传至 LLM。

### 2.2 权限控制 (Approvals)
系统提供显式的权限拦截逻辑：
- `read-only`: 拒绝 `write_file`、`run_command` 等修改操作。
- `prompt`: 等待 TUI 或 Web 端的用户实时授权。
- `full`: 自动批准所有请求。

## 3. 工具集成

### 3.1 搜索与索引
- **ripgrep (rg)**: 用于 `search_files` 工具。
- **fd**: 用于启动时的并发索引扫描 (`index.Indexer`)。

### 3.2 补丁系统
- **apply_core_edit**: 将 LLM 生成的改动建议转化为内核原生的 `ApplyCodeEditRequest` 格式并应用。

## 4. 存储与会话

- **会话目录**: `~/.agy_go/sessions/<session_id>/`。
- **存储文件**: 
  - `meta.json`: 会话元信息。
  - `events.jsonl`: 结构化事件流。
  - `messages.json`: LLM 对话上下文。

## 6. 内核能力研究与解构 (Core Research)

通过对 `antigravity_core` (v1.48.0) 的二进制解构与 RPC 链路探测，本项目识别并利用了以下核心能力：

### 6.1 RPC 通信协议
内核采用 **Connect RPC** (基于 HTTP/2 的 gRPC 兼容协议) 进行通信。宿主程序通过注入 TLV 格式元数据至 `stdin` 唤醒内核，随后内核开启 HTTP 服务。

### 6.2 已识别的关键接口 (Discovered APIs)

| 接口名称 | 核心职责 | 本项目利用方式 |
| :--- | :--- | :--- |
| `/Heartbeat` | 存活与就绪状态检查 | 用于 `WaitReady` 阶段的应用层就绪判定。 |
| `/ApplyCodeEdit` | 结构化代码补丁应用 | `apply_core_edit` 工具的核心，支持多文件原子修改。 |
| `/GetDiagnostics` | 项目级编译/Lint 错误拉取 | 实现 **CSE 闭环反馈**，在补丁应用后自动校验正确性。 |
| `/CaptureScreenshot` | 浏览器快照采集 | 赋能 Agent “视觉”能力，支持 Web UI 调试。 |
| `/GetRepoInfos` | 仓库元数据与索引洞察 | 用于 `get_repo_metadata` 工具，增强 Agent 的项目认知。 |
| `/GetMcpServerStates` | MCP 插件状态查询 | 动态获取并挂载外部工具能力簇。 |
| `/GetStaticExperimentStatus` | 实验性功能开关审计 | 在 `agy doctor` 中展示内核的隐藏特性（如 Cascade 引擎）。 |

### 6.3 关键参数探测

- **启动参数**: 内核支持 `--app_data_dir` (持久化目录) 和 `--metadata_provider` 模式。
- **ApplyCodeEdit 参数**: 接收 `edits` 数组，每个 unit 包含 `uri`、`new_text` 以及 `range` 信息。
- **CaptureScreenshot 参数**: 依赖 `page_id` 定位特定的浏览器 Context。

### 6.4 内核能力的工程化利用逻辑

本项目并非简单的内核透传工具，而是通过以下三种模式深度集成并增强了内核能力：

#### 1. 宿主屏蔽与环境自愈 (Abstraction & Self-healing)
- **原理**: 本项目将内核视为一个“不稳定的物理对象”，通过 `internal/core/Host` 建立隔离层。
- **实践**: 
    - 针对内核启动初期的索引加载压力，通过 **Heartbeat 探测环路** 实现“应用层延迟注入”，确保 Agent 逻辑在内核真正 Ready 后才开始工作。
    - 在探测到辅助工具（如 `fd`）架构不匹配时，宿主程序会自动执行 **能力降级 (Fallback)**，通过原生 Go 指令集模拟内核缺少的搜索能力，保证了感知层面的高可用。

#### 2. 执行器闭环化 (Actuator Closed-looping)
- **原理**: 内核原生的 `ApplyCodeEdit` 是开环的（只负责改，不负责查错）。本项目通过 **CSE 闭环理论** 对其进行了增强。
- **实践**: 
    - 系统封装了 `apply_core_edit` 复合工具。每当该工具被调用，系统会自动触发内核的 `/GetDiagnostics` 接口。
    - 如果诊断发现补丁导致了新的编译错误，系统会将错误信息作为“环境负反馈”直接喂给 Agent 的决策引擎，强制其进入 **“自校正循环 (Self-Correction Loop)”**，直到错误消除。

#### 3. 多协议归一化路由 (Protocol Normalization)
- **原理**: 内核虽然强大，但需要高规格的 LLM（如 GPT-4o/Claude 3.5）驱动其复杂的补丁逻辑。
- **实践**: 
    - 本项目通过 `internal/llm` 层建立了一套 **协议映射矩阵**。它能将通用的 OpenAI/Gemini 指令集精准转化为驱动内核所需的高精度逻辑参数。
    - 即使是 Ollama 等本地模型，在经过本项目的 Base URL 智能补全和协议对齐后，也能获得与官方环境一致的内核操控能力。

#### 4. 视觉感知集成 (Visual Feedback)
- **原理**: 利用内核探测到的浏览器快照能力，将“纯文本 Agent”升级为“具备视觉的 Agent”。
- **实践**: 
    - 通过 `browser_screenshot` 工具，Agent 可以主动向内核申请 UI 状态。快照数据不仅用于辅助决策，还会被持久化到 `~/.agy_go/sessions` 中，作为后期人类审计的重要依据。

## 7. 运维与升级
