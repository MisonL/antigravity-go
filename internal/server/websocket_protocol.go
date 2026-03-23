package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mison/antigravity-go/internal/agent"
)

type wsProtocolHandler struct {
	server *WSServer
}

func newWSProtocolHandler(server *WSServer) *wsProtocolHandler {
	return &wsProtocolHandler{server: server}
}

func (handler *wsProtocolHandler) PermissionFunc(conn *websocket.Conn, mode string) agent.PermissionFunc {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "full":
		return func(req agent.PermissionRequest) bool { return true }
	case "read-only", "readonly":
		return func(req agent.PermissionRequest) bool { return false }
	default:
		return handler.promptPermissionFunc(conn)
	}
}

func (handler *wsProtocolHandler) promptPermissionFunc(conn *websocket.Conn) agent.PermissionFunc {
	return func(req agent.PermissionRequest) bool {
		sess := handler.server.getSession(conn)
		if sess == nil {
			return false
		}

		reqID := fmt.Sprintf("%d", time.Now().UnixNano())
		ch := make(chan approvalDecision, 1)
		handler.registerPendingApproval(sess, reqID, ch)
		handler.sendApprovalRequest(conn, reqID, req)

		select {
		case decision, ok := <-ch:
			handler.recordApprovalDecision(sess, reqID, req.ToolName, decision, ok)
			return ok && decision.Allow
		case <-time.After(approvalWaitTimeout):
			handler.handleApprovalTimeout(conn, sess, reqID, req.ToolName)
			return false
		}
	}
}

func (handler *wsProtocolHandler) registerPendingApproval(sess *webSession, reqID string, ch chan approvalDecision) {
	sess.pendingMu.Lock()
	defer sess.pendingMu.Unlock()
	sess.pending[reqID] = ch
}

func (handler *wsProtocolHandler) sendApprovalRequest(conn *websocket.Conn, reqID string, req agent.PermissionRequest) {
	payload := buildApprovalRequestPayload(req.ToolName, req.Args, handler.workspaceRootForConn(conn))
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

	handler.server.sendJSON(conn, map[string]any{
		"type": "approval_request",
		"data": payload,
	})
}

func (handler *wsProtocolHandler) workspaceRootForConn(conn *websocket.Conn) string {
	if sess := handler.server.getSession(conn); sess != nil && strings.TrimSpace(sess.workspaceRoot) != "" {
		return sess.workspaceRoot
	}
	return handler.server.workspaceRoot
}

func (handler *wsProtocolHandler) recordApprovalDecision(
	sess *webSession,
	reqID string,
	toolName string,
	decision approvalDecision,
	channelOpen bool,
) {
	if sess == nil || sess.rec == nil {
		return
	}

	record := map[string]any{
		"id":    reqID,
		"tool":  toolName,
		"allow": channelOpen && decision.Allow,
	}
	if decision.Reason != "" {
		record["reason"] = decision.Reason
	}
	if !channelOpen {
		record["reason"] = "channel_closed"
	}
	_ = sess.rec.Append("approval_decision", record)
}

func (handler *wsProtocolHandler) handleApprovalTimeout(
	conn *websocket.Conn,
	sess *webSession,
	reqID string,
	toolName string,
) {
	if sess == nil {
		return
	}

	sess.pendingMu.Lock()
	ch, ok := sess.pending[reqID]
	if ok {
		delete(sess.pending, reqID)
	}
	sess.pendingMu.Unlock()

	if !ok {
		return
	}

	select {
	case ch <- approvalDecision{Allow: false, Reason: "timeout"}:
	default:
	}
	close(ch)

	if sess.rec != nil {
		_ = sess.rec.Append("approval_decision", map[string]any{
			"id":     reqID,
			"tool":   toolName,
			"allow":  false,
			"reason": "timeout",
		})
	}

	handler.server.sendJSON(conn, map[string]any{
		"type": "approval_timeout",
		"data": map[string]string{"id": reqID},
	})
}

func (handler *wsProtocolHandler) HandleApprovalResponse(conn *websocket.Conn, payload string) {
	var resp approvalResponsePayload
	if err := json.Unmarshal([]byte(payload), &resp); err != nil || resp.ID == "" {
		return
	}

	sess := handler.server.getSession(conn)
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
