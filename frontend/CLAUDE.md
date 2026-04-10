---
description: 在前端目录中优先使用 Bun，而不是 Node.js、npm、pnpm 或其他脚本入口。
globs: "*.ts, *.tsx, *.html, *.css, *.js, *.jsx, package.json"
alwaysApply: false
---

默认优先使用 Bun，而不是 Node.js。

- 使用 `bun <file>` 代替 `node <file>` 或 `ts-node <file>`
- 使用 `bun test` 代替 `jest` 或 `vitest`
- 使用 `bun build <file.html|file.ts|file.css>` 代替 `webpack` 或 `esbuild`
- 使用 `bun install` 代替 `npm install`、`yarn install` 或 `pnpm install`
- 使用 `bun run <script>` 代替 `npm run <script>`、`yarn run <script>` 或 `pnpm run <script>`
- 使用 `bunx <package> <command>` 代替 `npx <package> <command>`
- Bun 会自动加载 `.env`，因此不要额外引入 `dotenv`

## API 约定

- `Bun.serve()` 原生支持 WebSocket、HTTPS 和路由，默认不要引入 `express`
- SQLite 优先使用 `bun:sqlite`，不要默认引入 `better-sqlite3`
- Redis 优先使用 `Bun.redis`，不要默认引入 `ioredis`
- PostgreSQL 优先使用 `Bun.sql`，不要默认引入 `pg` 或 `postgres.js`
- `WebSocket` 为内建能力，默认不要引入 `ws`
- 文件读写优先使用 `Bun.file`，而不是直接依赖 `node:fs` 的 `readFile` / `writeFile`
- Shell 执行优先使用 `Bun.$\`...\``，而不是 `execa`

## 测试

使用 `bun test` 执行测试。

```ts
import { test, expect } from "bun:test";

test("hello world", () => {
  expect(1).toBe(1);
});
```

## 前端开发约定

若使用 Bun 自带静态资源服务，可直接让 HTML 文件导入 React、CSS 与脚本资源；不要在这种场景下再额外引入 Vite。

服务端示例：

```ts
import index from "./index.html";

Bun.serve({
  routes: {
    "/": index,
    "/api/users/:id": {
      GET: (req) => {
        return new Response(JSON.stringify({ id: req.params.id }));
      },
    },
  },
  websocket: {
    open: (ws) => {
      ws.send("Hello, world!");
    },
    message: (ws, message) => {
      ws.send(message);
    },
    close: () => {
      // 关闭处理
    },
  },
  development: {
    hmr: true,
    console: true,
  },
});
```

HTML 文件可以直接导入 `.tsx`、`.jsx` 或 `.js`；Bun 会自动完成转译与打包。`<link>` 标签可直接引用样式表，Bun 的 CSS 打包器会一并处理。
