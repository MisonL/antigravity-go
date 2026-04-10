# 前端工作区说明

## 技术基线
- 包管理与脚本执行：`Bun 1.3.9`
- 构建工具：`Vite 8`
- 视图库：`React 19`
- 语言：`TypeScript 5.9`

## 界面基线
- 所有核心面板、徽标、按钮与表单控件统一使用 `0px` 圆角。
- Web 工作台采用“主工作台 + 侧边工作区”的布局结构。
- 生产构建产物会复制到 `../internal/server/dist/` 供后端嵌入。

## 常用命令
安装依赖：
```bash
bun install
```

启动开发服务器：
```bash
bun run dev
```

执行检查并构建：
```bash
bun run lint
bun run build
```

预览生产包：
```bash
bun run preview
```
