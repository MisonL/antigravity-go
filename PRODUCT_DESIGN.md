# Antigravity Go (Agentic IDE) 架构设计方案 (V2)

> 本文描述的是 **V2.0 目标架构**。当前仓库已实现 MVP 主要能力闭环（Core Host + Agent + Web Dashboard + 权限/会话），仍有少量 Roadmap 项（例如完整 MCP 管理、Docker 化）处于规划/待对齐阶段。

## 0. 产品定义

**Antigravity Go 是一个“高性能、全透明”的 AI 协作工作台：利用 `antigravity_core` 作为感知引擎，由自研 Hybrid Agent 及 Web Dashboard 提供顶层的推理、执行与交互体验。**

## 1. 产品愿景

**让“IDE 级智能 + Agent 级执行力”脱离 IDE，变成可脚本化、可审计、可部署的 CLI 工具。**

### 核心价值

- **可控**：所有工具调用、文件改动、命令执行都可审计，可设置权限与确认策略。
- **可组合**：以 MCP 作为插件协议，既能接入外部工具，也能输出为其它系统的能力模块。
- **可自动化**：既支持交互式 TUI，也支持非交互式批处理（JSON 输出、CI/CD）。

## 2. 核心使用场景（优先级）

1. **交互式编码**：在终端里完成“理解→计划→改码→验证→总结”的闭环。
2. **仓库级任务**：修 Bug、重构、补测试、生成文档、代码审查。
3. **流水线自动化**：在 CI 中生成修复补丁、生成变更说明、跑静态检查并给出建议。

## 3. 产品能力拆解（MVP → 完整体）

### 3.1 Core Host（必须）

负责把 `antigravity_core` 从“IDE 插件依赖”变成“可独立运行的引擎”：

- **启动编排**：参数组装、`stdin` 注入 Protobuf metadata（已知最小可用 `0a027b7d`）、日志收集与就绪探测。
- **端口与协议管理**：识别 `42100/42101/HTTP(随机)` 等监听端口，选择合适的调用面（Connect RPC / LSP TCP）。
- **隔离的运行目录**：独立的 `--gemini_dir`（避免污染用户全局目录），内置可迁移的配置与会话存储。

### 3.2 Agent 运行时（MVP）

基于 `antigravity_core` 内置的 **Cascade/Agent** 能力（从二进制可见大量 `Cascade*`、`RunToolRequest`、Browser overlay、MCP 管理器等模块）：

- **任务输入**：自然语言 + 选定文件/目录范围 + 约束（风格、风险、时限）。
- **轨迹/工件**：每次任务生成可回放的 trajectory（计划、工具调用、补丁、结果）。
- **流式输出**：边思考边输出，工具调用与补丁生成可实时展示。

### 3.3 工具系统（MVP：本地工具 + MCP）

两层工具策略：

1. **内置工具（最小集合）**：`read_file`、`write_file`、`search`、`run`、`git diff`、`apply_patch`、`open_url`（可选）。
2. **MCP 插件**：通过 MCP 暴露更复杂/敏感的能力（浏览器自动化、数据库、工单、云资源等）。

> 设计目标：让工具系统可扩展，但默认安全；默认启用的工具要“少而精”。

### 3.4 会话与可追溯性（MVP）

- **Session**：每次对话/任务一个 session，支持 `resume` 继续。
- **Artifacts**：补丁、diff、日志、报告、截图（若启用浏览器）统一落盘。
- **可审计**：工具调用记录、文件改动记录、命令执行记录。

### 3.5 安全与权限（MVP）

借鉴优秀 CLI Agent（例如 Codex CLI、Gemini CLI）中成熟的交互范式：

- **Approval Modes**：`read-only` / `prompt` / `full` 三档（默认 `prompt`）。
- **敏感信息保护**：默认隐藏 token、key、cookie；会话落盘与事件流会做基础脱敏（避免分享/审计时泄露密钥）。
- **Prompt Injection 防护**：将“网页内容/仓库文本/命令输出”与“系统指令”隔离；在 System Prompt 与工具输出层面建立最小信任边界。

## 4. CLI/TUI 交互设计（面向“编码任务”）

> 若后续补充 Web 控制台（调试/可视化），默认采用**亮色主题**，并以信息密度、可读性与性能为优先。

### 4.1 命令结构（建议）

- `agy`：交互式 TUI（默认）
- `agy run "<任务描述>" --dir . [--approvals read-only|full]`：单次任务（非交互式可用；`prompt` 会退化为只读）
- `agy resume <session>`：恢复会话（会话落盘在 `~/.antigravity/sessions/<id>/`）
- `agy doctor`：环境自检（依赖、二进制、配置）
- `agy mcp list`：查看 Core 内部管理的 MCP 状态（其它 MCP 管理能力需 Core RPC 支持，当前版本可能返回 404）

### 4.2 Slash Commands（交互式）

借鉴 Codex CLI 的 slash command 模式（例如 `/model`、`/status`、`/compact`、`/approvals`），在 TUI 内提供：

- `/status`：core 状态、认证状态、当前工作区
- `/approvals`：切换权限策略
- `/compact`：压缩上下文并保留关键结论
- `/help`：显示命令与快捷键

## 5. 技术架构（高层）

```mermaid
graph TD
  User[用户] -->|TUI/CLI| Host[Antigravity Go Host]

  subgraph Local
    Host -->|spawn + metadata| Core[antigravity_core]
    Host -->|Connect RPC / LSP TCP| Core
    Host -->|MCP Server(s)| MCP[MCP Tools/Resources]
  end

  Core -->|HTTPS| CloudAPI[Google Cloud Code API]
```

### 关键设计取舍

- **不把 core 当“黑盒 LSP”**：优先使用其 Connect RPC 面（LanguageServerService）承载 Agent 能力；LSP TCP 用于兼容/补充。
- **不依赖 IDE**：用 MCP/本地工具补齐“执行环境”，把 IDE 扩展能力最小化/可替换。

## 6. 参考项目（我们要学什么）

> 这里只记录“可落地的设计模式”，不做无意义的功能堆叠。

- **OpenAI Codex CLI**：交互式 TUI、approval modes、可恢复会话、slash commands、MCP 集成、非交互式运行。
  - Repo: https://github.com/openai/codex
  - Docs: https://developers.openai.com/codex/cli
- **Gemini CLI**：内置工具 + MCP、非交互式 JSON 输出、可配置的目录上下文与忽略规则。
  - Repo: https://github.com/google-gemini/gemini-cli
  - Docs: https://google-gemini.github.io/gemini-cli/
- **Aider**：Git 驱动的补丁工作流（diff-first）、对话+提交的闭环体验。
  - Repo: https://github.com/Aider-AI/aider
- **OpenHands / SWE-agent**：面向任务的可配置运行方式（YAML/配置驱动）、可复现实验/评测思路。
  - OpenHands Repo: https://github.com/All-Hands-AI/OpenHands
  - SWE-agent Repo: https://github.com/SWE-agent/SWE-agent
- **MCP 协议**：标准化工具/资源接入与权限边界。
  - Spec/Repo: https://github.com/modelcontextprotocol/modelcontextprotocol

## 7. Roadmap（按“可验证里程碑”拆分）

### Phase 0：对齐真实能力（1-2 天）

- [ ] 固化 `antigravity_core` 的启动/就绪探测（端口、日志模式）。
- [ ] 确认可稳定调用的 Connect RPC：`GetStatus`、`GetUserStatus` 等。

### Phase 1：CLI Agent MVP（3-7 天）

- [ ] `agy`：交互式 TUI（最小可用：输入任务→输出计划→生成补丁→确认应用）。
- [ ] 本地工具：读/写/搜索/运行命令/Git diff（带权限确认）。
- [ ] Session 落盘与 `resume`。

### Phase 2：MCP 插件化（1-2 周）

- [ ] 内置 MCP Server（把本地工具以 MCP 形式提供给 core）。
- [ ] MCP 配置隔离（不写入用户全局 `~/.gemini/antigravity`）。

### Phase 3：可靠性与部署（持续）

- [~] Docker 化 + Docker Compose（已提供骨架与统一分组；受限于当前 core 仅提供 macOS 二进制，容器内运行需 Linux 版本 core）。
- [ ] 观测：结构化日志、指标、可选 pprof dump。

---

设计原则：**默认简单、默认安全、默认可审计；在此基础上追求极致体验与性能。**
