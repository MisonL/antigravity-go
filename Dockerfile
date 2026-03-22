FROM oven/bun:1.3.6 AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lockb ./
RUN bun install --frozen-lockfile
COPY frontend/ .
RUN bun run build

FROM golang:1.22 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist /tmp/frontend-dist
RUN rm -rf internal/server/dist && cp -r /tmp/frontend-dist internal/server/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/agy ./cmd/agy

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /workspace
COPY --from=backend /out/agy /usr/local/bin/agy

# 注意：容器里必须提供 Linux 可执行的 antigravity_core（本仓库自带的是 macOS 版本）。
# 运行时建议挂载工作区与 core 二进制：
#   -v "$PWD:/workspace" -w /workspace -e AGY_WEB_TOKEN=... -p 8888:8888
ENTRYPOINT ["agy"]
