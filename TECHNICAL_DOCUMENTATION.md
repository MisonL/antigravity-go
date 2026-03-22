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

### 6.4 内核功能的具体代码利用实现

本项目在代码层面通过以下具体逻辑实现对内核能力的集成：

#### 1. 启动与就绪探测实现 (Host Logic)
- **输入注入**: `internal/core/host.go` 中的 `generateMetadata` 函数构造特定 TLV 字节流（`0x0a` + 长度 + JSON），通过 `cmd.Stdin` 注入内核以触发初始化。
- **双重探测环路**: `WaitReady` 函数不只依赖 TCP 连接，还会循环调用 `rpc.Client.Heartbeat()`。只有当内核返回 HTTP 200 且 RPC 路由响应成功时，宿主进程才释放阻塞，防止 Agent 在索引未加载完成时发起请求。
- **架构自适应**: `internal/index/indexer.go` 在执行前会先运行 `fd --version` 测试。若返回 `bad CPU type`（如 Intel Mac 运行 ARM 版 fd），系统会立即捕捉 `exec.Error` 并切换至 `scanProjectLegacy`（原生 `filepath.WalkDir` 递归），确保索引流程不中断。

#### 2. 代码补丁与自动诊断流 (Edit & Lint Flow)
- **原子补丁**: `internal/tools/core_v2.go` 将 LLM 生成的改动封装为 `ApplyCodeEditRequest`。该请求包含 `uri` 和 `TextEdit` 数组（含起始行列号），通过 Connect RPC 发送。
- **反馈注入逻辑**: 在 `internal/agent/agent.go` 的工具执行循环中，若 `apply_core_edit` 返回成功，程序会立即追加一个提示字符串。这会诱导 LLM 下一轮主动调用 `get_core_diagnostics`。
- **诊断集成**: `GetCoreDiagnosticsTool` 直接调用内核 `/GetDiagnostics` 接口，将返回的 JSON 错误列表（含错误行、信息、严重程度）转化为文本并喂回给 LLM 消息历史，实现错误的自动识别。

#### 3. LLM 协议转换工厂 (Provider Implementation)
- **多驱动映射**: `internal/llm/factory.go` 中的 `BuildProvider` 函数充当分发器。它将前端定义的 `ollama`、`lmstudio` 等标识符统一映射到 `OpenAIProvider` 驱动。
- **URL 自动重写**: 针对本地 Provider，工厂函数会自动检查并补全 Base URL。例如，若检测到 `provider == "ollama"` 且 URL 为空，则自动注入 `http://localhost:11434/v1`，确保底层驱动的 `http.Client` 能够直接通信。

#### 4. 视觉数据处理 (Screenshot Implementation)
- **快照采集**: `CaptureScreenshotTool` 接收 `page_id` 参数，调用内核 RPC 获取图片二进制数据的 Base64 编码。
- **会话持久化**: 采集到的 Base64 数据会被 `internal/session/recorder.go` 实时写入 `~/.agy_go/sessions/` 目录下的 `events.jsonl` 文件，确保 UI 调试过程可回溯。

## 7. 运维与升级
