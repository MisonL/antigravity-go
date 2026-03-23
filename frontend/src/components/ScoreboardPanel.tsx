import { useAppDomain } from '../domains/AppDomainContext';

export interface TaskSummaryItem {
  id: string;
  reference: string;
  status: string;
  updated_at: string;
}

export interface TaskSummaryResponse {
  generated_at: string;
  total: number;
  success: number;
  failed: number;
  in_progress: number;
  success_rate: number;
  current_task?: TaskSummaryItem;
  recent_failure?: TaskSummaryItem;
  tasks: TaskSummaryItem[];
}

interface ScoreboardPanelProps {
  error?: string;
  summary: TaskSummaryResponse | null;
}

function taskStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    pending: 'scoreboard.status.pending',
    running: 'scoreboard.status.running',
    validating: 'scoreboard.status.validating',
    success: 'scoreboard.status.success',
    failed: 'scoreboard.status.failed',
  };
  return labels[status] ?? 'scoreboard.status.unknown';
}

export function ScoreboardPanel({ error = '', summary }: ScoreboardPanelProps) {
  const { t } = useAppDomain();
  const currentTask = summary?.current_task;
  const recentFailure = summary?.recent_failure;
  const successRate = summary ? `${summary.success_rate.toFixed(0)}%` : '--';

  return (
    <div className="scoreboard-panel" data-testid="scoreboard-panel">
      <div className="scoreboard-card">
        <div className="scoreboard-label">{t('scoreboard.task')}</div>
        {currentTask ? (
          <>
            <span className={`badge ${taskStatusClass(currentTask.status)}`}>
              {t(taskStatusLabel(currentTask.status))}
            </span>
            <span className="scoreboard-text" title={currentTask.reference}>
              {shortReference(currentTask.reference)}
            </span>
          </>
        ) : (
          <span className="scoreboard-text scoreboard-text-muted">{t('scoreboard.no_active_task')}</span>
        )}
      </div>

      <div className="scoreboard-card">
        <div className="scoreboard-label">{t('scoreboard.success_rate')}</div>
        <div className="scoreboard-metric">{successRate}</div>
        {summary && (
          <span className="scoreboard-text scoreboard-text-muted">
            {t('scoreboard.stats', summary.success, summary.failed, summary.in_progress)}
          </span>
        )}
      </div>

      <div className="scoreboard-card scoreboard-card-alert">
        <div className="scoreboard-label">{t('scoreboard.latest_failure')}</div>
        {recentFailure ? (
          <>
            <span className="badge error">{t('scoreboard.status.failed')}</span>
            <span className="scoreboard-text" title={recentFailure.reference}>
              {shortReference(recentFailure.reference)}
            </span>
          </>
        ) : (
          <span className="scoreboard-text scoreboard-text-muted">{t('scoreboard.no_recent_failure')}</span>
        )}
        {error && <span className="scoreboard-text scoreboard-text-error">{error}</span>}
      </div>
    </div>
  );
}

function taskStatusClass(status: string): string {
  switch (status) {
    case 'success':
      return 'success';
    case 'failed':
      return 'error';
    case 'running':
    case 'validating':
    case 'pending':
      return 'processing';
    default:
      return 'info';
  }
}

function shortReference(reference: string): string {
  const text = reference.trim();
  if (text.length <= 48) {
    return text;
  }
  return `${text.slice(0, 45)}...`;
}
