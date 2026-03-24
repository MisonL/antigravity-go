import type { ApprovalRequest } from '../components/ApprovalModal';
import {
  isRecord,
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
  token: string,
  resumeTrajectoryId: string,
  locale: string,
  explicitURL = '',
): string {
  if (explicitURL.trim()) {
    const url = new URL(explicitURL, window.location.origin);
    if (locale) {
      url.searchParams.set('locale', locale);
    }
    return url.toString();
  }

  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const params = new URLSearchParams();
  if (token) {
    params.set('token', token);
  }
  if (resumeTrajectoryId) {
    params.set('resume_trajectory', resumeTrajectoryId);
  }
  if (locale) {
    params.set('locale', locale);
  }
  const suffix = params.toString();
  return `${proto}://${window.location.host}/ws${suffix ? `?${suffix}` : ''}`;
}

export function buildTokenQuery(token: string): string {
  return token ? `?token=${encodeURIComponent(token)}` : '';
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
