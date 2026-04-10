# 2026-04-10 全量复核报告

## 范围
- 重建嵌入式前端产物与 `ago` 二进制
- 重新执行前后端验证
- 在降级模式与内核就绪模式下复核 Web 工作台
- 通过自动化测试与源码复核检查 TUI 执行账本链路
- 复核宿主与内核兼容性、顶层分层以及界面体验

## 验证记录

| 命令 | 退出码 | 结果 |
| --- | --- | --- |
| `go test ./...` | 0 | 通过 |
| `go vet ./...` | 0 | 通过 |
| `cd frontend && bun run lint && bun run build` | 0 | 通过 |
| `make build` | 0 | 通过 |
| `git diff --check` | 0 | 通过 |
| `go test ./internal/core ./internal/server ./internal/tui ./cmd/ago -run 'Test.*(Execution|Startup|Recovery|Status|Server|Observability|Tasks|Render|Host|Wait|Restart)' -v` | 0 | 通过 |
| `./ago --web --no-tui --web-host 127.0.0.1 --port 8901 --data /tmp/ago-review3.pcQV1X` | 运行中 | 内核达到就绪状态，Web 服务可用 |
| `curl -s http://127.0.0.1:8901/api/status` | 0 | 返回 `{"core_port":50401,"ready":true,"token_usage":0}` |
| Chrome DevTools + Lighthouse（移动端） | 不适用 | 无障碍 95，最佳实践 100，SEO 82 |

## 本次修复

1. 宿主与内核兼容性漂移
   - 修复前，宿主传递 `--random_port=true`，而 `antigravity_core 1.22.2` 会以退出码 2 拒绝该参数。
   - 已将启动参数改为 `--http_server_port=0`：
     - [internal/core/watchdog.go](/Volumes/Work/code/antigravity-go/internal/core/watchdog.go#L33)
     - [scripts/run_core.sh](/Volumes/Work/code/antigravity-go/scripts/run_core.sh#L7)
   - 修复后证据：`/tmp/ago-review3.pcQV1X/core.log` 出现 HTTP/HTTPS 随机端口并打印 `initialized server successfully`。

2. 数据面可访问性回归
   - 隐藏的工作区抽屉不再在 DOM 中保留可聚焦控件。
   - 已调整 [frontend/src/App.tsx](/Volumes/Work/code/antigravity-go/frontend/src/App.tsx#L200) 的条件渲染逻辑。

3. 移动端头部裁切
   - 已在 [frontend/src/styles/layout.css](/Volumes/Work/code/antigravity-go/frontend/src/styles/layout.css#L591) 增加窄屏换行规则。
   - 结果：390px 宽度下顶部命令区域不再被硬裁切。

## 发现

### 警告
1. 终端面板本轮没有完成新的真实端到端交互复核。
   - [frontend/src/components/TerminalPanel.tsx](/Volumes/Work/code/antigravity-go/frontend/src/components/TerminalPanel.tsx#L42) 仍会把初始化失败收敛成统一的兜底提示，因此若未来再次回归，定位成本仍偏高。
   - 本轮已验证构建、接口和嵌入产物一致性，但由于会话中的 DevTools 传输异常，未能重新执行浏览器驱动的终端交互。

### 信息
1. SEO 问题主要集中在缺少 `meta description` 和 `robots.txt` 无效。
   - 对本地认证型工作台来说，这一风险较低。

2. 本轮未在真实模型服务上跑完整对话闭环。
   - 当前环境未配置 API 密钥，因此模型服务初始化按设计保持禁用。
   - 但 Web 工作台、路由、可观测性弹窗、文件树、代码查看器挂载以及宿主生命周期已完成验证。

3. 旧版 `COMMANDER PARADIGM` 对比度问题已不再适用。
   - 顶层界面文案已经改为面向产品的 `工作台`、`主工作台` 等名称。

## 架构复核说明
- 仓库仍保持清晰分层：宿主（`internal/core`）、协议客户端（`internal/rpc`）、会话账本（`internal/session`）、HTTP/WebSocket 服务（`internal/server`）、TUI（`internal/tui`）以及 React 领域与界面层（`frontend/src/domains`、`frontend/src/components`）。
- 当前最脆弱的边界仍是闭源内核兼容层。一个过期启动参数就可能导致整个运行时不可用，因此每次升级内核都必须补契约验证。
- 执行账本在 CLI、TUI、HTTP 与 Web 工作台之间复用了同一套数据模型，顶层方向正确。

## 界面体验复核说明
- 桌面端视觉语言保持一致：方角控件、克制的蓝色强调、低噪声面板以及清晰的信息层级。
- 移动端经过头部换行后明显好转，但窄屏下命令区仍偏密。
- 空态文案总体清晰，能够引导用户继续操作。
- 终端面板仍是当前数据面体验里的主要未闭环点。
