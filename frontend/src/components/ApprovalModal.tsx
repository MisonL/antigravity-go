import React from 'react';
import { JsonRecord, formatValue } from './planeData';

export interface ApprovalRequest {
  id: string;
  tool: string;
  category: string;
  summary: string;
  args: string;
  preview: string;
  metadata: JsonRecord;
}

interface ApprovalModalProps {
  request: ApprovalRequest;
  onDecision: (allow: boolean) => void;
}

export function ApprovalModal({ request, onDecision }: ApprovalModalProps) {
  const metadataEntries = Object.entries(request.metadata);

  return (
    <div className="modal-overlay">
      <div
        className="glass-panel modal-content approval-modal"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="modal-header">
          <div>
            <h3>等待人工审批</h3>
            <div className="approval-modal-subtitle">{request.summary}</div>
          </div>
        </div>
        <div className="modal-body approval-modal-body">
          <div className="approval-meta-grid">
            <div className="approval-meta-card">
              <div className="approval-meta-label">工具</div>
              <code>{request.tool}</code>
            </div>
            <div className="approval-meta-card">
              <div className="approval-meta-label">类别</div>
              <span>{request.category}</span>
            </div>
            {metadataEntries.map(([key, value]) => (
              <div key={key} className="approval-meta-card">
                <div className="approval-meta-label">{key}</div>
                <span>{formatValue(value)}</span>
              </div>
            ))}
          </div>

          <section className="approval-section">
            <div className="data-section-title">变更预览</div>
            <pre className="approval-preview">{request.preview || request.args}</pre>
          </section>

          <details className="approval-details">
            <summary>查看原始参数</summary>
            <pre className="approval-args">{request.args}</pre>
          </details>
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" type="button" onClick={() => onDecision(false)}>
            拒绝
          </button>
          <button className="btn-primary" type="button" onClick={() => onDecision(true)}>
            同意
          </button>
        </div>
      </div>
    </div>
  );
}
