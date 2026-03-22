# Antigravity Go - 旗舰级 Agentic IDE (V2)

> [!CAUTION]
> **这是一个实验性项目。**
> 本项目的主要目的是**深入研究与探索 Antigravity 核心 AI 功能的工作原理**。它通过解构内核 RPC 协议与 Agent 决策链路，实现了一个独立的 IDE 环境原型，不建议在生产环境或处理高度机密代码时使用。

Antigravity Go 是一个基于 `antigravity_core` 深度定制的旗舰级 Agentic 编码工作台。它集成了高性能内核、自研 Hybrid Agent Brain 以及 Premium 设计感的 Web 桌面。

> [!NOTE]
> 本项目已实现 V2 核心能力闭环（Core Host + Agent + Web Dashboard + 权限/会话）。Docker/Compose 已提供骨架，但容器内运行需要 **Linux 可执行的 `antigravity_core`**（本仓库自带的是 macOS 版本）。

## 🏗️ 架构说明

`antigravity-go` 采用了 **"强引擎 + 活大脑"** 的差异化架构：

1. **Engine (Antigravity Core V2)**:
   - 负责底层代码的高性能感知 (LSP over TCP)。
   - 提供基于 Connect RPC 的编排接口 (`LanguageServerService`)。
   - 实现原生的代码编辑补丁应用 (`ApplyCodeEdit`)。

2. **Brain (Hybrid Agent)**:
   - 独立于 Core 的自主推理引擎，支持双模式切换。
   - **Specialist Mode**: 针对架构、安全性、代码质量进行多专家会诊。
   - **Take-Itism Toolchain**: 深度集成并提取了 Core 自带的 `rg` (Ripgrep) 与 `fd`，实现毫秒级全局搜索与索引。

3. **User Interface (Web Dashboard)**:
   - 基于 React + Vite 的现代 Web 界面。
   - Premium 磨砂玻璃质感，集成 Full PTY 终端。

## 🎨 核心特性

- ✅ **LSP 深度集成**: 支持 Goto Definition, Hover, Symbols 等高级 IDE 功能。
- ✅ **高性能索引**: 基于 `fd` 的并发索引器，秒级扫描大型本地仓库。
- ✅ **原生编辑代理**: 代码改动由 Core V2 原生驱动，更稳健、更懂代码结构。
- ✅ **反馈闭环系统**: 实时 Thumb Up/Down 反馈，打通用户与模型对齐的最后一公里。
- ✅ **自动化运维**: 旗舰级 `make update-core` 脚本，保障内核工具实时更新。

## 🚀 快速开始

### 前置要求

- macOS (Intel/Apple Silicon)
- 已安装官方 Antigravity 客户端 (安装于 `/Applications/Antigravity.app`)
- Go 1.21+ / Bun (用于调试开发)

### 核心提取 (Core Extraction)

本项目依赖官方 Antigravity 客户端内置的内核引擎。在首次运行或官方客户端更新后，请执行以下命令提取最新内核：

```bash
# 自动从官方 App 中提取并签署内核 (antigravity_core, rg, fd)
make update-core
```

该脚本会自动完成：
1. 定位官方应用内的 `language_server_macos_x64`。
2. 提取并重命名为 `./antigravity_core`。
3. 对二进制文件进行 Ad-hoc 签名（防止 macOS 系统拦截）。
4. 同步高性能搜索工具 `rg` 与 `fd` 到 `./bin` 目录。

### 运行 (Production)

```bash
# 1. 自动同步内核与工具 (Take-Itism!)
make update-core

# 2. 启动旗舰级控制台
make run
```

浏览器访问 `http://127.0.0.1:8888` 即可进入工作台（默认仅监听本机回环地址，避免误暴露）。

> 若启用 `--token`（鉴权），请用 `http://127.0.0.1:8888/?token=你的token` 打开页面，前端会自动携带鉴权信息访问 `/api` 与 `/ws`。

### 🧠 LLM 配置（OpenAI 兼容 / iFlow）

默认 Provider 为 `openai`。如需对接 **OpenAI 兼容接口**（可改 Base URL / Model），可在 `~/.antigravity/config.yaml` 配置：

```yaml
provider: openai      # openai / gemini / iflow
model: gpt-4o         # 不同渠道的模型名不同
base_url: ""          # OpenAI 兼容接口 Base URL（例如 https://apis.iflow.cn/v1）
api_key: ""           # 可选；更推荐用环境变量
max_output_tokens: 0  # 最大输出 token（0 表示使用默认/自动适配）
```

常用环境变量：

- OpenAI：`OPENAI_API_KEY`，可选 `OPENAI_BASE_URL`（或 `OPENAI_API_BASE`）
- iFlow（预置渠道）：`IFLOW_API_KEY`，可选 `IFLOW_BASE_URL`（默认 `https://apis.iflow.cn/v1`）

也可用 CLI 覆盖（示例）：

```bash
./agy --provider openai --base-url https://apis.iflow.cn/v1 --model tstars2.0
```

查看 iFlow 官方模型清单（含最大上下文/最大输出）：

```bash
./agy models --provider iflow
```

### 常用命令

```bash
# 环境自检
./agy doctor

# 非交互式执行一次任务（建议明确 approvals）
./agy run "请总结项目结构并指出潜在风险" --approvals read-only

# 恢复某次会话（会话 ID 会在运行时输出）
./agy resume <session_id>

# 查看 Core 管理的 MCP 状态
./agy mcp list
```

## 🐳 Docker（可选）

> 受限于 Core 目前是 macOS 二进制，Docker 只能在你提供 **Linux 版本 `antigravity_core`** 时才能真正跑起来。

```bash
# 1) 准备 token
export AGY_WEB_TOKEN="请换成强随机字符串"

# 2) 启动
docker compose up --build

# 3) 打开（带 token）
# http://127.0.0.1:8888/?token=你的token
```

## 📡 更多文档

- `TECHNICAL_DOCUMENTATION.md`: 深入解析 Core RPC、Web 协议与会话/权限机制。
- `PRODUCT_DESIGN.md`: 架构设计哲学与演进路线。

## 📄 许可证

MIT License

## 🫡 汇报老板

本项目由您的专职 AI 员工开发完毕，欢迎使用！🚀
