package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/config"
	"github.com/mison/antigravity-go/internal/core"

	"github.com/mison/antigravity-go/internal/pkg/pathutil"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/session"
	"github.com/mison/antigravity-go/internal/tools"
)

type Server struct {
	port          int
	hostAddr      string
	authToken     string
	approvals     string
	cfgMu         sync.RWMutex
	cfg           config.Config
	workspaceRoot string
	sessionsRoot  string
	host          *core.Host
	agent         *agent.Agent
	client        *rpc.Client // Added rpc.Client
	lsp           *tools.LSPManager
	ws            *WSServer
	httpServer    *http.Server     // Kept for Start/Stop
	tm            *TerminalManager // Kept for HandleTerminalWS and NewWSServer
}

func NewServer(cfg *config.Config, host *core.Host, agt *agent.Agent, lsp *tools.LSPManager, client *rpc.Client) *Server {
	workspaceRoot, err := filepath.Abs(".")
	if err != nil {
		workspaceRoot = "."
	}

	serverCfg := config.DefaultConfig()
	if cfg != nil {
		serverCfg = cfg
	}

	sessionsRoot := ""
	if serverCfg.DataDir != "" {
		sessionsRoot = filepath.Join(serverCfg.DataDir, "sessions")
		_ = os.MkdirAll(sessionsRoot, 0755)
	}
	tm := NewTerminalManager()
	s := &Server{
		port:          serverCfg.WebPort,
		hostAddr:      serverCfg.WebHost,
		authToken:     serverCfg.AuthToken,
		approvals:     serverCfg.Approvals,
		cfg:           *serverCfg,
		workspaceRoot: workspaceRoot,
		sessionsRoot:  sessionsRoot,
		host:          host,
		agent:         agt,
		client:        client,
		lsp:           lsp,
		ws:            NewWSServer(agt, client, tm, workspaceRoot, serverCfg.Approvals, sessionsRoot), // Initialized WSServer with client
		tm:            tm,
	}

	// Wire up log broadcasting
	host.SetOnLog(func(line string) {
		s.ws.Broadcast(map[string]interface{}{
			"type": "log",
			"data": line,
		})
	})

	// Wire up indexing status broadcasting
	host.OnIndexStatus = func(status string) {
		s.ws.Broadcast(map[string]interface{}{
			"type":   "index_status",
			"status": status,
		})
	}

	return s
}

func (s *Server) Start() error {
	// 安全默认：仅允许回环地址监听，避免误暴露导致远程执行风险
	if s.hostAddr != "127.0.0.1" && s.hostAddr != "localhost" && s.hostAddr != "::1" {
		// Docker 场景：允许绑定 0.0.0.0/::，但必须开启 token 鉴权
		if s.hostAddr == "0.0.0.0" || s.hostAddr == "::" {
			if strings.TrimSpace(s.authToken) == "" {
				return fmt.Errorf("绑定到 %q 需要同时设置 --token（启用鉴权），否则会有远程执行风险", s.hostAddr)
			}
			log.Printf("⚠️ Web 控制台已绑定到 %s（已启用 token 鉴权），请勿在不可信网络暴露该端口", s.hostAddr)
		} else {
			return fmt.Errorf("出于安全原因，Web 控制台仅允许监听回环地址（127.0.0.1/localhost/::1），或在启用 --token 时监听 0.0.0.0/::，当前为 %q", s.hostAddr)
		}
	}

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/history", s.handleHistory)

	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionDetail)
	mux.HandleFunc("/ws", s.ws.HandleWS)

	// FS & LSP APIs
	mux.HandleFunc("/api/fs/tree", s.handleFSTree)
	mux.HandleFunc("/api/fs/content", s.handleFSContent)
	mux.HandleFunc("/api/lsp/hover", s.handleLSPHover)
	mux.HandleFunc("/api/lsp/symbols", s.handleLSPSymbols)

	mux.HandleFunc("/ws/term", s.HandleTerminalWS)

	// Static Assets (Embedded)
	mux.Handle("/", s.handleStatic())

	// Apply Middlewares
	handler := http.Handler(mux)
	if s.authToken != "" {
		handler = s.authMiddleware(handler)
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.hostAddr, s.port),
		Handler: handler,
	}

	log.Printf("🌐 Web Server 已监听：http://%s:%d", s.hostAddr, s.port)

	// Start status broadcaster
	go s.broadcastStatusLoop()

	return s.httpServer.ListenAndServe()
}

func (s *Server) handleStatic() http.Handler {
	assetFS := GetAssetsFS()
	fileServer := http.FileServer(assetFS)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if file exists in embedded FS
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to see if the file exists in dist
		f, err := assets.Open("dist" + path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) broadcastStatusLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tokenUsage := 0
		if s.ws != nil {
			tokenUsage = s.ws.TotalTokenUsage()
		} else if s.agent != nil {
			tokenUsage = s.agent.GetTokenUsage()
		}

		status := struct {
			Type       string `json:"type"`
			Ready      bool   `json:"ready"`
			CorePort   int    `json:"core_port"`
			TokenUsage int    `json:"token_usage"`
		}{
			Type:       "status",
			Ready:      s.host.IsReady(),
			CorePort:   s.host.HTTPPort(),
			TokenUsage: tokenUsage,
		}
		s.ws.Broadcast(status)
	}
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// ... broadcastStatusLoop ...

// Middleware
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow some paths? Or check all?
		// For now check all except pre-flight (handled by CORS)
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			// Check query param for WS
			token = r.URL.Query().Get("token")
		}

		// Simple prefix check
		expected := "Bearer " + s.authToken
		if s.authToken != "" && token != expected && token != s.authToken {
			http.Error(w, "Unauthorized (Invalid Token)", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handlers
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	tokenUsage := 0
	if s.agent != nil {
		tokenUsage = s.agent.GetTokenUsage()
	}

	status := map[string]interface{}{
		"ready":       s.host.IsReady(),
		"core_port":   s.host.HTTPPort(),
		"token_usage": tokenUsage,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 脱敏处理，不返回完整的 API Key
		displayCfg := *cfg
		if len(displayCfg.APIKey) > 8 {
			displayCfg.APIKey = displayCfg.APIKey[:4] + "..." + displayCfg.APIKey[len(displayCfg.APIKey)-4:]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(displayCfg)

	case http.MethodPost:
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// 权限护栏：更新配置并持久化
		cfg.Provider = newCfg.Provider
		cfg.Model = newCfg.Model
		cfg.BaseURL = newCfg.BaseURL

		// 只有在提供有效的新 Key 时才覆盖
		if newCfg.APIKey != "" && !strings.Contains(newCfg.APIKey, "...") {
			cfg.APIKey = newCfg.APIKey
		}

		if err := cfg.Save(); err != nil {
			http.Error(w, "failed to save config", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	// 兼容接口：返回指定 session 或最近一次 session 的消息
	w.Header().Set("Content-Type", "application/json")

	if s.sessionsRoot == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"messages": []interface{}{},
		})
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		metas, err := session.List(s.sessionsRoot)
		if err != nil || len(metas) == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":   "ok",
				"messages": []interface{}{},
			})
			return
		}
		id = metas[0].ID
	}

	rec, err := session.Load(s.sessionsRoot, id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"messages": []interface{}{},
		})
		return
	}
	defer rec.Close()

	msgs, err := rec.LoadMessages()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"messages": []interface{}{},
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"session_id": id,
		"messages":   msgs,
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.sessionsRoot == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]session.Metadata{})
		return
	}

	metas, err := session.List(s.sessionsRoot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metas)
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	// /api/sessions/{id}/messages
	prefix := "/api/sessions/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	id := parts[0]
	resource := parts[1]

	if s.sessionsRoot == "" {
		http.Error(w, "sessions disabled", http.StatusNotFound)
		return
	}

	switch resource {
	case "messages":
		rec, err := session.Load(s.sessionsRoot, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		defer rec.Close()
		msgs, err := rec.LoadMessages()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(msgs)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// --- FS Handlers ---

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "dir"
	Children []*FileNode `json:"children,omitempty"`
}

func (s *Server) handleFSTree(w http.ResponseWriter, r *http.Request) {
	rootPath := r.URL.Query().Get("path")
	if rootPath == "" {
		rootPath = "." // Default to CWD
	}
	maxDepth := 1
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if v, err := strconv.Atoi(depthStr); err == nil && v >= 0 {
			maxDepth = v
		}
	}

	absRoot, err := pathutil.SanitizePath(".", rootPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	tree, err := buildFileTree(s.workspaceRoot, absRoot, 0, maxDepth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func buildFileTree(workspaceRootAbs string, absPath string, depth int, maxDepth int) (*FileNode, error) {
	if depth > maxDepth {
		return nil, nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(workspaceRootAbs, absPath)
	if err != nil {
		relPath = absPath
	}
	relPath = filepath.ToSlash(relPath)

	node := &FileNode{
		Name: info.Name(),
		Path: relPath,
		Type: "file",
	}
	if info.IsDir() {
		node.Type = "dir"
		if depth >= maxDepth {
			return node, nil
		}

		entries, err := os.ReadDir(absPath)
		if err != nil {
			return node, nil
		}
		sort.Slice(entries, func(i, j int) bool {
			ai, aj := entries[i], entries[j]
			if ai.IsDir() != aj.IsDir() {
				return ai.IsDir()
			}
			return strings.ToLower(ai.Name()) < strings.ToLower(aj.Name())
		})

		for _, entry := range entries {
			name := entry.Name()
			if name == ".git" || name == ".gemini" || name == "node_modules" || name == "dist" || name == "build" || name == ".DS_Store" {
				continue
			}
			// 跳过前端依赖等大目录
			if name == "vendor" {
				continue
			}
			childAbs := filepath.Join(absPath, name)
			childNode, _ := buildFileTree(workspaceRootAbs, childAbs, depth+1, maxDepth)
			if childNode != nil {
				node.Children = append(node.Children, childNode)
			}
		}
	}
	return node, nil
}

func (s *Server) handleFSContent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}

		safePath, err := pathutil.SanitizePath(".", filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		content, err := os.ReadFile(safePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rel, err := filepath.Rel(s.workspaceRoot, safePath)
		if err != nil {
			rel = safePath
		}
		rel = filepath.ToSlash(rel)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path":    rel,
			"content": string(content),
		})
	case "POST":
		var req struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		safePath, err := pathutil.SanitizePath(".", req.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		if err := os.WriteFile(safePath, []byte(req.Content), 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		rel, err := filepath.Rel(s.workspaceRoot, safePath)
		if err != nil {
			rel = safePath
		}
		rel = filepath.ToSlash(rel)
		s.ws.Broadcast(map[string]string{
			"type": "file_change",
			"path": rel,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// BroadcastToTerminal sends a message to all active terminal sessions
func (s *Server) BroadcastToTerminal(msg string) {
	if s.tm != nil {
		s.tm.Broadcast([]byte(msg))
	}
}

// --- LSP Handlers ---

func (s *Server) handleLSPHover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File      string `json:"file"` // 相对工作区路径
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	absFile, err := pathutil.SanitizePath(".", req.File)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	res, err := s.lsp.Hover(r.Context(), absFile, req.Line, req.Character)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 将 LSP Hover 结构转成 Monaco 友好的 markdown 字段
	markdown := extractHoverMarkdown([]byte(res))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"markdown": markdown})
}

func (s *Server) handleLSPSymbols(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"` // 相对工作区路径
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	absFile, err := pathutil.SanitizePath(".", req.File)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	res, err := s.lsp.DocumentSymbols(r.Context(), absFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(res)) // Is JSON string
}

func extractHoverMarkdown(raw []byte) string {
	var v map[string]interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	contents, ok := v["contents"]
	if !ok {
		return ""
	}

	switch c := contents.(type) {
	case string:
		return c
	case map[string]interface{}:
		// MarkupContent: {kind,value}
		if value, ok := c["value"].(string); ok {
			return value
		}
	case []interface{}:
		var parts []string
		for _, item := range c {
			switch it := item.(type) {
			case string:
				parts = append(parts, it)
			case map[string]interface{}:
				// MarkedString: {language,value} or {value}
				if value, ok := it["value"].(string); ok {
					if lang, ok := it["language"].(string); ok && lang != "" {
						parts = append(parts, "```"+lang+"\n"+value+"\n```")
					} else {
						parts = append(parts, value)
					}
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}
