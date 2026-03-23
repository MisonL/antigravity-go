import { SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

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
  const { t } = useAppDomain();

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="glass-panel modal-content data-modal memory-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="visual-self-test-modal-title"
      >
        <div className="modal-header">
          <h3 id="visual-self-test-modal-title">{t('visual.title')}</h3>
          <button type="button" onClick={onClose} aria-label={t('common.close')}>
            X
          </button>
        </div>

        <div className="modal-body data-modal-body">
          <div className="data-list-toolbar">
            <div>
              <div className="data-section-title">{t('visual.task.title')}</div>
              <div className="data-section-subtitle">{t('visual.task.subtitle')}</div>
            </div>
            <button type="button" className="btn-secondary" onClick={onRefresh} disabled={isLoading}>
              {t('common.refresh')}
            </button>
          </div>

          {isLoading && !sample && (
            <div className="loading-shell">
              <div className="data-state">{t('visual.loading')}</div>
              <SkeletonRows lines={4} />
            </div>
          )}
          {!isLoading && error && <div className="data-state data-state-error">{error}</div>}

          {sample && (
            <>
              <div className="data-detail-grid">
                <div className="data-field">
                  <span className="data-field-label">{t('visual.field.task_title')}</span>
                  <span className="data-field-value">{sample.title}</span>
                </div>
                <div className="data-field">
                  <span className="data-field-label">{t('visual.field.target_url')}</span>
                  <span className="data-field-value">{sample.url}</span>
                </div>
              </div>

              <div className="data-json-block">
                <div className="data-section-title">{t('visual.suggested_task')}</div>
                <pre className="data-json">{sample.task}</pre>
              </div>

              <div className="data-json-block">
                <div className="data-section-title">{t('visual.checklist')}</div>
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
            {t('common.close')}
          </button>
          <button
            className={`btn-primary${isLoading ? ' is-busy' : ''}`}
            type="button"
            onClick={() => sample && onInsertTask(sample.task)}
            disabled={!sample || isLoading}
            aria-busy={isLoading}
          >
            <span>{t('visual.insert')}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
