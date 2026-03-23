package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/corecap"
	"github.com/mison/antigravity-go/internal/rpc"
	"github.com/mison/antigravity-go/internal/session"
	"github.com/mison/antigravity-go/internal/tools"
)

// WSMessage represents incoming WebSocket message
type WSMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type webSession struct {
	agent         *agent.Agent
	rec           *session.Recorder
	workspaceRoot string

	pendingMu sync.Mutex
	pending   map[string]chan approvalDecision
}

type WSServer struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mutex     sync.Mutex
	writeMu   sync.Mutex // Mutex to ensure only one writer per connection set (simplified for now)
	baseAgent *agent.Agent
	client    *rpc.Client
	tm        *TerminalManager

	workspaceRoot    string
	defaultApprovals string
	sessionsRoot     string
	sessions         map[*websocket.Conn]*webSession
	protocol         *wsProtocolHandler
}

func NewWSServer(baseAgent *agent.Agent, client *rpc.Client, tm *TerminalManager, workspaceRoot string, approvals string, sessionsRoot string) *WSServer {
	ws := &WSServer{
		clients:          make(map[*websocket.Conn]bool),
		broadcast:        make(chan []byte),
		baseAgent:        baseAgent,
		client:           client,
		tm:               tm,
		workspaceRoot:    workspaceRoot,
		defaultApprovals: approvals,
		sessionsRoot:     sessionsRoot,
		sessions:         make(map[*websocket.Conn]*webSession),
	}
	ws.protocol = newWSProtocolHandler(ws)
	return ws
}

func (ws *WSServer) ReplaceToolsByPrefix(prefix string, replacements []tools.Tool) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	if ws.baseAgent != nil {
		ws.baseAgent.ReplaceToolsByPrefix(prefix, replacements)
	}
	for _, sess := range ws.sessions {
		if sess != nil && sess.agent != nil {
			sess.agent.ReplaceToolsByPrefix(prefix, replacements)
		}
	}
}

func (ws *WSServer) BroadcastObservabilityEvent(toolName string, status string, extra map[string]interface{}) {
	plane := classifyObservabilityPlane(toolName)
	if plane == "" {
		return
	}

	data := map[string]interface{}{
		"plane":     plane,
		"tool":      toolName,
		"status":    status,
		"message":   buildObservabilityMessage(plane, toolName, status),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for key, value := range extra {
		data[key] = value
	}

	ws.Broadcast(map[string]interface{}{
		"type": "observability_event",
		"data": data,
	})
}

func (ws *WSServer) HandleWS(w http.ResponseWriter, r *http.Request) {
	resumeTrajectoryID := strings.TrimSpace(r.URL.Query().Get("resume_trajectory"))
	var snapshot *session.TrajectorySnapshot
	if resumeTrajectoryID != "" {
		var err error
		snapshot, err = session.LoadTrajectorySnapshot(resumeTrajectoryID, corecap.NewTrajectoryManager(ws.client), ws.workspaceRoot)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return isAllowedLocalOrigin(r)
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WS upgrade error:", err)
		return
	}

	var sess *webSession
	if ws.baseAgent != nil {
		var err error
		sess, err = ws.newSession(snapshot)
		if err != nil {
			ws.sendJSON(conn, map[string]interface{}{
				"type":  "chat_error",
				"error": err.Error(),
			})
			conn.Close()
			return
		}
	}

	ws.mutex.Lock()
	ws.clients[conn] = true
	if sess != nil {
		sess.agent.SetPermissionFunc(ws.protocol.PermissionFunc(conn, ws.defaultApprovals))
		ws.sessions[conn] = sess
	}
	ws.mutex.Unlock()

	// Initial generic push
	ws.sendJSON(conn, map[string]interface{}{
		"type":    "info",
		"message": "已连接到 Antigravity 后端",
	})
	if resumeTrajectoryID != "" && snapshot != nil {
		ws.sendJSON(conn, map[string]interface{}{
			"type": "session_resumed",
			"data": map[string]string{
				"trajectory_id":  resumeTrajectoryID,
				"workspace_root": snapshot.WorkspaceRoot,
			},
		})
	}

	// Read loop - handle incoming messages
	go func() {
		defer func() {
			ws.mutex.Lock()
			delete(ws.clients, conn)
			sess := ws.sessions[conn]
			delete(ws.sessions, conn)
			ws.mutex.Unlock()
			if sess != nil {
				sess.cancelPendingApprovals("connection_closed")
			}
			if sess != nil && sess.rec != nil {
				_ = sess.rec.Close()
			}
			conn.Close()
		}()

		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg WSMessage
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				log.Printf("WS parse error: %v", err)
				continue
			}

			switch msg.Type {
			case "chat":
				ws.handleChat(conn, msg.Payload)
			case "feedback":
				if ws.client == nil {
					continue
				}
				var feedback map[string]interface{}
				if err := json.Unmarshal([]byte(msg.Payload), &feedback); err == nil {
					ws.client.RecordChatFeedback(feedback)
				}
			case "approval_response", "permission_response":
				ws.protocol.HandleApprovalResponse(conn, msg.Payload)
			}
		}
	}()
}

func (ws *WSServer) handleChat(conn *websocket.Conn, input string) {
	// Send "thinking" indicator
	ws.sendJSON(conn, map[string]interface{}{
		"type": "chat_start",
	})

	// Core V2 Telemetry: Record chat event
	if ws.client != nil {
		ws.client.RecordEvent(map[string]interface{}{
			"type":      "chat_sent",
			"interface": "web_dashboard",
			"timestamp": time.Now().Unix(),
		})
	}

	// Inject Terminal Broadcaster into context
	ctx := context.Background()
	if ws.tm != nil {
		ctx = context.WithValue(ctx, tools.TerminalOutputKey{}, func(data []byte) {
			ws.tm.Broadcast(data)
		})
	}
	// Inject File Change Callback
	ctx = context.WithValue(ctx, tools.FileChangeKey{}, func(path string) {
		relPath := ws.normalizeWorkspacePathForConn(conn, path)
		ws.Broadcast(map[string]string{
			"type": "file_change",
			"path": relPath,
		})
		if sess := ws.getSession(conn); sess != nil && sess.rec != nil {
			_ = sess.rec.Append("file_change", map[string]any{"path": relPath})
		}
	})

	sess := ws.getSession(conn)
	if sess == nil || sess.agent == nil {
		ws.sendJSON(conn, map[string]interface{}{
			"type":  "chat_error",
			"error": "Agent 未初始化（可能缺少 API Key？）",
		})
		return
	}

	if sess.rec != nil {
		_ = sess.rec.Append("user_message", map[string]any{"content": input})
	}

	var assistantBuf strings.Builder

	// Run Agent with streaming callback
	err := sess.agent.RunStream(ctx, input, func(chunk string, err error) {
		if err != nil {
			ws.sendJSON(conn, map[string]interface{}{
				"type":  "chat_error",
				"error": err.Error(),
			})
			return
		}
		if chunk != "" {
			assistantBuf.WriteString(chunk)
		}
		// Send chunk to this specific client
		ws.sendJSON(conn, map[string]interface{}{
			"type":  "chat_chunk",
			"chunk": chunk,
		})
	}, func(event, name, args, result string) {
		// Map event to WS message type
		wsType := "tool_" + event // tool_start, tool_end, tool_error
		ws.sendJSON(conn, map[string]interface{}{
			"type": wsType,
			"data": map[string]string{
				"name":   name,
				"args":   args,
				"result": result,
			},
		})

		if sess.rec != nil {
			_ = sess.rec.Append(wsType, map[string]string{
				"name":   name,
				"args":   args,
				"result": result,
			})
		}

		switch event {
		case "start":
			ws.BroadcastObservabilityEvent(name, "running", nil)
		case "end":
			ws.BroadcastObservabilityEvent(name, "completed", nil)
		case "error":
			ws.BroadcastObservabilityEvent(name, "error", map[string]interface{}{
				"error": result,
			})
		}
	})

	if err != nil {
		ws.sendJSON(conn, map[string]interface{}{
			"type":  "chat_error",
			"error": err.Error(),
		})
	} else {
		ws.sendJSON(conn, map[string]interface{}{
			"type": "chat_done",
		})
	}

	if sess.rec != nil {
		_ = sess.rec.Append("assistant_message", map[string]any{"content": assistantBuf.String()})
		_ = sess.rec.SaveMessages(sess.agent.SnapshotMessages())
	}
}

func (ws *WSServer) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}

	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()

	for client := range ws.clients {
		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Printf("WS write error: %v", err)
			client.Close()
			delete(ws.clients, client)
		}
	}
}

func (ws *WSServer) sendJSON(conn *websocket.Conn, v interface{}) {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	conn.WriteJSON(v)
}

func (ws *WSServer) getSession(conn *websocket.Conn) *webSession {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	return ws.sessions[conn]
}

func (ws *WSServer) newSession(snapshot *session.TrajectorySnapshot) (*webSession, error) {
	agentForConn := ws.baseAgent.CloneWithPrompt(ws.baseAgent.GetSystemPrompt())
	agentForConn.RegisterTool(agentForConn.GetSpecialistTool())

	workspaceRoot := ws.workspaceRoot
	if snapshot != nil && strings.TrimSpace(snapshot.WorkspaceRoot) != "" {
		workspaceRoot = snapshot.WorkspaceRoot
	}
	var tracker session.WorkspaceTracker
	if ws.client != nil {
		tracker = corecap.NewWorkspaceManager(ws.client)
	}
	if snapshot != nil {
		if err := session.RestoreAgentFromSnapshot(agentForConn, tracker, snapshot); err != nil {
			return nil, err
		}
	}

	var rec *session.Recorder
	if ws.sessionsRoot != "" {
		r, err := session.New(ws.sessionsRoot, session.Metadata{
			WorkspaceRoot: workspaceRoot,
			Interface:     "web",
			Approvals:     ws.defaultApprovals,
		})
		if err == nil {
			rec = r
			if snapshot != nil {
				if err := rec.SaveMessages(agentForConn.SnapshotMessages()); err != nil {
					return nil, err
				}
			}
		}
	}

	return &webSession{
		agent:         agentForConn,
		rec:           rec,
		workspaceRoot: workspaceRoot,
		pending:       make(map[string]chan approvalDecision),
	}, nil
}

func (ws *WSServer) normalizeWorkspacePathForConn(conn *websocket.Conn, p string) string {
	root := ws.workspaceRoot
	if sess := ws.getSession(conn); sess != nil && strings.TrimSpace(sess.workspaceRoot) != "" {
		root = sess.workspaceRoot
	}
	return normalizeWorkspacePath(root, p)
}

func normalizeWorkspacePath(root string, p string) string {
	if p == "" || root == "" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}

func (s *webSession) cancelPendingApprovals(reason string) {
	if s == nil {
		return
	}

	s.pendingMu.Lock()
	pending := s.pending
	s.pending = make(map[string]chan approvalDecision)
	s.pendingMu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- approvalDecision{Allow: false, Reason: reason}:
		default:
		}
		close(ch)
	}
}

func (ws *WSServer) TotalTokenUsage() int {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	total := 0
	for _, sess := range ws.sessions {
		if sess == nil || sess.agent == nil {
			continue
		}
		total += sess.agent.GetTokenUsage()
	}
	return total
}

func classifyObservabilityPlane(toolName string) string {
	switch {
	case strings.HasPrefix(toolName, "trajectory_"), toolName == "rollback_to_step":
		return "trajectory"
	case strings.HasPrefix(toolName, "memory_"):
		return "memory"
	case strings.HasPrefix(toolName, "browser_"):
		return "visual"
	case toolName == "apply_core_edit", toolName == "edit_preview", toolName == "get_validation_states":
		return "workspace"
	default:
		return ""
	}
}

func buildObservabilityMessage(plane string, toolName string, status string) string {
	switch status {
	case "running":
		return fmt.Sprintf("%s plane 正在执行 %s", plane, toolName)
	case "error":
		return fmt.Sprintf("%s plane 执行 %s 失败", plane, toolName)
	default:
		return fmt.Sprintf("%s plane 已完成 %s", plane, toolName)
	}
}

func isAllowedLocalOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器场景允许（例如本机调试脚本），但仍要求来自本机回环
		return isLoopbackRemoteAddr(r.RemoteAddr)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
