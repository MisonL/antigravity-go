package main

import "fmt"

func buildInitFiles(profile initProjectProfile) map[string]string {
	files := map[string]string{
		".gitignore":                  initGitignoreTemplate(),
		"Makefile":                    initMakefileTemplate(profile),
		"README.md":                   initReadmeTemplate(profile),
		"go.mod":                      initGoModTemplate(profile),
		"cmd/server/main.go":          initServerMainTemplate(profile),
		"internal/backend/server.go":  initBackendTemplate(),
		"frontend/.gitignore":         "dist\nnode_modules\n",
		"frontend/index.html":         initFrontendHTMLTemplate(profile),
		"frontend/package.json":       initFrontendPackageTemplate(profile),
		"frontend/src/App.tsx":        initFrontendAppTemplate(profile),
		"frontend/src/index.css":      initFrontendCSSTemplate(),
		"frontend/src/main.tsx":       initFrontendMainTemplate(),
		"frontend/src/vite-env.d.ts":  "/// <reference types=\"vite/client\" />\n",
		"frontend/tsconfig.json":      initFrontendTSConfigTemplate(),
		"frontend/tsconfig.node.json": initFrontendTSConfigNodeTemplate(),
		"frontend/vite.config.ts":     initViteConfigTemplate(),
	}
	return files
}

func initGitignoreTemplate() string {
	return "bin/\nfrontend/dist/\nfrontend/node_modules/\n.go-cache/\n.DS_Store\n"
}

func initMakefileTemplate(profile initProjectProfile) string {
	return fmt.Sprintf(".PHONY: build backend-build frontend-build test dev clean\n\nAPP_NAME := %s\nFRONTEND_DIR := frontend\n\nbuild: frontend-build backend-build\n\nbackend-build:\n\tgo build -o bin/$(APP_NAME) ./cmd/server\n\nfrontend-build:\n\tcd $(FRONTEND_DIR) && npm install && npm run build\n\ntest:\n\tgo test ./...\n\ndev:\n\t@printf \"backend: go run ./cmd/server\\n\"\n\t@printf \"frontend: cd frontend && npm install && npm run dev\\n\"\n\nclean:\n\trm -rf bin frontend/dist\n", profile.ProjectSlug)
}

func initReadmeTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("# %s\n\n一个极简但标准的 Go Backend + React/Vite Frontend 起步模板。\n\n## 目录结构\n\n- `cmd/server`: 后端入口\n- `internal/backend`: 后端 HTTP 路由\n- `frontend`: React/Vite 前端\n- `Makefile`: 常用构建命令\n\n## 快速开始\n\n```bash\ngo build ./...\ncd frontend && npm install\ncd frontend && npm run build\n```\n\n## 开发命令\n\n```bash\nmake backend-build\nmake frontend-build\nmake test\n```\n\n## 默认接口\n\n- `GET /api/healthz`: 健康检查\n", profile.DisplayName)
}

func initGoModTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("module %s\n\ngo 1.24.0\n", profile.ModulePath)
}

func initServerMainTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("package main\n\nimport (\n\t\"log\"\n\t\"net/http\"\n\t\"os\"\n\n\t\"%s/internal/backend\"\n)\n\nfunc main() {\n\tport := os.Getenv(\"PORT\")\n\tif port == \"\" {\n\t\tport = \"8080\"\n\t}\n\n\taddr := \":\" + port\n\tlog.Printf(\"backend listening on %%s\", addr)\n\tif err := http.ListenAndServe(addr, backend.NewServer()); err != nil {\n\t\tlog.Fatal(err)\n\t}\n}\n", profile.ModulePath)
}

func initBackendTemplate() string {
	return "package backend\n\nimport (\n\t\"encoding/json\"\n\t\"net/http\"\n\t\"time\"\n)\n\ntype healthResponse struct {\n\tName      string `json:\"name\"`\n\tStatus    string `json:\"status\"`\n\tTimestamp string `json:\"timestamp\"`\n\tVersion   string `json:\"version\"`\n}\n\nfunc NewServer() http.Handler {\n\tmux := http.NewServeMux()\n\tmux.HandleFunc(\"/api/healthz\", func(w http.ResponseWriter, r *http.Request) {\n\t\tif r.Method != http.MethodGet {\n\t\t\tw.WriteHeader(http.StatusMethodNotAllowed)\n\t\t\treturn\n\t\t}\n\n\t\twriteJSON(w, http.StatusOK, healthResponse{\n\t\t\tName:      \"backend\",\n\t\t\tStatus:    \"ok\",\n\t\t\tTimestamp: time.Now().UTC().Format(time.RFC3339),\n\t\t\tVersion:   \"v0.1.0\",\n\t\t})\n\t})\n\tmux.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n\t\twriteJSON(w, http.StatusOK, map[string]string{\n\t\t\t\"message\": \"backend is ready\",\n\t\t\t\"hint\":    \"use GET /api/healthz\",\n\t\t})\n\t})\n\treturn mux\n}\n\nfunc writeJSON(w http.ResponseWriter, status int, payload any) {\n\tw.Header().Set(\"Content-Type\", \"application/json; charset=utf-8\")\n\tw.WriteHeader(status)\n\t_ = json.NewEncoder(w).Encode(payload)\n}\n"
}

func initFrontendPackageTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("{\n  \"name\": \"%s\",\n  \"private\": true,\n  \"version\": \"0.1.0\",\n  \"type\": \"module\",\n  \"scripts\": {\n    \"dev\": \"vite\",\n    \"build\": \"tsc && vite build\",\n    \"preview\": \"vite preview\"\n  },\n  \"dependencies\": {\n    \"react\": \"^19.2.0\",\n    \"react-dom\": \"^19.2.0\"\n  },\n  \"devDependencies\": {\n    \"@types/react\": \"^19.2.2\",\n    \"@types/react-dom\": \"^19.2.2\",\n    \"@vitejs/plugin-react\": \"^5.1.0\",\n    \"typescript\": \"^5.9.3\",\n    \"vite\": \"^7.3.1\"\n  }\n}\n", profile.FrontendName)
}

func initFrontendHTMLTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("<!doctype html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"UTF-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n    <title>%s</title>\n  </head>\n  <body>\n    <div id=\"root\"></div>\n    <script type=\"module\" src=\"/src/main.tsx\"></script>\n  </body>\n</html>\n", profile.DisplayName)
}

func initFrontendMainTemplate() string {
	return "import React from 'react';\nimport ReactDOM from 'react-dom/client';\nimport App from './App';\nimport './index.css';\n\nReactDOM.createRoot(document.getElementById('root')!).render(\n  <React.StrictMode>\n    <App />\n  </React.StrictMode>,\n);\n"
}

func initFrontendAppTemplate(profile initProjectProfile) string {
	return fmt.Sprintf("import { useEffect, useState } from 'react';\n\ntype HealthState = {\n  status: string;\n  timestamp: string;\n  version: string;\n};\n\nconst apiBaseURL = import.meta.env.VITE_API_BASE_URL?.trim() || 'http://localhost:8080';\n\nexport default function App() {\n  const [health, setHealth] = useState<HealthState | null>(null);\n  const [error, setError] = useState('');\n\n  useEffect(() => {\n    let cancelled = false;\n\n    async function loadHealth() {\n      try {\n        const response = await fetch(`${apiBaseURL}/api/healthz`);\n        if (!response.ok) {\n          throw new Error(`health check failed: ${response.status}`);\n        }\n        const payload = (await response.json()) as HealthState;\n        if (!cancelled) {\n          setHealth(payload);\n          setError('');\n        }\n      } catch (loadError) {\n        if (!cancelled) {\n          const message = loadError instanceof Error ? loadError.message : 'request failed';\n          setError(message);\n        }\n      }\n    }\n\n    void loadHealth();\n    return () => {\n      cancelled = true;\n    };\n  }, []);\n\n  return (\n    <main className=\"shell\">\n      <section className=\"hero\">\n        <p className=\"eyebrow\">Go Backend + React/Vite Frontend</p>\n        <h1>%s</h1>\n        <p className=\"lead\">\n          一个可直接扩展的全栈起步模板，默认暴露健康检查接口，并预置前后端构建命令。\n        </p>\n      </section>\n      <section className=\"grid\">\n        <article className=\"panel\">\n          <h2>Backend</h2>\n          <p>API Base URL</p>\n          <code>{apiBaseURL}</code>\n          <p>Health</p>\n          <strong>{health?.status ?? 'waiting'}</strong>\n          <p className=\"muted\">{health ? `${health.timestamp} · ${health.version}` : error || 'start the Go server to verify the loop'}</p>\n        </article>\n        <article className=\"panel\">\n          <h2>Commands</h2>\n          <ul>\n            <li><code>go build ./...</code></li>\n            <li><code>cd frontend && npm install</code></li>\n            <li><code>cd frontend && npm run build</code></li>\n          </ul>\n        </article>\n      </section>\n    </main>\n  );\n}\n", profile.DisplayName)
}

func initFrontendCSSTemplate() string {
	return ":root {\n  color: #102033;\n  background: #f4f7fb;\n  font-family: 'Segoe UI', sans-serif;\n}\n\n* {\n  box-sizing: border-box;\n}\n\nbody {\n  margin: 0;\n  min-height: 100vh;\n  background: linear-gradient(160deg, #f4f7fb 0%%, #e4ecf8 100%%);\n}\n\ncode {\n  font-family: 'SFMono-Regular', 'Consolas', monospace;\n}\n\n.shell {\n  min-height: 100vh;\n  padding: 48px 24px;\n}\n\n.hero {\n  max-width: 720px;\n  margin: 0 auto 32px;\n}\n\n.eyebrow {\n  margin: 0 0 12px;\n  font-size: 12px;\n  letter-spacing: 0.18em;\n  text-transform: uppercase;\n  color: #3f6db3;\n}\n\nh1 {\n  margin: 0;\n  font-size: clamp(2.5rem, 7vw, 4.5rem);\n}\n\n.lead {\n  max-width: 54ch;\n  font-size: 1.05rem;\n  line-height: 1.7;\n  color: #425466;\n}\n\n.grid {\n  max-width: 960px;\n  margin: 0 auto;\n  display: grid;\n  gap: 20px;\n  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));\n}\n\n.panel {\n  padding: 24px;\n  border: 1px solid rgba(40, 70, 110, 0.12);\n  border-radius: 20px;\n  background: rgba(255, 255, 255, 0.8);\n  box-shadow: 0 18px 60px rgba(16, 32, 51, 0.12);\n}\n\n.panel h2 {\n  margin-top: 0;\n}\n\n.panel ul {\n  margin: 0;\n  padding-left: 20px;\n  line-height: 1.8;\n}\n\n.muted {\n  color: #66788a;\n}\n"
}

func initFrontendTSConfigTemplate() string {
	return "{\n  \"compilerOptions\": {\n    \"target\": \"ES2020\",\n    \"useDefineForClassFields\": true,\n    \"lib\": [\"DOM\", \"DOM.Iterable\", \"ES2020\"],\n    \"allowJs\": false,\n    \"skipLibCheck\": true,\n    \"esModuleInterop\": true,\n    \"allowSyntheticDefaultImports\": true,\n    \"strict\": true,\n    \"forceConsistentCasingInFileNames\": true,\n    \"module\": \"ESNext\",\n    \"moduleResolution\": \"Node\",\n    \"resolveJsonModule\": true,\n    \"isolatedModules\": true,\n    \"noEmit\": true,\n    \"jsx\": \"react-jsx\"\n  },\n  \"include\": [\"src\"],\n  \"references\": [{ \"path\": \"./tsconfig.node.json\" }]\n}\n"
}

func initFrontendTSConfigNodeTemplate() string {
	return "{\n  \"compilerOptions\": {\n    \"composite\": true,\n    \"module\": \"ESNext\",\n    \"moduleResolution\": \"Node\",\n    \"allowSyntheticDefaultImports\": true\n  },\n  \"include\": [\"vite.config.ts\"]\n}\n"
}

func initViteConfigTemplate() string {
	return "import { defineConfig } from 'vite';\nimport react from '@vitejs/plugin-react';\n\nexport default defineConfig({\n  plugins: [react()],\n});\n"
}
