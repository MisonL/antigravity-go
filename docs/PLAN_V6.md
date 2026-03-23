# Antigravity Go v0.1.5 - PLAN_V6

日期: 2026-03-23

## 0. 已验证基线

本轮已经确认以下事实成立：

- `agy init` 已实现，能够在空目录生成极简标准的 `Go Backend + React/Vite Frontend` 脚手架，并落地 `Makefile`、`README.md` 与基础工程文件。
- 在临时空目录实测执行 `agy init` 后，生成项目可执行 `GOCACHE=$(pwd)/.go-cache go build ./...`。
- 当前仓库执行 `GOCACHE=$(pwd)/.go-cache go build -o agy cmd/agy/*.go` 通过。
- 当前仓库执行 `GOCACHE=$(pwd)/.go-cache go test ./...` 通过。
- 当前前端执行 `npm run build` 通过，且已消除 Vite 的大 chunk warning。

## 1. CSE 总体诊断

### 1.1 当前控制拓扑

- Reference:
  产品目标、CLI 子命令、现有评审文档、工作区代码事实。
- Sensors:
  `get_validation_states`、`get_core_diagnostics`、`go test ./...`、WebSocket 日志、`/api/observability/summary`、轨迹与记忆查询。
- Controller:
  `agent.Agent`、Maker-Checker、Reviewer specialist、TUI/Web 交互层、`agy review`/`agy auto-fix`/`agy deploy`/`agy init`。
- Actuators:
  `apply_core_edit`、`write_file`、`rollback_to_step`、browser 工具、MCP 动态工具、部署记录写入。

### 1.2 结论

当前系统不是“没有闭环”，而是“局部闭环存在、全局闭环不完整”。

- Sensor 层已经具备基础观测能力，但传感器分散，缺少统一任务级状态面。
- Controller 层已经能做补丁、自检、预审和审批，但没有项目级任务账本和长期控制策略。
- Actuator 层覆盖代码、浏览器、MCP、回滚与部署，但没有形成一致的策略编排面。

### 1.3 当前单点故障

- `antigravity_core` 是明显单点故障。`internal/core/host.go` 只管理单个核心进程，Core 挂掉时 TUI、Web、Agent 与 MCP 全部失效。
- `WSServer` 是单控制面汇聚点。单个 WebSocket 中枢负责会话、审批、广播和观测事件，失效会导致 Web 交互面整体失联。
- LLM Provider 是单推理通道。当前没有 provider 级降级编排、熔断和任务重放机制。
- 前端静态资源同步链路存在分裂。`frontend/dist` 与 `internal/server/dist` 并存，且 `Makefile` 默认用 Bun，实际验证又走 NPM，发布口径不统一。

## 2. 第一性原理下的核心缺口

### 2.1 控制回路缺口

1. 缺少统一任务状态面。
   现状：任务目标、验证结果、轨迹、记忆、审批、部署记录分散在不同对象里。
   后果：系统知道“发生了什么”，但不知道“当前任务离验收差多少”。
   要求：引入任务对象，统一记录 reference、plan、evidence、status、rollback point。

2. 缺少 supervisor 级自恢复。
   现状：Core、WebSocket、Provider 失败后主要依赖人工重试。
   后果：控制器本身没有对 plant 做持续稳定控制。
   要求：引入 host watchdog、provider retry budget、Web 会话重连状态机。

3. 观测存在，但没有门禁级评分板。
   现状：有日志、摘要、validation、test output，但没有统一任务健康度、失败分类、时延与成功率面板。
   后果：难以判断系统是在收敛还是振荡。
   要求：建立 task health、tool latency、approval latency、rollback count、core reconnect count 等指标。

### 2.2 工程边界缺口

1. 多个核心文件已经明显失控。
   证据：
   `frontend/src/App.tsx` 1056 行。
   `frontend/src/index.css` 1290 行。
   `internal/tui/model.go` 721 行。
   `internal/server/server.go` 852 行。
   `internal/server/websocket.go` 624 行。
   后果：系统复杂度集中在少数超大文件中，局部修改会放大全局回归风险。

2. 文本与视觉输出违反仓库自身卫生约束。
   证据：TUI、Web、Server 日志存在大量 emoji 与装饰性符号；例如 `internal/tui/model.go`、`internal/tui/commands.go`、`frontend/src/App.tsx`、`internal/server/server.go`。
   后果：跨终端显示不稳定、国际化困难、审查噪音高。

3. 前端构建工具链口径不统一。
   证据：根 `Makefile` 使用 `bun install && bun run build`，但实际前端也维护了 `package-lock.json` 且本轮要求用 `npm run build`。
   后果：构建可重复性差，CI 和本地环境容易漂移。

## 3. 与 Claude Code / Codex 的功能差距

### 3.1 还缺的“杀手级”能力

1. 多 Agent 并行编排。
   当前只有 `ask_specialist` 的轻量专家模式，没有真正的并行 worker、依赖图拆解和结果汇总能力。

2. 明确的任务规划与执行账本。
   Claude Code/Codex 的强项之一是把计划、执行、验证串成可追踪链路；当前系统仍然更像“带工具的聊天界面”。

3. 统一的变更审阅面。
   当前虽然已有 Maker-Checker 和审批预览，但还没有完整的“计划 diff -> 实际 diff -> 风险 diff -> 回滚点”同屏工作流。

4. 真正的断点续跑。
   当前 `resume` 已能恢复轨迹和消息，但还不能完整恢复环境变量、命令上下文、局部工作树状态和未完成子任务图。

5. 自动策略分层。
   当前 provider、工具、审批和回滚逻辑已存在，但缺少按任务类型自动切换执行策略的“上层大脑”。

### 3.2 建议优先补齐的能力

- P0: 任务账本 + 任务级状态机。
- P1: 并行 worker + reviewer 汇总。
- P1: diff-centered 审批面与回滚点对照。
- P1: 可重放的 session snapshot。
- P2: 模板市场与项目类型选择器，而不是单一脚手架。

## 4. UI/UX 与美观度诊断

### 4.1 TUI

结论：可用，但不沉浸，也不够稳。

- 优点：
  已有 Bubble Tea 交互、slash command、审批等待与上下文压缩。
- 问题：
  `internal/tui/model.go` 与 `internal/tui/commands.go` 内大量中文提示与 emoji 混杂，视觉密度高但信息层级弱。
- 问题：
  没有稳定的“三栏式”任务视图，计划、diff、审批、日志、当前工具状态没有明确分区。
- 问题：
  授权提示仍偏文本弹窗式，缺少变更摘要、影响文件、可回滚点与验证结果的集中展示。

### 4.2 Web Dashboard

结论：功能比外观强，工程感足够，但产品感不足。

- `frontend/src/App.tsx` 承担了状态管理、WebSocket 协议、模态框调度、聊天区、欢迎页、配置页、反馈等过多职责。
- `frontend/src/index.css` 体量过大，说明视觉系统没有被组件化。
- 仍使用 `alert(...)` 作为关键交互反馈，不足以支撑专业工作台体验。
- 本轮虽然通过 `manualChunks` 消除了构建 warning，但 bundle 拆分仍是被动止损，不是系统性的前端架构收敛。
- 当前界面有明显“功能堆叠”感：轨迹、记忆、MCP、视觉自测、设置、聊天、终端、日志都在同一主界面竞争注意力。

### 4.3 UI/UX 行动项

- P0: 拆分 `App.tsx`，至少分出 shell、chat、observability、settings、session recovery 五个子域。
- P1: 引入 toast / notification center，替换 `alert`。
- P1: 建立 design tokens 与组件级 CSS 边界，压缩 `index.css`。
- P1: 重新设计 TUI 的审批、计划和执行三区布局。
- P2: 为 Web 控制台补齐响应式断点和低带宽模式。

## 5. 全球化与 i18n 诊断

结论：当前硬编码中文已经是国际化阻塞项，不是小问题。

### 5.1 事实

- `frontend/src/App.tsx`、各类 modal 组件、`internal/tui/model.go`、`internal/tui/commands.go`、`cmd/agy/*`、`internal/server/*` 中存在大量硬编码中文。
- 这些中文不仅在 UI 文本里，还出现在错误消息、日志、占位符、提示词和状态标签中。
- 当前没有 locale 文件、message key、语言切换、默认语言策略，也没有服务端与前端共享词条层。

### 5.2 风险

- 无法自然支持英文或多语言用户。
- CLI/TUI/Web 三个界面的文案会各自漂移，导致语义不一致。
- 文案与逻辑耦合，后续改文案会牵动测试和行为判断。

### 5.3 行动项

- P0: 建立统一文案层，前后端都改为 message key + locale bundle。
- P1: 默认提供 `zh-CN` 与 `en-US` 两套语言包。
- P1: 错误码与用户文案分离，日志保留稳定英文 code，界面层再本地化。
- P2: 对 prompt 模板和 specialist prompt 引入可切换语言策略。

## 6. 分阶段补完清单

### Phase 6A: 控制面收敛

- [DONE] 建立 `TaskRecord`，统一 reference、status、evidence、rollback point。
- [DONE] 为 Core Host 增加 watchdog、restart policy、health budget。
- [ ] 为 provider 增加错误分类、重试预算和 fail-fast 策略。
- [DONE] 建立统一 observability scoreboard，而不只是散点摘要。

### Phase 6B: 体验与工程拆分

- [DONE] 拆分 `frontend/src/App.tsx`。
- [DONE] 拆分 `frontend/src/index.css` 并建立 token 系统。
- [ ] 拆分 `internal/server/server.go` 与 `internal/server/websocket.go` 的协议职责。
- [ ] 重构 `internal/tui/model.go`，把输入、权限、渲染、命令总线拆开。

### Phase 6C: 能力升级

- [DONE] 引入多 worker 并行执行与 reviewer 汇总。
- [ ] 为 `resume` 增加命令历史、环境快照、工作树差异恢复。
- [DONE] 建立 diff-centered 审批工作流。
- [ ] 扩展 `agy init` 为模板选择器，而不是单模板输出。

### Phase 6D: 全球化与产品化

- [DONE] 建立 i18n 词条系统。
- [DONE] 清理 emoji 与装饰性输出，统一成纯文本与可翻译消息。
- [ ] 统一前端构建口径，选定 NPM 或 Bun，其余降为可选。
- [ ] 增加 CI 门禁，至少覆盖 `go build`、`go test ./...`、`frontend npm run build`。

### Phase 6E: 最终完美化优化

- [DONE] 修正部署模板中的 `ENTRYPOINT` / `command` 叠加问题。
- [DONE] 为前端引入非阻塞通知中心并补齐窄屏布局收口。
- [DONE] 完成最终文本卫生清理与交付同步。

## 7. 最终判断

Antigravity Go v0.1.5 目前已经具备“可工作的 Agent 工作台”雏形，但还没有成为“稳定、可扩展、全球化、具备产品级控制闭环的工程系统”。

最关键的事实不是功能少，而是：

- 局部闭环已经成立。
- 统一任务状态面已经具备基础形态，但 provider 策略分层仍待补齐。
- 单点故障仍然明显。
- UI 与 i18n 已完成第一轮产品化收口，但仍有继续打磨空间。

如果只继续叠加功能，系统会越来越像“能做很多事的实验台”；如果先补控制面、状态面、拆分边界与全球化，才有机会变成真正可与 Claude Code / Codex 正面竞争的产品。
