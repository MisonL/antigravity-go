import { useMemo } from 'react';
import { StateMessage } from './AsyncState';
import { useAppDomain } from '../domains/AppDomainContext';
import type { CodeFrequencyBucket, CodeFrequencyResponse } from '../domains/types';

interface CodeFrequencyPanelProps {
  error?: string;
  loading: boolean;
  onRefresh: () => void;
  summary: CodeFrequencyResponse | null;
}

interface CodeFrequencyTotals {
  commits: number;
  linesAdded: number;
  linesDeleted: number;
}

export function CodeFrequencyPanel({
  error = '',
  loading,
  onRefresh,
  summary,
}: CodeFrequencyPanelProps) {
  const { t } = useAppDomain();
  const recentBuckets = useMemo(
    () => summary?.code_frequency.slice(-6) ?? [],
    [summary],
  );
  const totals = useMemo(() => summarizeBuckets(summary?.code_frequency ?? []), [summary]);
  const maxChangedLines = useMemo(
    () => Math.max(...recentBuckets.map((item) => item.lines_added + item.lines_deleted), 1),
    [recentBuckets],
  );

  return (
    <div className="code-frequency-panel scoreboard-card" data-testid="code-frequency-panel">
      <div className="code-frequency-panel__header">
        <div>
          <div className="scoreboard-label">{t('code_frequency.title')}</div>
          <div className="scoreboard-text scoreboard-text-muted">{t('code_frequency.subtitle')}</div>
        </div>
        <button
          className="btn-secondary code-frequency-panel__refresh"
          disabled={loading}
          onClick={onRefresh}
          type="button"
        >
          {summary ? t('code_frequency.action.refresh') : t('code_frequency.action.load')}
        </button>
      </div>

      {loading && <StateMessage message={t('code_frequency.loading')} />}
      {!loading && error && <StateMessage kind="error" message={error} />}
      {!loading && !error && !summary && (
        <StateMessage message={t('code_frequency.empty')} />
      )}

      {summary && (
        <>
          <div className="code-frequency-panel__metrics">
            <div className="code-frequency-panel__metric">
              <span className="scoreboard-label">{t('code_frequency.metric.commits')}</span>
              <span className="scoreboard-metric">{totals.commits}</span>
            </div>
            <div className="code-frequency-panel__metric">
              <span className="scoreboard-label">{t('code_frequency.metric.added')}</span>
              <span className="scoreboard-metric">{formatDelta(totals.linesAdded)}</span>
            </div>
            <div className="code-frequency-panel__metric">
              <span className="scoreboard-label">{t('code_frequency.metric.deleted')}</span>
              <span className="scoreboard-metric">{formatDeleted(totals.linesDeleted)}</span>
            </div>
          </div>

          <div className="code-frequency-panel__bars" aria-label={t('code_frequency.bars')}>
            {recentBuckets.map((bucket) => {
              const changedLines = bucket.lines_added + bucket.lines_deleted;
              const height = `${Math.max(20, Math.round((changedLines / maxChangedLines) * 100))}%`;
              return (
                <div className="code-frequency-panel__bucket" key={`${bucket.record_start_time}-${bucket.record_end_time}`}>
                  <div className="code-frequency-panel__bar-shell">
                    <div className="code-frequency-panel__bar" style={{ height }} />
                  </div>
                  <div className="code-frequency-panel__bucket-label">{formatBucketLabel(bucket.record_end_time)}</div>
                  <div className="code-frequency-panel__bucket-meta">{bucket.num_commits}c</div>
                </div>
              );
            })}
          </div>

          <div className="scoreboard-text scoreboard-text-muted">
            {t(
              'code_frequency.window',
              formatWindow(summary.code_frequency[0]?.record_start_time),
              formatWindow(summary.code_frequency[summary.code_frequency.length - 1]?.record_end_time),
            )}
          </div>
        </>
      )}
    </div>
  );
}

function summarizeBuckets(buckets: CodeFrequencyBucket[]): CodeFrequencyTotals {
  return buckets.reduce<CodeFrequencyTotals>((acc, item) => ({
    commits: acc.commits + item.num_commits,
    linesAdded: acc.linesAdded + item.lines_added,
    linesDeleted: acc.linesDeleted + item.lines_deleted,
  }), {
    commits: 0,
    linesAdded: 0,
    linesDeleted: 0,
  });
}

function formatDelta(value: number): string {
  return value > 0 ? `+${value}` : String(value);
}

function formatDeleted(value: number): string {
  return value > 0 ? `-${value}` : String(value);
}

function formatBucketLabel(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
}

function formatWindow(value: string | undefined): string {
  if (!value) {
    return '--';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
