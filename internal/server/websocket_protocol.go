package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mison/antigravity-go/internal/agent"
	"github.com/mison/antigravity-go/internal/pkg/i18n"
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
		return func(req agent.PermissionRequest) agent.PermissionDecision {
			return agent.PermissionDecision{Allow: true}
		}
	case "read-only", "readonly":
		return func(req agent.PermissionRequest) agent.PermissionDecision {
			return agent.PermissionDecision{Allow: false}
		}
	default:
		return handler.promptPermissionFunc(conn)
	}
}

func (handler *wsProtocolHandler) promptPermissionFunc(conn *websocket.Conn) agent.PermissionFunc {
	return func(req agent.PermissionRequest) agent.PermissionDecision {
		sess := handler.server.getSession(conn)
		if sess == nil {
			return agent.PermissionDecision{Allow: false}
		}

		reqID := fmt.Sprintf("%d", time.Now().UnixNano())
		payload, plan := handler.buildApprovalPayload(conn, reqID, req)
		ch := make(chan approvalDecision, 1)
		handler.registerPendingApproval(sess, reqID, &pendingApproval{
			req:  req,
			plan: plan,
			ch:   ch,
		})
		handler.sendApprovalRequest(conn, payload)

		select {
		case decision, ok := <-ch:
			finalDecision := handler.finalizePermissionDecision(conn, sess, plan, decision, ok)
			handler.recordApprovalDecision(sess, reqID, req.ToolName, finalDecision, ok)
			return finalDecision
		case <-time.After(approvalWaitTimeout):
			handler.handleApprovalTimeout(conn, sess, reqID, req.ToolName)
			return agent.PermissionDecision{Allow: false, Reason: "timeout"}
		}
	}
}

func (handler *wsProtocolHandler) registerPendingApproval(sess *webSession, reqID string, pending *pendingApproval) {
	sess.pendingMu.Lock()
	defer sess.pendingMu.Unlock()
	sess.pending[reqID] = pending
}

func (handler *wsProtocolHandler) buildApprovalPayload(conn *websocket.Conn, reqID string, req agent.PermissionRequest) (approvalRequestPayload, *approvalExecutionPlan) {
	payload, plan := buildApprovalRequestPayload(handler.server.localeForConn(conn), req.ToolName, req.Args, handler.workspaceRootForConn(conn))
	if overridePlan := handler.buildApprovalPlanFromMetadata(conn, req); overridePlan != nil {
		plan = overridePlan
		attachApprovalChunks(i18n.MustLocalizer(handler.server.localeForConn(conn)), &payload, plan)
		if payload.Metadata == nil {
			payload.Metadata = make(map[string]any)
		}
		switch req.ToolName {
		case "apply_core_edit":
			payload.Metadata["file_path"] = normalizeApprovalPath(handler.workspaceRootForConn(conn), plan.targetPath)
		case "write_file":
			payload.Metadata["path"] = normalizeApprovalPath(handler.workspaceRootForConn(conn), plan.targetPath)
		}
	}
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
	return payload, plan
}

func (handler *wsProtocolHandler) buildApprovalPlanFromMetadata(conn *websocket.Conn, req agent.PermissionRequest) *approvalExecutionPlan {
	rawPath, ok := req.Metadata["approval_target_path"].(string)
	if !ok || strings.TrimSpace(rawPath) == "" {
		return nil
	}
	before, ok := req.Metadata["approval_before"].(string)
	if !ok {
		return nil
	}

	afterBytes, err := os.ReadFile(rawPath)
	if err != nil && !os.IsNotExist(err) {
		return nil
	}

	plan, err := buildApprovalExecutionPlan(
		i18n.MustLocalizer(handler.server.localeForConn(conn)),
		req.ToolName,
		rawPath,
		before,
		string(afterBytes),
	)
	if err != nil {
		return nil
	}
	return plan
}

func (handler *wsProtocolHandler) sendApprovalRequest(conn *websocket.Conn, payload approvalRequestPayload) {
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
	decision agent.PermissionDecision,
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
	if decision.Applied {
		record["applied"] = true
	}
	if len(decision.ApprovedChunkIDs) > 0 {
		record["approved_chunk_ids"] = decision.ApprovedChunkIDs
	}
	if strings.TrimSpace(decision.Result) != "" {
		record["result"] = truncateApprovalText(decision.Result, 4000)
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
	pendingReq, ok := sess.pending[reqID]
	if ok {
		delete(sess.pending, reqID)
	}
	sess.pendingMu.Unlock()

	if !ok || pendingReq == nil {
		return
	}
	ch := pendingReq.ch

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
	pendingReq, ok := sess.pending[resp.ID]
	if ok {
		delete(sess.pending, resp.ID)
	}
	sess.pendingMu.Unlock()

	if !ok || pendingReq == nil {
		return
	}

	pendingReq.ch <- approvalDecision{
		Allow:            resp.Allow,
		Reason:           strings.TrimSpace(resp.Reason),
		ApprovedChunkIDs: resp.ApprovedChunkIDs,
	}
	close(pendingReq.ch)
}

func (handler *wsProtocolHandler) finalizePermissionDecision(
	conn *websocket.Conn,
	sess *webSession,
	plan *approvalExecutionPlan,
	decision approvalDecision,
	channelOpen bool,
) agent.PermissionDecision {
	finalDecision := agent.PermissionDecision{
		Allow:            channelOpen && decision.Allow,
		Reason:           decision.Reason,
		ApprovedChunkIDs: decision.ApprovedChunkIDs,
	}
	if !channelOpen {
		finalDecision.Reason = "channel_closed"
		return finalDecision
	}
	if !decision.Allow {
		return finalDecision
	}

	approvedChunkIDs := normalizeApprovedChunkIDs(plan, decision.ApprovedChunkIDs)
	finalDecision.ApprovedChunkIDs = approvedChunkIDs
	if plan == nil || len(plan.hunks) == 0 {
		return finalDecision
	}
	if len(approvedChunkIDs) == 0 {
		return agent.PermissionDecision{
			Allow:  false,
			Reason: "no_chunks_approved",
		}
	}
	if len(approvedChunkIDs) == len(plan.hunks) {
		return finalDecision
	}

	result, err := applyApprovedChunks(plan, approvedChunkIDs, func(path string) {
		relPath := handler.server.normalizeWorkspacePathForConn(conn, path)
		handler.server.Broadcast(map[string]string{
			"type": "file_change",
			"path": relPath,
		})
		if sess != nil && sess.rec != nil {
			_ = sess.rec.Append("file_change", map[string]any{"path": relPath})
		}
	})
	if err != nil {
		return agent.PermissionDecision{
			Allow:  false,
			Reason: "apply_approved_chunks_failed: " + err.Error(),
		}
	}

	finalDecision.Applied = true
	finalDecision.Result = result
	return finalDecision
}
