import { AsyncContent } from './AsyncState';
import { formatValue } from './planeData';
import { SkeletonCardList, SkeletonRows } from './Skeleton';
import {
  type ExecutionDetailResponse,
  type ExecutionSummaryItem,
} from '../domains/types';
import { useAppDomain } from '../domains/AppDomainContext';

interface ExecutionModalProps {
  detail: ExecutionDetailResponse | null;
  detailError: string;
  detailLoading: boolean;
  isLoading: boolean;
  items: ExecutionSummaryItem[];
  listError: string;
  onClose: () => void;
  onRefresh: () => void;
  onSelect: (id: string) => void;
  selectedId: string;
}

function statusLabel(
  t: (key: string, ...args: unknown[]) => string,
  status: string,
): string {
  const normalized = status.trim().toLowerCase();
  switch (normalized) {
    case 'pending':
      return t('scoreboard.status.pending');
    case 'running':
      return t('scoreboard.status.running');
    case 'validating':
      return t('scoreboard.status.validating');
    case 'success':
      return t('scoreboard.status.success');
    case 'failed':
      return t('scoreboard.status.failed');
    default:
      return t('scoreboard.status.unknown');
  }
}

function statusClass(status: string): string {
  switch (status.trim().toLowerCase()) {
    case 'success':
      return 'success';
    case 'failed':
      return 'error';
    case 'pending':
    case 'running':
    case 'validating':
      return 'processing';
    default:
      return 'info';
  }
}

function summarizeReference(item: ExecutionSummaryItem): string {
  return item.summary || item.title || item.reference;
}

export function ExecutionModal({
  detail,
  detailError,
  detailLoading,
  isLoading,
  items,
  listError,
  onClose,
  onRefresh,
  onSelect,
  selectedId,
}: ExecutionModalProps) {
  const { t } = useAppDomain();
  const execution = detail?.execution;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="glass-panel modal-content data-modal"
        data-testid="execution-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="execution-modal-title"
      >
        <div className="modal-header">
          <h3 id="execution-modal-title">{t('execution.title')}</h3>
          <button type="button" onClick={onClose} aria-label={t('common.close')}>
            X
          </button>
        </div>

        <div className="data-modal-shell">
          <section className="data-list-panel">
            <div className="data-list-toolbar">
              <div>
                <div className="data-section-title">{t('execution.list.title')}</div>
                <div className="data-section-subtitle">{t('execution.list.subtitle')}</div>
              </div>
              <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
                {t('common.refresh')}
              </button>
            </div>

            <div className="data-list" data-testid="execution-list">
              <AsyncContent
                emptyMessage={t('execution.empty')}
                error={listError}
                hasContent={items.length > 0}
                loading={isLoading}
                loadingMessage={t('execution.loading')}
                skeleton={<SkeletonCardList cards={4} lines={3} />}
              >
                {items.map((item) => (
                  <div
                    key={item.id}
                    className={`data-list-item ${selectedId === item.id ? 'active' : ''}`}
                  >
                    <button type="button" className="data-list-item-select" onClick={() => onSelect(item.id)}>
                      <div className="data-list-item-header">
                        <span className="data-list-item-title">{item.reference}</span>
                        <span className={`badge ${statusClass(item.status)}`}>{statusLabel(t, item.status)}</span>
                      </div>
                      <div className="data-list-item-summary">{summarizeReference(item)}</div>
                      {item.updated_at && <div className="data-list-item-meta">{item.updated_at}</div>}
                    </button>
                  </div>
                ))}
              </AsyncContent>
            </div>
          </section>

          <section className="data-detail-panel" data-testid="execution-detail">
            <div className="data-detail-header">
              <div>
                <div className="data-section-title">{t('execution.detail.title')}</div>
                <div className="data-section-subtitle">{t('execution.detail.subtitle')}</div>
              </div>
            </div>

            <div className="data-detail-body">
              <AsyncContent
                emptyMessage={t('execution.detail.empty')}
                error={detailError}
                hasContent={Boolean(detail && execution)}
                loading={detailLoading}
                loadingMessage={t('execution.detail.loading')}
                skeleton={(
                  <>
                    <SkeletonRows lines={4} />
                    <SkeletonCardList cards={3} lines={2} />
                  </>
                )}
              >
                {detail && execution && (
                  <>
                    <div className="data-detail-grid">
                      <div className="data-field">
                        <span className="data-field-label">ID</span>
                        <span className="data-field-value">{execution.id}</span>
                      </div>
                      <div className="data-field">
                        <span className="data-field-label">{t('execution.field.status')}</span>
                        <span className="data-field-value">{statusLabel(t, execution.status)}</span>
                      </div>
                      <div className="data-field">
                        <span className="data-field-label">{t('execution.field.reference')}</span>
                        <span className="data-field-value">{execution.reference}</span>
                      </div>
                      <div className="data-field">
                        <span className="data-field-label">{t('execution.field.updated_at')}</span>
                        <span className="data-field-value">{execution.updated_at || '-'}</span>
                      </div>
                      <div className="data-field">
                        <span className="data-field-label">{t('execution.field.checkpoint')}</span>
                        <span className="data-field-value">{execution.checkpoint_id || '-'}</span>
                      </div>
                      <div className="data-field">
                        <span className="data-field-label">{t('execution.field.rollback')}</span>
                        <span className="data-field-value">{execution.rollback_point || '-'}</span>
                      </div>
                    </div>

                    <div className="data-json-block">
                      <div className="data-section-title">{t('execution.steps')}</div>
                      {detail.steps.length === 0 && <div className="data-state">{t('execution.steps.empty')}</div>}
                      {detail.steps.length > 0 && (
                        <div className="trajectory-step-list">
                          {detail.steps.map((step) => (
                            <div key={step.id} className="trajectory-step-card">
                              <div className="trajectory-step-header">
                                <div>
                                  <div className="trajectory-step-title">{step.title}</div>
                                  <div className="trajectory-step-meta">{step.kind}</div>
                                </div>
                                <span className={`badge ${statusClass(step.status)}`}>{statusLabel(t, step.status)}</span>
                              </div>
                              {step.started_at && <div className="trajectory-step-meta">{step.started_at}</div>}
                              {step.finished_at && step.finished_at !== step.started_at && (
                                <div className="trajectory-step-meta">{step.finished_at}</div>
                              )}
                              {step.summary && <div className="data-field-value">{step.summary}</div>}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="data-json-block">
                      <div className="data-section-title">{t('execution.timeline')}</div>
                      {detail.timeline.length === 0 && <div className="data-state">{t('execution.timeline.empty')}</div>}
                      {detail.timeline.length > 0 && (
                        <div className="trajectory-step-list">
                          {detail.timeline.map((entry) => (
                            <div key={entry.id} className="trajectory-step-card">
                              <div className="trajectory-step-header">
                                <div>
                                  <div className="trajectory-step-title">{entry.title}</div>
                                  <div className="trajectory-step-meta">{entry.type || entry.kind || entry.id}</div>
                                </div>
                                <span className={`badge ${statusClass(entry.status)}`}>{statusLabel(t, entry.status)}</span>
                              </div>
                              {(entry.time || entry.updated_at) && (
                                <div className="trajectory-step-meta">{entry.time || entry.updated_at}</div>
                              )}
                              {entry.message && <div className="data-field-value">{entry.message}</div>}
                              {entry.summary && entry.summary !== entry.message && (
                                <div className="trajectory-step-meta">{entry.summary}</div>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="data-json-block">
                      <div className="data-section-title">{t('execution.raw_json')}</div>
                      <pre className="data-json">{formatValue(detail.raw)}</pre>
                    </div>
                  </>
                )}
              </AsyncContent>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
