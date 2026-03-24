package tools

import (
	"fmt"
	"strings"
)

func dockerfileTemplate(profile WorkspaceProfile) string {
	lines := frontendStageLines(profile)
	lines = append(lines, goBuilderLines(profile)...)
	lines = append(lines, runtimeStageLines(profile.BinaryName)...)
	return strings.Join(lines, "\n") + "\n"
}

func frontendStageLines(profile WorkspaceProfile) []string {
	if !profile.HasFrontend {
		return nil
	}
	lockFile := "package-lock.json"
	installCmd := "npm ci"
	buildCmd := "npm run build"
	image := "node:22-bookworm-slim"
	if profile.UsesBun {
		lockFile = profile.BunLockFile
		if lockFile == "" {
			lockFile = "bun.lock"
		}
		installCmd = "bun install --frozen-lockfile"
		image = "oven/bun:1.3.6"
	}
	return []string{
		fmt.Sprintf("FROM %s AS frontend", image),
		"WORKDIR /src/frontend",
		fmt.Sprintf("COPY frontend/package.json frontend/%s ./", lockFile),
		fmt.Sprintf("RUN %s", installCmd),
		"COPY frontend/ ./",
		fmt.Sprintf("RUN %s", buildCmd),
		"",
	}
}

func goBuilderLines(profile WorkspaceProfile) []string {
	lines := []string{
		fmt.Sprintf("FROM golang:%s AS builder", profile.GoVersion),
		"WORKDIR /src",
		"COPY go.mod go.sum ./",
		"RUN go mod download",
		"COPY . .",
	}
	if profile.HasFrontend {
		lines = append(lines,
			"COPY --from=frontend /src/frontend/dist /tmp/frontend-dist",
			"RUN rm -rf internal/server/dist && mkdir -p internal/server && cp -r /tmp/frontend-dist internal/server/dist",
		)
	}
	lines = append(lines, fmt.Sprintf("RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/%s %s", profile.BinaryName, profile.GoMainPackage), "")
	return lines
}

func runtimeStageLines(binaryName string) []string {
	return []string{
		"FROM debian:bookworm-slim AS runtime",
		"RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates curl git ripgrep && rm -rf /var/lib/apt/lists/*",
		fmt.Sprintf("WORKDIR %s", defaultRuntimeWorkdir),
		"EXPOSE 8888",
		fmt.Sprintf("COPY --from=builder /out/%s /usr/local/bin/%s", binaryName, binaryName),
		"# 容器运行时仍需提供 Linux 版 antigravity_core 与目标工作区挂载。",
		fmt.Sprintf("ENTRYPOINT [\"%s\"]", binaryName),
	}
}

func dockerignoreTemplate() string {
	lines := []string{
		".git",
		".DS_Store",
		".ago-doctor",
		".go-cache",
		"target",
		"tmp",
		"tmp_build",
		"ago",
		"ago_data*",
		"frontend/node_modules",
		"frontend/dist",
		"internal/server/dist",
		"docs/reviews",
		"node_modules",
		"**/*.log",
		"**/*.tmp",
	}
	return strings.Join(lines, "\n") + "\n"
}

func githubActionTemplate(plan DeploymentPlan) string {
	registry := containerRegistryHost(plan.ImageRepository)
	tagLines := []string{
		fmt.Sprintf("            %s:${{ github.sha }}", plan.ImageRepository),
	}
	if strings.TrimSpace(plan.ImageTag) != "" {
		tagLines = append(tagLines, fmt.Sprintf("            %s:%s", plan.ImageRepository, plan.ImageTag))
	}

	lines := []string{
		"name: Deploy",
		"",
		"on:",
		"  push:",
		"    branches:",
		"      - main",
		"  workflow_dispatch:",
		"",
		"env:",
		fmt.Sprintf("  IMAGE_REPOSITORY: %s", plan.ImageRepository),
		fmt.Sprintf("  DEPLOY_ENVIRONMENT: %s", plan.Environment),
		"",
		"jobs:",
		"  deploy:",
		"    name: Build and push image",
		"    runs-on: ubuntu-latest",
		"    environment: ${{ env.DEPLOY_ENVIRONMENT }}",
		"    permissions:",
		"      contents: read",
		"      packages: write",
		"    steps:",
		"      - name: Checkout",
		"        uses: actions/checkout@v4",
		"      - name: Set up Docker Buildx",
		"        uses: docker/setup-buildx-action@v3",
		"      - name: Log in to GitHub Container Registry",
		"        if: startsWith(env.IMAGE_REPOSITORY, 'ghcr.io/')",
		"        uses: docker/login-action@v3",
		"        with:",
		"          registry: ghcr.io",
		"          username: ${{ github.actor }}",
		"          password: ${{ secrets.GITHUB_TOKEN }}",
		"      - name: Log in to container registry",
		"        if: ${{ !startsWith(env.IMAGE_REPOSITORY, 'ghcr.io/') }}",
		"        uses: docker/login-action@v3",
		"        with:",
		fmt.Sprintf("          registry: %s", registry),
		"          username: ${{ secrets.DOCKER_USERNAME }}",
		"          password: ${{ secrets.DOCKER_PASSWORD }}",
		"      - name: Build and push image",
		"        uses: docker/build-push-action@v6",
		"        with:",
		"          context: .",
		"          file: Dockerfile",
		"          push: true",
		"          tags: |",
	}
	lines = append(lines, tagLines...)
	return strings.Join(lines, "\n") + "\n"
}

func dockerComposeTemplate(plan DeploymentPlan) string {
	lines := []string{
		"services:",
		"  ago:",
		fmt.Sprintf("    image: %s", plan.ImageRef),
		"    build:",
		"      context: .",
		fmt.Sprintf("      dockerfile: %s", plan.DockerfilePath),
		"    restart: unless-stopped",
		fmt.Sprintf("    working_dir: %s", defaultRuntimeWorkdir),
		"    ports:",
		"      - \"8888:8888\"",
		"    volumes:",
		fmt.Sprintf("      - .:%s", defaultRuntimeWorkdir),
		"      - ago-data:/var/lib/ago",
		"      - ./deploy/runtime/antigravity_core:/opt/antigravity/bin/antigravity_core:ro",
		"    environment:",
		"      AGO_WEB_TOKEN: ${AGO_WEB_TOKEN:?set AGO_WEB_TOKEN before deployment}",
		"      OPENAI_API_KEY: ${OPENAI_API_KEY-}",
		"      GEMINI_API_KEY: ${GEMINI_API_KEY-}",
		"      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY-}",
		"    command:",
		"      - --web",
		"      - --no-tui",
		"      - --web-host",
		"      - 0.0.0.0",
		"      - --port",
		"      - \"8888\"",
		"      - --token",
		"      - ${AGO_WEB_TOKEN}",
		"      - --data",
		"      - /var/lib/ago",
		"      - --bin",
		"      - /opt/antigravity/bin/antigravity_core",
		"    healthcheck:",
		"      test:",
		"        - CMD-SHELL",
		"        - curl -fsS -H \"Authorization: Bearer $$AGO_WEB_TOKEN\" http://127.0.0.1:8888/api/status >/dev/null || exit 1",
		"      interval: 30s",
		"      timeout: 5s",
		"      retries: 5",
		"      start_period: 20s",
		"",
		"volumes:",
		"  ago-data: {}",
	}
	return strings.Join(lines, "\n") + "\n"
}

func containerRegistryHost(imageRepository string) string {
	parts := strings.Split(strings.TrimSpace(imageRepository), "/")
	if len(parts) == 0 {
		return "docker.io"
	}
	if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
		return parts[0]
	}
	return "docker.io"
}
