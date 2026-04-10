import React from 'react';
import { JsonRecord, formatValue } from './planeData';
import { useAppDomain } from '../domains/AppDomainContext';

const LARGE_DIFF_LINE_THRESHOLD = 2000;
const CONTEXT_KEEP_LINES = 3;
const PREVIEW_BATCH_LINES = 600;
const PREVIEW_BATCH_INCREMENT = 400;
const PREVIEW_SECTION_SIZE = 80;
const CHUNK_BATCH_SIZE = 20;

type PreviewSection =
  | {
    id: string;
    kind: 'lines';
    lines: string[];
  }
  | {
    id: string;
    kind: 'fold';
    lines: string[];
    hiddenCount: number;
  };

export interface ApprovalChunk {
  id: string;
  header: string;
  preview: string;
}

export interface ApprovalRequest {
  id: string;
  tool: string;
  category: string;
  summary: string;
  args: string;
  preview: string;
  chunks: ApprovalChunk[];
  metadata: JsonRecord;
}

export interface ApprovalDecisionInput {
  allow: boolean;
  approvedChunkIDs: string[];
}

interface ApprovalModalProps {
  request: ApprovalRequest;
  onDecision: (decision: ApprovalDecisionInput) => void;
}

function splitLines(text: string): string[] {
  if (!text) {
    return [];
  }
  return text.replace(/\r\n/g, '\n').split('\n');
}

function pushLineSections(target: PreviewSection[], lines: string[], sectionSeed: number): number {
  let nextSeed = sectionSeed;
  for (let index = 0; index < lines.length; index += PREVIEW_SECTION_SIZE) {
    target.push({
      id: `lines-${nextSeed}`,
      kind: 'lines',
      lines: lines.slice(index, index + PREVIEW_SECTION_SIZE),
    });
    nextSeed += 1;
  }
  return nextSeed
}

function flushContextLines(
  sections: PreviewSection[],
  buffer: string[],
  shouldFold: boolean,
  sectionSeed: number,
): number {
  if (buffer.length === 0) {
    return sectionSeed;
  }

  if (shouldFold && buffer.length > CONTEXT_KEEP_LINES * 2) {
    let nextSeed = pushLineSections(sections, buffer.slice(0, CONTEXT_KEEP_LINES), sectionSeed);
    const hiddenLines = buffer.slice(CONTEXT_KEEP_LINES, buffer.length - CONTEXT_KEEP_LINES);
    sections.push({
      id: `fold-${nextSeed}`,
      kind: 'fold',
      lines: hiddenLines,
      hiddenCount: hiddenLines.length,
    });
    nextSeed += 1;
    return pushLineSections(sections, buffer.slice(buffer.length - CONTEXT_KEEP_LINES), nextSeed);
  }

  return pushLineSections(sections, buffer, sectionSeed);
}

function buildPreviewSections(preview: string, shouldFold: boolean): PreviewSection[] {
  const lines = splitLines(preview);
  if (lines.length === 0) {
    return [{
      id: 'lines-empty',
      kind: 'lines',
      lines: [''],
    }];
  }

  const sections: PreviewSection[] = [];
  const headerLines: string[] = [];
  let index = 0;
  while (index < lines.length && !lines[index].startsWith('@@ ')) {
    headerLines.push(lines[index]);
    index += 1;
  }

  let seed = pushLineSections(sections, headerLines, 0);
  while (index < lines.length) {
    const current = lines[index];
    if (!current.startsWith('@@ ')) {
      seed = pushLineSections(sections, [current], seed);
      index += 1;
      continue;
    }

    seed = pushLineSections(sections, [current], seed);
    index += 1;

    const contextBuffer: string[] = [];
    while (index < lines.length && !lines[index].startsWith('@@ ')) {
      const line = lines[index];
      if (line.startsWith(' ')) {
        contextBuffer.push(line);
      } else {
        seed = flushContextLines(sections, contextBuffer, shouldFold, seed);
        contextBuffer.length = 0;
        seed = pushLineSections(sections, [line], seed);
      }
      index += 1;
    }
    seed = flushContextLines(sections, contextBuffer, shouldFold, seed);
  }

  return sections;
}

function countVisibleLines(section: PreviewSection, expanded: boolean): number {
  if (section.kind === 'fold' && !expanded) {
    return 1;
  }
  return section.lines.length;
}

function formatApprovalToolLabel(
  t: (key: string, ...args: unknown[]) => string,
  toolName: string,
): string {
  const labels: Record<string, string> = {
    apply_core_edit: 'approval.tool.apply_core_edit',
    write_file: 'approval.tool.write_file',
    rollback_to_step: 'approval.tool.rollback_to_step',
  };
  return t(labels[toolName] ?? 'approval.tool.generic', toolName);
}

function formatApprovalCategoryLabel(
  t: (key: string, ...args: unknown[]) => string,
  category: string,
): string {
  const labels: Record<string, string> = {
    file_write: 'approval.category.file_write',
    workspace_edit: 'approval.category.workspace_edit',
    trajectory_rollback: 'approval.category.trajectory_rollback',
    tool_execution: 'approval.category.tool_execution',
  };
  return t(labels[category] ?? 'approval.category.generic', category);
}

function formatApprovalMetadataLabel(
  t: (key: string, ...args: unknown[]) => string,
  key: string,
): string {
  const labels: Record<string, string> = {
    file_path: 'approval.meta.file_path',
    path: 'approval.meta.path',
    edit_count: 'approval.meta.edit_count',
    content_bytes: 'approval.meta.content_bytes',
    chunk_count: 'approval.meta.chunk_count',
    step_id: 'approval.meta.step_id',
    parse_error: 'approval.meta.parse_error',
    preview_error: 'approval.meta.preview_error',
  };
  return t(labels[key] ?? 'approval.meta.generic', key);
}

export function ApprovalModal({ request, onDecision }: ApprovalModalProps) {
  const { t } = useAppDomain();
  const rawPreview = request.preview || request.args;
  const metadataEntries = Object.entries(request.metadata);
  const hasChunks = request.chunks.length > 0;
  const isLargeDiff = React.useMemo(() => splitLines(rawPreview).length > LARGE_DIFF_LINE_THRESHOLD, [rawPreview]);
  const previewSections = React.useMemo(() => buildPreviewSections(rawPreview, isLargeDiff), [rawPreview, isLargeDiff]);
  const [selectedChunkIDs, setSelectedChunkIDs] = React.useState<string[]>(() => request.chunks.map((chunk) => chunk.id));
  const [expandedFoldIDs, setExpandedFoldIDs] = React.useState<string[]>([]);
  const [previewLineBudget, setPreviewLineBudget] = React.useState<number>(isLargeDiff ? PREVIEW_BATCH_LINES : Number.MAX_SAFE_INTEGER);
  const [visibleChunkCount, setVisibleChunkCount] = React.useState<number>(isLargeDiff ? Math.min(request.chunks.length, CHUNK_BATCH_SIZE) : request.chunks.length);

  React.useEffect(() => {
    setSelectedChunkIDs(request.chunks.map((chunk) => chunk.id));
    setExpandedFoldIDs([]);
    setPreviewLineBudget(isLargeDiff ? PREVIEW_BATCH_LINES : Number.MAX_SAFE_INTEGER);
    setVisibleChunkCount(isLargeDiff ? Math.min(request.chunks.length, CHUNK_BATCH_SIZE) : request.chunks.length);
  }, [request, isLargeDiff]);

  const toggleChunk = React.useCallback((chunkID: string) => {
    setSelectedChunkIDs((current) => (
      current.includes(chunkID)
        ? current.filter((id) => id !== chunkID)
        : [...current, chunkID]
    ));
  }, []);

  const toggleFold = React.useCallback((sectionID: string) => {
    React.startTransition(() => {
      setExpandedFoldIDs((current) => (
        current.includes(sectionID)
          ? current.filter((id) => id !== sectionID)
          : [...current, sectionID]
      ));
    });
  }, []);

  const visiblePreviewSections = React.useMemo(() => {
    if (!isLargeDiff) {
      return { sections: previewSections, hasMore: false };
    }

    const sections: PreviewSection[] = [];
    let consumedLines = 0;
    for (const section of previewSections) {
      const isExpanded = expandedFoldIDs.includes(section.id);
      const nextLineCount = countVisibleLines(section, isExpanded);
      if (sections.length > 0 && consumedLines+nextLineCount > previewLineBudget) {
        return { sections, hasMore: true };
      }
      sections.push(section);
      consumedLines += nextLineCount;
    }
    return { sections, hasMore: false };
  }, [expandedFoldIDs, isLargeDiff, previewLineBudget, previewSections]);

  const visibleChunks = React.useMemo(() => (
    isLargeDiff ? request.chunks.slice(0, visibleChunkCount) : request.chunks
  ), [isLargeDiff, request.chunks, visibleChunkCount]);

  const approveDisabled = hasChunks && selectedChunkIDs.length === 0;

  return (
    <div className="modal-overlay">
      <div
        className="glass-panel modal-content approval-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="approval-modal-title"
      >
        <div className="modal-header">
          <div>
            <h3 id="approval-modal-title">{t('approval.title')}</h3>
            <div className="approval-modal-subtitle">{request.summary}</div>
          </div>
        </div>
        <div className="modal-body approval-modal-body">
          <div className="approval-meta-grid">
            <div className="approval-meta-card">
              <div className="approval-meta-label">{t('approval.meta.tool')}</div>
              <code>{formatApprovalToolLabel(t, request.tool)}</code>
            </div>
            <div className="approval-meta-card">
              <div className="approval-meta-label">{t('approval.meta.category')}</div>
              <span>{formatApprovalCategoryLabel(t, request.category)}</span>
            </div>
            {metadataEntries.map(([key, value]) => (
              <div key={key} className="approval-meta-card">
                <div className="approval-meta-label">{formatApprovalMetadataLabel(t, key)}</div>
                <span>{formatValue(value)}</span>
              </div>
            ))}
          </div>

          <section className="approval-section">
            <div className="approval-section-header">
              <div className="data-section-title">{t('approval.preview')}</div>
              {isLargeDiff && (
                <span className="approval-preview-badge">{t('approval.large_diff_notice')}</span>
              )}
            </div>
            <div className="approval-preview approval-preview-shell">
              {visiblePreviewSections.sections.map((section) => {
                if (section.kind === 'fold' && !expandedFoldIDs.includes(section.id)) {
                  return (
                    <button
                      key={section.id}
                      className="approval-fold-toggle"
                      type="button"
                      onClick={() => toggleFold(section.id)}
                    >
                      {t('approval.fold_context', section.hiddenCount)}
                    </button>
                  );
                }
                return (
                  <pre key={section.id} className="approval-preview-block">
                    {section.lines.join('\n')}
                  </pre>
                );
              })}
              {visiblePreviewSections.hasMore && (
                <button
                  className="approval-preview-more"
                  type="button"
                  onClick={() => {
                    React.startTransition(() => {
                      setPreviewLineBudget((current) => current + PREVIEW_BATCH_INCREMENT);
                    });
                  }}
                >
                  {t('approval.load_more_preview')}
                </button>
              )}
            </div>
          </section>

          {hasChunks && (
            <section className="approval-section">
              <div className="data-section-title">{t('approval.chunks')}</div>
              <div className="approval-chunk-list">
                {visibleChunks.map((chunk) => {
                  const checked = selectedChunkIDs.includes(chunk.id);
                  return (
                    <label key={chunk.id} className={`approval-chunk-card${checked ? ' is-selected' : ''}`}>
                      <div className="approval-chunk-header">
                        <input
                          checked={checked}
                          onChange={() => toggleChunk(chunk.id)}
                          type="checkbox"
                        />
                        <div>
                          <div className="approval-chunk-title">{chunk.id}</div>
                          <div className="approval-chunk-subtitle">{chunk.header}</div>
                        </div>
                      </div>
                      <pre className="approval-preview approval-chunk-preview">{chunk.preview}</pre>
                    </label>
                  );
                })}
                {isLargeDiff && visibleChunkCount < request.chunks.length && (
                  <button
                    className="approval-preview-more"
                    type="button"
                    onClick={() => {
                      React.startTransition(() => {
                        setVisibleChunkCount((current) => Math.min(request.chunks.length, current + CHUNK_BATCH_SIZE));
                      });
                    }}
                  >
                    {t('approval.load_more_chunks', request.chunks.length - visibleChunkCount)}
                  </button>
                )}
              </div>
            </section>
          )}

          <details className="approval-details">
            <summary>{t('approval.args')}</summary>
            <pre className="approval-args">{request.args}</pre>
          </details>
        </div>
        <div className="modal-footer">
          <button
            className="btn-secondary"
            type="button"
            onClick={() => onDecision({ allow: false, approvedChunkIDs: [] })}
          >
            {t('approval.reject')}
          </button>
          <button
            className="btn-primary"
            disabled={approveDisabled}
            type="button"
            onClick={() => onDecision({ allow: true, approvedChunkIDs: selectedChunkIDs })}
          >
            {hasChunks ? t('approval.approve_selected', selectedChunkIDs.length) : t('approval.approve')}
          </button>
        </div>
      </div>
    </div>
  );
}
