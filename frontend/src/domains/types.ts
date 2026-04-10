import type { ApprovalRequest } from '../components/ApprovalModal';
import {
  extractCollection,
  isRecord,
  pickString,
  type JsonRecord,
} from '../components/planeData';

export interface ServerStatus {
  ready: boolean;
  core_port: number;
  token_usage: number;
}

export interface ToolData {
  name: string;
  args: string;
  result?: string;
  status: 'running' | 'completed' | 'error';
}

export interface ChatMessage {
  role: 'user' | 'assistant' | 'tool';
  content?: string;
  toolData?: ToolData;
}

export interface PlaneSnapshot {
  count: number;
  latest_id?: string;
  latest_updated_at?: string;
}

export interface ObservabilitySummary {
  generated_at: string;
  memories: PlaneSnapshot;
  trajectories: PlaneSnapshot;
}

export interface ExecutionSummaryItem {
  id: string;
  reference: string;
  status: string;
  updated_at: string;
  title?: string;
  summary?: string;
  stage?: string;
  execution_id?: string;
  checkpoint_id?: string;
  rollback_point?: string;
  raw: JsonRecord;
}

export interface ExecutionTimelineEntry {
  id: string;
  status: string;
  title: string;
  updated_at: string;
  time?: string;
  message?: string;
  type?: string;
  kind?: string;
  step_id?: string;
  checkpoint_id?: string;
  summary?: string;
  raw: JsonRecord;
}

export interface ExecutionStepEntry {
  id: string;
  kind: string;
  title: string;
  status: string;
  started_at: string;
  finished_at: string;
  summary?: string;
  raw: JsonRecord;
}

export interface ExecutionDetailResponse {
  execution: ExecutionSummaryItem | null;
  timeline: ExecutionTimelineEntry[];
  steps: ExecutionStepEntry[];
  raw: JsonRecord;
}

export interface ExecutionSummaryResponse {
  generated_at: string;
  total: number;
  success: number;
  failed: number;
  in_progress: number;
  success_rate: number;
  pending?: number;
  blocked?: number;
  planning?: number;
  awaiting_approval?: number;
  validating?: number;
  rolled_back?: number;
  current_task?: ExecutionSummaryItem;
  current_execution?: ExecutionSummaryItem;
  recent_failure?: ExecutionSummaryItem;
  tasks: ExecutionSummaryItem[];
  executions: ExecutionSummaryItem[];
  timeline?: ExecutionTimelineEntry[];
}

export interface ObservabilityEvent {
  message: string;
  plane: string;
  status: string;
  timestamp: string;
  tool: string;
}

export interface SelfTestChecklistItem {
  label: string;
  selector: string;
}

export interface VisualSelfTestSample {
  checklist: SelfTestChecklistItem[];
  task: string;
  title: string;
  url: string;
}

export interface ResumeSessionResponse {
  messages?: unknown;
  redirect_url?: string;
  trajectory_id?: string;
  websocket_url?: string;
}

export interface SettingsConfig {
  provider: string;
  model: string;
  base_url: string;
  api_key: string;
}

export interface ResumeSessionHydration {
  messages: ChatMessage[];
  trajectoryId: string;
  websocketURL: string;
}

export interface ChatDomainBridge {
  hydrateResumeSession: (payload: ResumeSessionHydration) => void;
  insertPrompt: (text: string) => void;
}

export type NotificationKind = 'info' | 'success' | 'error';

export interface NotificationItem {
  id: number;
  kind: NotificationKind;
  message: string;
}

export interface ObservabilityDomainBridge {
  handleEvent: (event: ObservabilityEvent) => void;
}

function pickRecord(record: JsonRecord, keys: string[]): JsonRecord {
  for (const key of keys) {
    const value = record[key];
    if (isRecord(value)) {
      return value;
    }
  }
  return record;
}

function readNumber(record: JsonRecord, keys: string[]): number | undefined {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === 'string') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }
  }
  return undefined;
}

function normalizeExecutionStatus(status: string): string {
  const value = status.trim().toLowerCase();
  if (!value) {
    return 'unknown';
  }
  switch (value) {
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
      return value;
  }
}

function normalizeExecutionItem(item: JsonRecord, index: number): ExecutionSummaryItem {
  const normalizedStatus = normalizeExecutionStatus(pickString(item, [
    'status',
    'state',
    'phase',
    'current_status',
    'result_status',
  ]));
  const reference = pickString(item, [
    'reference',
    'title',
    'summary',
    'description',
    'name',
    'task',
  ]);
  return {
    id: pickString(item, ['id', 'execution_id', 'executionId', 'task_id', 'taskId', 'uuid']) || `execution-${index + 1}`,
    reference: reference || pickString(item, ['prompt', 'input', 'goal']) || `Execution ${index + 1}`,
    status: normalizedStatus,
    updated_at: pickString(item, ['updated_at', 'updatedAt', 'last_updated_at', 'lastUpdatedAt', 'created_at', 'createdAt', 'timestamp']),
    title: pickString(item, ['title', 'summary', 'description', 'name']),
    summary: pickString(item, ['summary', 'description']),
    stage: pickString(item, ['stage', 'phase']),
    execution_id: pickString(item, ['execution_id', 'executionId']),
    checkpoint_id: pickString(item, ['checkpoint_id', 'checkpointId']),
    rollback_point: pickString(item, ['rollback_point', 'rollbackPoint']),
    raw: item,
  };
}

function normalizeExecutionTimelineEntry(item: JsonRecord, index: number): ExecutionTimelineEntry {
  return {
    id: pickString(item, ['id', 'step_id', 'stepId', 'checkpoint_id', 'checkpointId']) || `timeline-${index + 1}`,
    status: normalizeExecutionStatus(pickString(item, ['status', 'state', 'phase'])),
    title: pickString(item, ['title', 'name', 'summary', 'description', 'action']) || `Step ${index + 1}`,
    updated_at: pickString(item, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp']),
    time: pickString(item, ['time', 'timestamp', 'updated_at', 'updatedAt']),
    message: pickString(item, ['message', 'summary', 'description']),
    type: pickString(item, ['type', 'kind']),
    kind: pickString(item, ['kind', 'type']),
    step_id: pickString(item, ['step_id', 'stepId']),
    checkpoint_id: pickString(item, ['checkpoint_id', 'checkpointId']),
    summary: pickString(item, ['summary', 'description']),
    raw: item,
  };
}

function normalizeExecutionStepEntry(item: JsonRecord, index: number): ExecutionStepEntry {
  return {
    id: pickString(item, ['id', 'step_id', 'stepId', 'checkpoint_id', 'checkpointId']) || `step-${index + 1}`,
    kind: pickString(item, ['kind', 'type']) || 'step',
    title: pickString(item, ['title', 'name', 'summary', 'description']) || `Step ${index + 1}`,
    status: normalizeExecutionStatus(pickString(item, ['status', 'state', 'phase'])),
    started_at: pickString(item, ['started_at', 'startedAt', 'created_at', 'createdAt', 'timestamp']),
    finished_at: pickString(item, ['finished_at', 'finishedAt', 'updated_at', 'updatedAt', 'timestamp']),
    summary: pickString(item, ['summary', 'description']),
    raw: item,
  };
}

function collectExecutionItems(source: JsonRecord): JsonRecord[] {
  const items = extractCollection(source, ['executions', 'tasks', 'items', 'data', 'results', 'records']);
  if (items.length > 0) {
    return items;
  }
  const nested = pickRecord(source, ['summary', 'execution_summary', 'data', 'result']);
  return extractCollection(nested, ['executions', 'tasks', 'items', 'data', 'results', 'records']);
}

function pickExecutionSource(payload: unknown): JsonRecord {
  const root = isRecord(payload) ? payload : {};
  for (const key of ['summary', 'execution_summary', 'data', 'result']) {
    const nested = root[key];
    if (isRecord(nested)) {
      return nested;
    }
  }
  return root;
}

function pickSingleExecutionItem(source: JsonRecord, keys: string[]): ExecutionSummaryItem | undefined {
  for (const key of keys) {
    const value = source[key];
    if (isRecord(value)) {
      return normalizeExecutionItem(value, 0);
    }
  }
  return undefined;
}

function countStatuses(items: ExecutionSummaryItem[], wanted: string[]): number {
  const lookup = new Set(wanted);
  return items.reduce((total, item) => total + (lookup.has(item.status) ? 1 : 0), 0);
}

function inferInProgressCount(items: ExecutionSummaryItem[]): number {
  return items.reduce((total, item) => {
    switch (item.status) {
      case 'pending':
      case 'running':
      case 'validating':
      case 'blocked':
      case 'planning':
      case 'awaiting_approval':
        return total + 1;
      default:
        return total;
    }
  }, 0);
}

export function normalizeExecutionSummary(payload: unknown): ExecutionSummaryResponse {
  const source = pickExecutionSource(payload);
  const rawItems = collectExecutionItems(source);
  const tasks = rawItems.map((item, index) => normalizeExecutionItem(item, index));
  const executions = tasks;
  const currentTask = pickSingleExecutionItem(source, [
    'current_task',
    'current_execution',
    'active_task',
    'current',
    'current_item',
  ]) ?? tasks.find((item) => item.status !== 'success' && item.status !== 'failed') ?? tasks[0];
  const recentFailure = pickSingleExecutionItem(source, [
    'recent_failure',
    'latest_failure',
    'last_failure',
    'failure',
  ]) ?? tasks.find((item) => item.status === 'failed' || item.status === 'blocked');

  const success = readNumber(source, ['success', 'succeeded', 'completed']) ?? countStatuses(tasks, ['success']);
  const failed = readNumber(source, ['failed', 'failures', 'error']) ?? countStatuses(tasks, ['failed']);
  const inProgress = readNumber(source, ['in_progress', 'running', 'active']) ?? inferInProgressCount(tasks);
  const total = readNumber(source, ['total', 'count', 'size']) ?? tasks.length;
  const completed = success + failed;
  const successRate = readNumber(source, ['success_rate', 'successRate'])
    ?? (completed > 0 ? (success / completed) * 100 : 0);

  return {
    generated_at: pickString(source, ['generated_at', 'generatedAt', 'updated_at', 'updatedAt', 'timestamp']) || new Date().toISOString(),
    total,
    success,
    failed,
    in_progress: inProgress,
    success_rate: successRate,
    pending: readNumber(source, ['pending']),
    blocked: readNumber(source, ['blocked']),
    planning: readNumber(source, ['planning']),
    awaiting_approval: readNumber(source, ['awaiting_approval', 'awaitingApproval']),
    validating: readNumber(source, ['validating']),
    rolled_back: readNumber(source, ['rolled_back', 'rolledBack']),
    current_task: currentTask,
    current_execution: currentTask,
    recent_failure: recentFailure,
    tasks,
    executions,
    timeline: extractCollection(source, ['timeline', 'events', 'history']).map((item, index) => normalizeExecutionTimelineEntry(item, index)),
  };
}

export function normalizeExecutionDetail(payload: unknown): ExecutionDetailResponse {
  const source = isRecord(payload) ? payload : {};
  const executionRecord = isRecord(source.execution)
    ? normalizeExecutionItem(source.execution, 0)
    : pickSingleExecutionItem(source, ['execution', 'item', 'data']);

  return {
    execution: executionRecord ?? null,
    timeline: extractCollection(source, ['timeline', 'events', 'history']).map((item, index) => normalizeExecutionTimelineEntry(item, index)),
    steps: extractCollection(source, ['steps', 'items', 'data']).map((item, index) => normalizeExecutionStepEntry(item, index)),
    raw: source,
  };
}

export function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

export function normalizeApprovalRequest(payload: unknown): ApprovalRequest {
  const data = isRecord(payload) ? payload : {};
  const rawChunks = Array.isArray(data.chunks) ? data.chunks : [];
  return {
    id: typeof data.id === 'string' ? data.id : '',
    tool: typeof data.tool === 'string' ? data.tool : '',
    category: typeof data.category === 'string' ? data.category : 'tool_execution',
    summary: typeof data.summary === 'string' ? data.summary : 'The tool request requires human confirmation.',
    args: typeof data.args === 'string' ? data.args : '',
    preview: typeof data.preview === 'string' ? data.preview : '',
    chunks: rawChunks.map((item) => {
      const chunk = isRecord(item) ? item : {};
      return {
        id: typeof chunk.id === 'string' ? chunk.id : '',
        header: typeof chunk.header === 'string' ? chunk.header : '',
        preview: typeof chunk.preview === 'string' ? chunk.preview : '',
      };
    }).filter((chunk) => chunk.id !== ''),
    metadata: isRecord(data.metadata) ? data.metadata : {},
  };
}

export function normalizeChatHistory(payload: unknown): ChatMessage[] {
  if (!Array.isArray(payload)) {
    return [];
  }

  const messages: ChatMessage[] = [];

  for (const item of payload) {
    if (!isRecord(item)) {
      continue;
    }

    const role = typeof item.Role === 'string'
      ? item.Role
      : typeof item.role === 'string'
        ? item.role
        : '';
    const content = typeof item.Content === 'string'
      ? item.Content
      : typeof item.content === 'string'
        ? item.content
        : '';

    const toolCalls = Array.isArray(item.ToolCalls)
      ? item.ToolCalls
      : Array.isArray(item.tool_calls)
        ? item.tool_calls
        : [];

    if (role === 'user' || role === 'assistant') {
      if (content.trim()) {
        messages.push({ role, content });
      }
      for (const toolCall of toolCalls) {
        if (!isRecord(toolCall)) {
          continue;
        }
        const name = typeof toolCall.Name === 'string'
          ? toolCall.Name
          : typeof toolCall.name === 'string'
            ? toolCall.name
            : '';
        const args = typeof toolCall.Args === 'string'
          ? toolCall.Args
          : typeof toolCall.args === 'string'
            ? toolCall.args
            : '';
        if (name) {
          messages.push({
            role: 'tool',
            toolData: {
              name,
              args,
              status: 'completed',
            },
          });
        }
      }
      continue;
    }

    if (role === 'tool') {
      const name = typeof item.Name === 'string'
        ? item.Name
        : typeof item.name === 'string'
          ? item.name
          : 'tool';
      messages.push({
        role: 'tool',
        toolData: {
          name,
          args: '',
          result: content,
          status: 'completed',
        },
      });
    }
  }

  return messages;
}

export function buildWebSocketURL(
  resumeTrajectoryId: string,
  locale: string,
  explicitURL = '',
): string {
  if (explicitURL.trim()) {
    const url = new URL(explicitURL, window.location.origin);
    url.searchParams.delete('token');
    if (locale) {
      url.searchParams.set('locale', locale);
    }
    return url.toString();
  }

  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const params = new URLSearchParams();
  if (resumeTrajectoryId) {
    params.set('resume_trajectory', resumeTrajectoryId);
  }
  if (locale) {
    params.set('locale', locale);
  }
  const suffix = params.toString();
  return `${proto}://${window.location.host}/ws${suffix ? `?${suffix}` : ''}`;
}

export function formatCodeActionPrompt(
  t: (key: string, ...args: unknown[]) => string,
  currentFile: string | null,
  code: string,
): string {
  const extension = currentFile?.split('.').pop() || '';
  return t('types.code_action_prompt', extension, code);
}

export function parseSpecialistToolArgs(raw: string): JsonRecord {
  try {
    const parsed = JSON.parse(raw);
    return isRecord(parsed) ? parsed : {};
  } catch {
    return {};
  }
}
