import { formatValue, MemorySummary } from './planeData';

interface MemoryModalProps {
  isLoading: boolean;
  items: MemorySummary[];
  listError: string;
  onClose: () => void;
  onRefresh: () => void;
}

export function MemoryModal({ isLoading, items, listError, onClose, onRefresh }: MemoryModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="glass-panel modal-content data-modal memory-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <h3>系统记忆</h3>
          <button type="button" onClick={onClose}>
            X
          </button>
        </div>

        <div className="modal-body data-modal-body">
          <div className="data-list-toolbar">
            <div>
              <div className="data-section-title">记忆列表</div>
              <div className="data-section-subtitle">展示核心分类与内容摘要</div>
            </div>
            <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
              刷新
            </button>
          </div>

          <div className="memory-list">
            {isLoading && items.length === 0 && <div className="data-state">正在加载系统记忆...</div>}
            {!isLoading && listError && <div className="data-state data-state-error">{listError}</div>}
            {!isLoading && !listError && items.length === 0 && <div className="data-state">暂无记忆数据。</div>}

            {items.map((item) => (
              <article key={item.id} className="memory-card">
                <div className="memory-card-header">
                  <div className="memory-card-title-group">
                    <span className="memory-card-category">{item.category}</span>
                    <span className="memory-card-id">{item.id}</span>
                  </div>
                  {item.updatedAt && <span className="memory-card-time">{item.updatedAt}</span>}
                </div>
                <div className="memory-card-content">{item.content}</div>
                <details className="memory-card-details">
                  <summary>查看原始 JSON</summary>
                  <pre className="data-json">{formatValue(item.raw)}</pre>
                </details>
              </article>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
