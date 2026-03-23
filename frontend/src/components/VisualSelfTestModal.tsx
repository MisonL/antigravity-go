interface SelfTestChecklistItem {
  label: string;
  selector: string;
}

interface VisualSelfTestSample {
  checklist: SelfTestChecklistItem[];
  task: string;
  title: string;
  url: string;
}

interface VisualSelfTestModalProps {
  error: string;
  isLoading: boolean;
  onClose: () => void;
  onInsertTask: (task: string) => void;
  onRefresh: () => void;
  sample: VisualSelfTestSample | null;
}

export function VisualSelfTestModal({
  error,
  isLoading,
  onClose,
  onInsertTask,
  onRefresh,
  sample,
}: VisualSelfTestModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="glass-panel modal-content data-modal memory-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <h3>视觉自测原型</h3>
          <button type="button" onClick={onClose}>
            X
          </button>
        </div>

        <div className="modal-body data-modal-body">
          <div className="data-list-toolbar">
            <div>
              <div className="data-section-title">Agent 自测任务</div>
              <div className="data-section-subtitle">将示例任务注入对话框，驱动 Agent 调用 browser 工具验证控制台。</div>
            </div>
            <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
              刷新
            </button>
          </div>

          {isLoading && !sample && <div className="data-state">正在生成自测任务...</div>}
          {!isLoading && error && <div className="data-state data-state-error">{error}</div>}

          {sample && (
            <>
              <div className="data-detail-grid">
                <div className="data-field">
                  <span className="data-field-label">任务标题</span>
                  <span className="data-field-value">{sample.title}</span>
                </div>
                <div className="data-field">
                  <span className="data-field-label">目标 URL</span>
                  <span className="data-field-value">{sample.url}</span>
                </div>
              </div>

              <div className="data-json-block">
                <div className="data-section-title">建议任务</div>
                <pre className="data-json">{sample.task}</pre>
              </div>

              <div className="data-json-block">
                <div className="data-section-title">核心检查点</div>
                <div className="trajectory-step-list">
                  {sample.checklist.map((item) => (
                    <div key={item.selector} className="trajectory-step-card">
                      <div className="trajectory-step-title">{item.label}</div>
                      <div className="trajectory-step-meta">{item.selector}</div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn-secondary" type="button" onClick={onClose}>
            关闭
          </button>
          <button
            className="btn-primary"
            type="button"
            onClick={() => sample && onInsertTask(sample.task)}
            disabled={!sample}
          >
            插入到对话框
          </button>
        </div>
      </div>
    </div>
  );
}
