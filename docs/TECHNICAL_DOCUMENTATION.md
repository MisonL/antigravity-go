# Antigravity-Go（AGo）v0.2.1 技术手册

## 1. 核心架构：宿主与内核
本项目将 `antigravity_core` 作为感知基础设施，通过宿主程序 `Host` 管理其生命周期并调用其提供的服务。

### 1.1 宿主管理
- **启动协议**：通过 `stdin` 注入 TLV 格式的 Protobuf 启动元数据。
- **端口识别**：通过正则表达式从内核标准输出中提取 HTTP、HTTPS 与 LSP 监听端口。
- **就绪探测**：通过 TCP 探活与 `Heartbeat` RPC 双重确认内核已可用。

### 1.2 通信接口
宿主程序封装 `rpc.Client` 调用内核的 Connect RPC 服务：
- `/exa.language_server_pb.LanguageServerService/ApplyCodeEdit`：应用结构化代码补丁
- `/exa.language_server_pb.LanguageServerService/GetMcpServerStates`：读取内核管理的工具服务状态
- `/exa.language_server_pb.LanguageServerService/GetStaticExperimentStatus`：读取实验功能状态

## 2. 智能体运行机制

### 2.1 决策循环
智能体基于消息队列完成多轮对话：
1. 接收用户指令
2. 调用大模型进行推理
3. 根据模型返回的工具调用执行本地或内核工具
4. 将工具结果反馈给模型，形成下一轮决策输入

### 2.2 权限控制
系统提供显式审批策略：
- `read-only`：拒绝 `write_file`、`run_command` 等修改操作
- `prompt`：等待 TUI 或 Web 端实时授权
- `full`：自动批准所有请求

## 3. 工具集成

### 3.1 搜索与索引
- `ripgrep (rg)`：用于 `search_files` 等仓库检索能力
- `fd`：用于启动阶段的并发索引扫描

### 3.2 补丁系统
- `apply_core_edit`：把大模型生成的补丁建议转化为内核原生 `ApplyCodeEditRequest` 后提交执行

## 4. 会话与存储
- **会话目录**：`~/.ago/sessions/<session_id>/`
- **关键文件**：
  - `meta.json`：会话元信息
  - `events.jsonl`：结构化事件流
  - `messages.json`：对话上下文

## 5. 内核能力适配与接口事实
本项目通过对 `antigravity_core` RPC 链路的探测与宿主适配，识别并接入以下关键能力：

### 5.1 已识别接口
| 接口名称 | 核心职责 | 项目利用方式 |
| :--- | :--- | :--- |
| `/Heartbeat` | 存活与就绪检查 | 用于 `WaitReady` 阶段的应用层就绪判定 |
| `/ApplyCodeEdit` | 结构化代码补丁应用 | 支撑 `apply_core_edit` 多文件原子修改 |
| `/GetDiagnostics` | 项目级编译与 Lint 错误拉取 | 在补丁应用后形成自动诊断反馈 |
| `/CaptureScreenshot` | 浏览器快照采集 | 支撑 Web 工作台视觉检查能力 |
| `/GetRepoInfos` | 仓库元数据与索引洞察 | 增强智能体项目级认知 |
| `/GetMcpServerStates` | 工具服务状态查询 | 动态挂载外部能力簇 |
| `/GetStaticExperimentStatus` | 内核功能状态审计 | 在 `ago doctor` 中展示实验功能状态 |

### 5.2 关键参数事实
- **启动参数**：内核支持 `--app_data_dir` 与 `--metadata_provider` 模式
- **ApplyCodeEdit**：接收 `edits` 数组，每个条目包含 `uri`、`new_text` 与 `range`
- **CaptureScreenshot**：依赖 `page_id` 指向特定浏览器上下文

### 5.3 关键实现路径

#### 1. 启动与就绪探测
- `internal/core/host.go` 的 `generateMetadata` 会构造特定 TLV 字节流，通过 `cmd.Stdin` 注入内核
- `WaitReady` 不只依赖 TCP 连通性，还会循环调用 `rpc.Client.Heartbeat()`，防止在索引未加载完成时提前放行
- `internal/index/indexer.go` 会先执行 `fd --version` 测试；若检测到架构不兼容，则显式回退到 `filepath.WalkDir` 扫描

#### 2. 代码补丁与自动诊断
- `internal/tools/core_v2.go` 将补丁请求封装为 `ApplyCodeEditRequest` 并通过 Connect RPC 发送
- `internal/agent/agent.go` 在补丁成功后会追加诊断提示，引导下一轮拉取 `get_core_diagnostics`
- `GetCoreDiagnosticsTool` 直接调用 `/GetDiagnostics`，把返回的错误列表格式化为模型可消费文本

#### 3. 模型服务工厂
- `internal/llm/factory.go` 的 `BuildProvider` 负责将前端配置映射到具体驱动实现
- 对 `ollama`、`lmstudio` 等本地模型，工厂会自动补齐默认地址，如 `http://localhost:11434/v1`

#### 4. 视觉数据处理
- `CaptureScreenshotTool` 接收 `page_id` 并调用内核 RPC 获取 Base64 图像数据
- 图像事件会被 `internal/session/recorder.go` 实时写入 `~/.ago/sessions/` 下的 `events.jsonl`

#### 5. 仓库元数据与索引洞察
- `GetRepoInfosTool` 通过 `/GetRepoInfos` 拉取项目宏观画像
- 返回结果会被解构为结构化文本，并注入智能体上下文，帮助其形成全局视角

#### 6. 工具服务动态集成
- `GetMcpStatesTool` 调用 `/GetMcpServerStates` 感知当前内核已挂载的所有工具服务
- 宿主程序把这些能力转化为模型可理解的工具定义，实现按需扩展

#### 7. 功能状态审计
- `ago doctor` 会调用 `/GetStaticExperimentStatus`
- 宿主程序会将复杂的实验功能键值映射为可读的启用状态，便于确认核心推理能力是否已激活

## 6. 运维与升级
- 升级内核后必须重新验证宿主启动参数、就绪探测和 RPC 兼容性
- 发布前至少执行 `go test ./...`、`go vet ./...`、`cd frontend && bun run lint && bun run build` 与 `make build`
- 若 Web 工作台发生明显行为变化，应同步更新 `docs/reviews/` 下的复核记录
