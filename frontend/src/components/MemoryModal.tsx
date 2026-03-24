import { AsyncContent } from './AsyncState';
import { formatValue, MemorySummary } from './planeData';
import { SkeletonCardList } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

interface MemoryModalProps {
  isLoading: boolean;
  items: MemorySummary[];
  listError: string;
  onClose: () => void;
  onRefresh: () => void;
}

export function MemoryModal({ isLoading, items, listError, onClose, onRefresh }: MemoryModalProps) {
  const { t } = useAppDomain();

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="glass-panel modal-content data-modal memory-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="memory-modal-title"
      >
        <div className="modal-header">
          <h3 id="memory-modal-title">{t('memory.title')}</h3>
          <button type="button" onClick={onClose} aria-label={t('common.close')}>
            X
          </button>
        </div>

        <div className="modal-body data-modal-body">
          <div className="data-list-toolbar">
            <div>
              <div className="data-section-title">{t('memory.list.title')}</div>
              <div className="data-section-subtitle">{t('memory.list.subtitle')}</div>
            </div>
            <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
              {t('common.refresh')}
            </button>
          </div>

          <div className="memory-list">
            <AsyncContent
              emptyMessage={t('memory.empty')}
              error={listError}
              hasContent={items.length > 0}
              loading={isLoading}
              loadingMessage={t('memory.loading')}
              skeleton={<SkeletonCardList cards={4} lines={3} />}
            >
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
                    <summary>{t('memory.raw_json')}</summary>
                    <pre className="data-json">{formatValue(item.raw)}</pre>
                  </details>
                </article>
              ))}
            </AsyncContent>
          </div>
        </div>
      </div>
    </div>
  );
}
