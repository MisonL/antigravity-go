import { useAppDomain } from '../domains/AppDomainContext';
import {
  type ExecutionSummaryItem,
  type ExecutionSummaryResponse,
} from '../domains/types';

export type TaskSummaryItem = ExecutionSummaryItem;
export type TaskSummaryResponse = ExecutionSummaryResponse;

interface ScoreboardPanelProps {
  error?: string;
  summary: TaskSummaryResponse | null;
}

function taskStatusLabel(status: string): string {
  const normalized = normalizeTaskStatus(status);
  const labels: Record<string, string> = {
    pending: 'scoreboard.status.pending',
    running: 'scoreboard.status.running',
    validating: 'scoreboard.status.validating',
    success: 'scoreboard.status.success',
    failed: 'scoreboard.status.failed',
  };
  return labels[normalized] ?? 'scoreboard.status.unknown';
}

export function ScoreboardPanel({ error = '', summary }: ScoreboardPanelProps) {
  const { t } = useAppDomain();
  const currentTask = summary?.current_execution ?? summary?.current_task;
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
  switch (normalizeTaskStatus(status)) {
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

function normalizeTaskStatus(status: string): string {
  switch (status.trim().toLowerCase()) {
    case 'queued':
    case 'planning':
    case 'awaiting_approval':
    case 'approval_pending':
      return 'pending';
    case 'executing':
    case 'running':
    case 'in_progress':
      return 'running';
    case 'validating':
    case 'verifying':
      return 'validating';
    case 'completed':
    case 'done':
      return 'success';
    case 'rolled_back':
    case 'rollback':
    case 'blocked':
    case 'cancelled':
    case 'canceled':
      return 'failed';
    default:
      return status.trim().toLowerCase();
  }
}

function shortReference(reference: string): string {
  const text = reference.trim();
  if (text.length <= 48) {
    return text;
  }
  return `${text.slice(0, 45)}...`;
}
