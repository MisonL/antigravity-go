export type JsonRecord = Record<string, unknown>;

export interface TrajectorySummary {
  id: string;
  status: string;
  title: string;
  updatedAt: string;
  raw: JsonRecord;
}

export interface MemorySummary {
  id: string;
  category: string;
  content: string;
  updatedAt: string;
  raw: JsonRecord;
}

export interface TrajectoryStepSummary {
  id: string;
  status: string;
  title: string;
  updatedAt: string;
  raw: JsonRecord;
}

export function isRecord(value: unknown): value is JsonRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function pickString(record: JsonRecord, keys: string[]): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      return String(value);
    }
  }
  return '';
}

export function formatValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '';
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch (_error) {
    return String(value);
  }
}

export function summarizeValue(value: unknown, fallback = ''): string {
  const formatted = formatValue(value).replace(/\s+/g, ' ').trim();
  return formatted || fallback;
}

function toRecordArray(value: unknown): JsonRecord[] {
  if (Array.isArray(value)) {
    return value.filter(isRecord);
  }
  if (isRecord(value)) {
    const values = Object.values(value);
    if (values.length > 0 && values.every(isRecord)) {
      return values as JsonRecord[];
    }
  }
  return [];
}

function toNestedRecordArray(value: unknown, preferredKeys: string[]): JsonRecord[] {
  const directItems = toRecordArray(value);
  if (directItems.length > 0) {
    return directItems;
  }

  if (!isRecord(value)) {
    return [];
  }

  for (const key of preferredKeys) {
    const nested = toRecordArray(value[key]);
    if (nested.length > 0) {
      return nested;
    }
  }

  for (const nestedValue of Object.values(value)) {
    const nested = toNestedRecordArray(nestedValue, preferredKeys);
    if (nested.length > 0) {
      return nested;
    }
  }

  return [];
}

export function extractCollection(value: unknown, preferredKeys: string[]): JsonRecord[] {
  const directItems = toRecordArray(value);
  if (directItems.length > 0) {
    return directItems;
  }

  if (!isRecord(value)) {
    return [];
  }

  for (const key of preferredKeys) {
    const nestedItems = toRecordArray(value[key]);
    if (nestedItems.length > 0) {
      return nestedItems;
    }
  }

  return [];
}

export function normalizeTrajectories(payload: unknown): TrajectorySummary[] {
  const items = extractCollection(payload, ['trajectories', 'items', 'data', 'results', 'records']);

  return items.map((item, index) => ({
    id: pickString(item, ['id', 'trajectory_id', 'trajectoryId', 'uuid']) || `trajectory-${index + 1}`,
    status: pickString(item, ['status', 'state']) || 'unknown',
    title: pickString(item, ['title', 'name', 'summary', 'description']),
    updatedAt: pickString(item, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp']),
    raw: item,
  }));
}

export function normalizeMemories(payload: unknown): MemorySummary[] {
  const items = extractCollection(payload, ['memories', 'items', 'data', 'results', 'records']);

  return items.map((item, index) => ({
    id: pickString(item, ['id', 'memory_id', 'memoryId', 'key']) || `memory-${index + 1}`,
    category: pickString(item, ['category', 'type', 'namespace', 'kind']) || 'uncategorized',
    content:
      summarizeValue(
        item.content ?? item.text ?? item.summary ?? item.value ?? item.message,
        summarizeValue(item, 'empty memory'),
      ),
    updatedAt: pickString(item, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp']),
    raw: item,
  }));
}

export function normalizeTrajectorySteps(payload: JsonRecord | null): TrajectoryStepSummary[] {
  if (!payload) {
    return [];
  }

  const items = toNestedRecordArray(payload, ['steps', 'nodes', 'checkpoints', 'timeline', 'history', 'events']);
  const seen = new Set<string>();

  return items.flatMap((item, index) => {
    const id = pickString(item, ['step_id', 'stepId', 'id', 'node_id', 'nodeId', 'checkpoint_id']);
    if (!id || seen.has(id)) {
      return [];
    }

    seen.add(id);
    return [{
      id,
      status: pickString(item, ['status', 'state']) || 'unknown',
      title: pickString(item, ['title', 'name', 'summary', 'description', 'action']) || `Step ${index + 1}`,
      updatedAt: pickString(item, ['updated_at', 'updatedAt', 'created_at', 'createdAt', 'timestamp']),
      raw: item,
    }];
  });
}
