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
  switch (status) {
    case 'pending':
      return 'Pending';
    case 'running':
      return 'Running...';
    case 'validating':
      return 'Validating...';
    case 'success':
      return 'Success';
    case 'failed':
      return 'Failed';
    default:
      return status || 'Unknown';
  }
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

export function ScoreboardPanel({ error = '', summary }: ScoreboardPanelProps) {
  const currentTask = summary?.current_task;
  const recentFailure = summary?.recent_failure;
  const successRate = summary ? `${summary.success_rate.toFixed(0)}%` : '--';

  return (
    <div className="scoreboard-panel" data-testid="scoreboard-panel">
      <div className="scoreboard-card">
        <div className="scoreboard-label">Task</div>
        {currentTask ? (
          <>
            <span className={`badge ${taskStatusClass(currentTask.status)}`}>
              {taskStatusLabel(currentTask.status)}
            </span>
            <span className="scoreboard-text" title={currentTask.reference}>
              {shortReference(currentTask.reference)}
            </span>
          </>
        ) : (
          <span className="scoreboard-text scoreboard-text-muted">No active task</span>
        )}
      </div>

      <div className="scoreboard-card">
        <div className="scoreboard-label">Success Rate</div>
        <div className="scoreboard-metric">{successRate}</div>
        {summary && (
          <span className="scoreboard-text scoreboard-text-muted">
            ok {summary.success} / fail {summary.failed} / active {summary.in_progress}
          </span>
        )}
      </div>

      <div className="scoreboard-card scoreboard-card-alert">
        <div className="scoreboard-label">Latest Failure</div>
        {recentFailure ? (
          <>
            <span className="badge error">Failed</span>
            <span className="scoreboard-text" title={recentFailure.reference}>
              {shortReference(recentFailure.reference)}
            </span>
          </>
        ) : (
          <span className="scoreboard-text scoreboard-text-muted">No recent failure</span>
        )}
        {error && <span className="scoreboard-text scoreboard-text-error">{error}</span>}
      </div>
    </div>
  );
}
