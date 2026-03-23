import { formatValue, JsonRecord, pickString, TrajectoryStepSummary, TrajectorySummary } from './planeData';

interface TrajectoryModalProps {
  detailError: string;
  detailLoading: boolean;
  isLoading: boolean;
  items: TrajectorySummary[];
  listError: string;
  onClose: () => void;
  onRefresh: () => void;
  onRollback: (stepId: string) => void;
  onSelect: (id: string) => void;
  rollbackError: string;
  rollbackStepId: string;
  rollbackSuccess: string;
  selectedDetail: JsonRecord | null;
  selectedId: string;
  steps: TrajectoryStepSummary[];
}

function renderDetailValue(detail: JsonRecord, keys: string[], fallback = '-') {
  return pickString(detail, keys) || fallback;
}

export function TrajectoryModal({
  detailError,
  detailLoading,
  isLoading,
  items,
  listError,
  onClose,
  onRefresh,
  onRollback,
  onSelect,
  rollbackError,
  rollbackStepId,
  rollbackSuccess,
  selectedDetail,
  selectedId,
  steps,
}: TrajectoryModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="glass-panel modal-content data-modal" data-testid="trajectory-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <h3>轨迹树</h3>
          <button type="button" onClick={onClose}>
            X
          </button>
        </div>

        <div className="data-modal-shell">
          <section className="data-list-panel">
            <div className="data-list-toolbar">
              <div>
                <div className="data-section-title">轨迹列表</div>
                <div className="data-section-subtitle">点击记录查看详情</div>
              </div>
              <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
                刷新
              </button>
            </div>

            <div className="data-list" data-testid="trajectory-list">
              {isLoading && items.length === 0 && <div className="data-state">正在加载轨迹数据...</div>}
              {!isLoading && listError && <div className="data-state data-state-error">{listError}</div>}
              {!isLoading && !listError && items.length === 0 && <div className="data-state">暂无轨迹数据。</div>}

              {items.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={`data-list-item ${selectedId === item.id ? 'active' : ''}`}
                  onClick={() => onSelect(item.id)}
                >
                  <div className="data-list-item-header">
                    <span className="data-list-item-title">{item.id}</span>
                    <span className="badge info">{item.status}</span>
                  </div>
                  {item.title && <div className="data-list-item-summary">{item.title}</div>}
                  {item.updatedAt && <div className="data-list-item-meta">{item.updatedAt}</div>}
                </button>
              ))}
            </div>
          </section>

          <section className="data-detail-panel" data-testid="trajectory-detail">
            <div className="data-detail-header">
              <div>
                <div className="data-section-title">轨迹详情</div>
                <div className="data-section-subtitle">核心字段与原始 JSON</div>
              </div>
            </div>

            <div className="data-detail-body">
              {detailLoading && <div className="data-state">正在加载轨迹详情...</div>}
              {!detailLoading && detailError && <div className="data-state data-state-error">{detailError}</div>}
              {!detailLoading && !detailError && !selectedDetail && <div className="data-state">请选择一条轨迹记录。</div>}

              {!detailLoading && !detailError && selectedDetail && (
                <>
                  <div className="data-detail-grid">
                    <div className="data-field">
                      <span className="data-field-label">ID</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['id', 'trajectory_id', 'trajectoryId', 'uuid'])}
                      </span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">状态</span>
                      <span className="data-field-value">{renderDetailValue(selectedDetail, ['status', 'state'])}</span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">标题</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['title', 'name', 'summary', 'description'])}
                      </span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">更新时间</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp'])}
                      </span>
                    </div>
                  </div>

                  <div className="data-json-block">
                    <div className="data-section-title">轨迹步骤</div>
                    {steps.length === 0 && <div className="data-state">当前轨迹未暴露可回滚步骤。</div>}
                    {steps.length > 0 && (
                      <div className="trajectory-step-list">
                        {steps.map((step) => (
                          <div key={step.id} className="trajectory-step-card">
                            <div className="trajectory-step-header">
                              <div>
                                <div className="trajectory-step-title">{step.title}</div>
                                <div className="trajectory-step-meta">{step.id}</div>
                              </div>
                              <span className="badge info">{step.status}</span>
                            </div>
                            {step.updatedAt && <div className="trajectory-step-meta">{step.updatedAt}</div>}
                            <button
                              type="button"
                              className="btn-secondary"
                              onClick={() => onRollback(step.id)}
                              disabled={rollbackStepId === step.id}
                            >
                              {rollbackStepId === step.id ? '回滚中...' : '回滚到此步骤'}
                            </button>
                          </div>
                        ))}
                      </div>
                    )}

                    {rollbackError && <div className="data-state data-state-error">{rollbackError}</div>}
                    {rollbackSuccess && <div className="data-state data-state-success">{rollbackSuccess}</div>}
                  </div>

                  <div className="data-json-block">
                    <div className="data-section-title">原始 JSON</div>
                    <pre className="data-json">{formatValue(selectedDetail)}</pre>
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
