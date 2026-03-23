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
	agent *agent.Agent
	rec   *session.Recorder

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
}

func NewWSServer(baseAgent *agent.Agent, client *rpc.Client, tm *TerminalManager, workspaceRoot string, approvals string, sessionsRoot string) *WSServer {
	return &WSServer{
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

	ws.mutex.Lock()
	ws.clients[conn] = true
	if ws.baseAgent != nil {
		agentForConn := ws.baseAgent.CloneWithPrompt(ws.baseAgent.GetSystemPrompt())
		agentForConn.RegisterTool(agentForConn.GetSpecialistTool())
		agentForConn.SetPermissionFunc(ws.permissionFuncForConn(conn, ws.defaultApprovals))

		var rec *session.Recorder
		if ws.sessionsRoot != "" {
			r, err := session.New(ws.sessionsRoot, session.Metadata{
				WorkspaceRoot: ws.workspaceRoot,
				Interface:     "web",
				Approvals:     ws.defaultApprovals,
			})
			if err == nil {
				rec = r
			}
		}

		ws.sessions[conn] = &webSession{
			agent:   agentForConn,
			rec:     rec,
			pending: make(map[string]chan approvalDecision),
		}
	}
	ws.mutex.Unlock()

	// Initial generic push
	ws.sendJSON(conn, map[string]interface{}{
		"type":    "info",
		"message": "已连接到 Antigravity 后端",
	})

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
				ws.handleApprovalResponse(conn, msg.Payload)
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
		relPath := ws.normalizeWorkspacePath(path)
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

func (ws *WSServer) normalizeWorkspacePath(p string) string {
	if p == "" || ws.workspaceRoot == "" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(ws.workspaceRoot, abs)
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

func (ws *WSServer) permissionFuncForConn(conn *websocket.Conn, mode string) agent.PermissionFunc {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "full":
		return func(req agent.PermissionRequest) bool { return true }
	case "read-only", "readonly":
		return func(req agent.PermissionRequest) bool { return false }
	default:
		// prompt
		return func(req agent.PermissionRequest) bool {
			sess := ws.getSession(conn)
			if sess == nil {
				return false
			}

			reqID := fmt.Sprintf("%d", time.Now().UnixNano())
			ch := make(chan approvalDecision, 1)

			sess.pendingMu.Lock()
			sess.pending[reqID] = ch
			sess.pendingMu.Unlock()

			payload := buildApprovalRequestPayload(req.ToolName, req.Args, ws.workspaceRoot)
			if strings.TrimSpace(req.Summary) != "" {
				payload.Summary = req.Summary
			}
			if strings.TrimSpace(req.Preview) != "" {
				if strings.TrimSpace(payload.Preview) != "" {
					payload.Preview = payload.Preview + "\n\n" + req.Preview
				} else {
					payload.Preview = req.Preview
				}
			}
			if len(req.Metadata) > 0 {
				if payload.Metadata == nil {
					payload.Metadata = make(map[string]any, len(req.Metadata))
				}
				for key, value := range req.Metadata {
					payload.Metadata[key] = value
				}
			}
			payload.ID = reqID
			ws.sendJSON(conn, map[string]interface{}{
				"type": "approval_request",
				"data": payload,
			})

			select {
			case decision, ok := <-ch:
				allow := ok && decision.Allow
				if sess.rec != nil {
					record := map[string]any{
						"id":    reqID,
						"tool":  req.ToolName,
						"allow": allow,
					}
					if decision.Reason != "" {
						record["reason"] = decision.Reason
					}
					if !ok {
						record["reason"] = "channel_closed"
					}
					_ = sess.rec.Append("approval_decision", record)
				}
				return allow
			case <-time.After(approvalWaitTimeout):
				// 超时需要清理 pending，避免泄漏并让前端弹窗可关闭
				sess.pendingMu.Lock()
				ch2, ok := sess.pending[reqID]
				if ok {
					delete(sess.pending, reqID)
				}
				sess.pendingMu.Unlock()
				if ok {
					select {
					case ch2 <- approvalDecision{Allow: false, Reason: "timeout"}:
					default:
					}
					close(ch2)
					if sess.rec != nil {
						_ = sess.rec.Append("approval_decision", map[string]any{
							"id":     reqID,
							"tool":   req.ToolName,
							"allow":  false,
							"reason": "timeout",
						})
					}
					ws.sendJSON(conn, map[string]interface{}{
						"type": "approval_timeout",
						"data": map[string]string{
							"id": reqID,
						},
					})
				}
				return false
			}
		}
	}
}

func (ws *WSServer) handleApprovalResponse(conn *websocket.Conn, payload string) {
	var resp approvalResponsePayload
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		return
	}
	if resp.ID == "" {
		return
	}

	sess := ws.getSession(conn)
	if sess == nil {
		return
	}

	sess.pendingMu.Lock()
	ch, ok := sess.pending[resp.ID]
	if ok {
		delete(sess.pending, resp.ID)
	}
	sess.pendingMu.Unlock()

	if !ok {
		return
	}
	ch <- approvalDecision{Allow: resp.Allow}
	close(ch)
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
