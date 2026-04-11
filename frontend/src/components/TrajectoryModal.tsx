import { AsyncContent, StateMessage } from './AsyncState';
import { formatValue, JsonRecord, pickString, TrajectoryStepSummary, TrajectorySummary } from './planeData';
import { SkeletonCardList, SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

interface TrajectoryModalProps {
  detailSupported: boolean;
  detailError: string;
  detailLoading: boolean;
  isLoading: boolean;
  items: TrajectorySummary[];
  listError: string;
  onClose: () => void;
  onRefresh: () => void;
  onResume: (id: string) => void;
  onRollback: (stepId: string) => void;
  onSelect: (id: string) => void;
  resumeSupported: boolean;
  resumeError: string;
  resumeLoadingId: string;
  resumeSuccess: string;
  rollbackSupported: boolean;
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
  detailSupported,
  detailError,
  detailLoading,
  isLoading,
  items,
  listError,
  onClose,
  onRefresh,
  onResume,
  onRollback,
  onSelect,
  resumeSupported,
  resumeError,
  resumeLoadingId,
  resumeSuccess,
  rollbackSupported,
  rollbackError,
  rollbackStepId,
  rollbackSuccess,
  selectedDetail,
  selectedId,
  steps,
}: TrajectoryModalProps) {
  const { t } = useAppDomain();

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="glass-panel modal-content data-modal"
        data-testid="trajectory-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="trajectory-modal-title"
      >
        <div className="modal-header">
          <h3 id="trajectory-modal-title">{t('trajectory.title')}</h3>
          <button type="button" onClick={onClose} aria-label={t('common.close')}>
            X
          </button>
        </div>

        <div className="data-modal-shell">
          <section className="data-list-panel">
            <div className="data-list-toolbar">
              <div>
                <div className="data-section-title">{t('trajectory.list.title')}</div>
                <div className="data-section-subtitle">
                  {detailSupported ? t('trajectory.list.subtitle') : t('trajectory.list.subtitle_read_only')}
                </div>
              </div>
              <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
                {t('common.refresh')}
              </button>
            </div>

            <div className="data-list" data-testid="trajectory-list">
              <AsyncContent
                emptyMessage={t('trajectory.empty')}
                error={listError}
                hasContent={items.length > 0}
                loading={isLoading}
                loadingMessage={t('trajectory.loading')}
                skeleton={<SkeletonCardList cards={4} lines={3} />}
              >
                {items.map((item) => (
                  <div
                    key={item.id}
                    className={`data-list-item ${selectedId === item.id ? 'active' : ''}`}
                  >
                    <button type="button" className="data-list-item-select" onClick={() => onSelect(item.id)}>
                      <div className="data-list-item-header">
                        <span className="data-list-item-title">{item.id}</span>
                        <span className="badge info">{item.status}</span>
                      </div>
                      {item.title && <div className="data-list-item-summary">{item.title}</div>}
                      {item.updatedAt && <div className="data-list-item-meta">{item.updatedAt}</div>}
                    </button>
                    {resumeSupported && (
                      <button
                        type="button"
                        className="btn-secondary trajectory-resume-button"
                        onClick={() => onResume(item.id)}
                        disabled={resumeLoadingId === item.id}
                      >
                        {resumeLoadingId === item.id ? t('trajectory.resuming') : t('trajectory.resume')}
                      </button>
                    )}
                  </div>
                ))}
              </AsyncContent>
            </div>
            {!resumeSupported && (
              <div className="data-list-toolbar">
                <StateMessage message={t('trajectory.resume_unavailable')} />
              </div>
            )}
            {resumeError && <div className="data-list-toolbar"><StateMessage kind="error" message={resumeError} /></div>}
            {resumeSuccess && <div className="data-list-toolbar"><StateMessage kind="success" message={resumeSuccess} /></div>}
          </section>

          <section className="data-detail-panel" data-testid="trajectory-detail">
            <div className="data-detail-header">
              <div>
                <div className="data-section-title">{t('trajectory.detail.title')}</div>
                <div className="data-section-subtitle">{t('trajectory.detail.subtitle')}</div>
              </div>
            </div>

            <div className="data-detail-body">
              <AsyncContent
                emptyMessage={detailSupported ? t('trajectory.detail.empty') : t('trajectory.detail.unsupported')}
                error={detailError}
                hasContent={!detailSupported || Boolean(selectedDetail)}
                loading={detailSupported && detailLoading}
                loadingMessage={detailSupported ? t('trajectory.detail.loading') : t('common.loading')}
                skeleton={(
                  <>
                    <SkeletonRows lines={4} />
                    <SkeletonCardList cards={2} lines={2} />
                  </>
                )}
              >
                {!detailSupported && (
                  <div className="data-state">{t('trajectory.detail.unsupported')}</div>
                )}
                {detailSupported && selectedDetail && (
                  <>
                  <div className="data-detail-grid">
                    <div className="data-field">
                      <span className="data-field-label">ID</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['id', 'trajectory_id', 'trajectoryId', 'uuid'])}
                      </span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">{t('trajectory.field.status')}</span>
                      <span className="data-field-value">{renderDetailValue(selectedDetail, ['status', 'state'])}</span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">{t('trajectory.field.title')}</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['title', 'name', 'summary', 'description'])}
                      </span>
                    </div>
                    <div className="data-field">
                      <span className="data-field-label">{t('trajectory.field.updated_at')}</span>
                      <span className="data-field-value">
                        {renderDetailValue(selectedDetail, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp'])}
                      </span>
                    </div>
                  </div>

                  <div className="data-json-block">
                    <div className="data-section-title">{t('trajectory.steps')}</div>
                    {steps.length === 0 && <div className="data-state">{t('trajectory.steps.empty')}</div>}
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
                            {rollbackSupported && (
                              <button
                                type="button"
                                className="btn-secondary"
                                onClick={() => onRollback(step.id)}
                                disabled={rollbackStepId === step.id}
                              >
                                {rollbackStepId === step.id ? t('trajectory.rolling_back') : t('trajectory.rollback')}
                              </button>
                            )}
                          </div>
                        ))}
                      </div>
                    )}

                    {!rollbackSupported && steps.length > 0 && <StateMessage message={t('trajectory.rollback_unavailable')} />}
                    <StateMessage kind="error" message={rollbackError} />
                    <StateMessage kind="success" message={rollbackSuccess} />
                  </div>

                  <div className="data-json-block">
                    <div className="data-section-title">{t('trajectory.raw_json')}</div>
                    <pre className="data-json">{formatValue(selectedDetail)}</pre>
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
