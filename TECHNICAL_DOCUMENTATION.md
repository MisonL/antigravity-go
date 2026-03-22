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

### 6.4 架构利用策略

1. **时滞抵消**: 利用 `Heartbeat` 接口替代传统的“日志监听”，确保 Agent 逻辑在内核索引完全加载后注入。
2. **误差修正闭环**: 将 `ApplyCodeEdit` 后的 `GetDiagnostics` 反馈结果自动追加至消息上下文，实现 Agent 的自愈式编程。
3. **环境归一化**: 通过 `CORE_VERSION.json` 抹平 x86_64 与 arm64 的物理架构扰动，确保内核驱动的稳定性。

## 7. 运维与升级
