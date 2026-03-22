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

## 5. 内核管理 (Core Extraction)

### 5.1 提取路径
- **内核 (Engine)**: `/Applications/Antigravity.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_x64`
- **Ripgrep (rg)**: `/Applications/Antigravity.app/Contents/Resources/app/node_modules/@vscode/ripgrep/bin/rg`
- **FD**: `/Applications/Antigravity.app/Contents/Resources/app/extensions/antigravity/bin/fd`

### 5.2 本地签名
提取后的二进制文件会进行 Ad-hoc 签名 (`codesign --force --deep -s -`)，以确保在 macOS 环境下启动内核进程时性能符合预期，避免被系统拦截或产生卡顿。

### 5.3 版本追踪与架构探测
项目根目录的 `CORE_VERSION.json` 记录了当前提取组件的详细状态：
- **core**: 记录内核版本、二进制路径及探测到的指令集架构 (x86_64/arm64)。
- **tools**: 记录 `rg` 与 `fd` 的版本及其架构。
- **system**: 记录执行提取时的宿主机操作系统与硬件架构。
- **source_app**: 记录源官方应用的安装路径与具体版本。

该文件在每次运行 `make update-core` 时会自动更新，用于确保 Agent 运行环境的组件兼容性。
