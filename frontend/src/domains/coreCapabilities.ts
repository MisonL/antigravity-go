import { isRecord } from '../components/planeData';

export interface MethodProbe {
  evidence?: string;
  requested: string;
  supported: boolean;
}

export interface McpRPCSupport {
  add: MethodProbe;
  invoke: MethodProbe;
  refresh: MethodProbe;
  restart: MethodProbe;
  stop: MethodProbe;
}

export interface CoreCapabilities {
  apply_edit: MethodProbe;
  browser_click: MethodProbe;
  browser_focus: MethodProbe;
  browser_list: MethodProbe;
  browser_open: MethodProbe;
  browser_screenshot: MethodProbe;
  browser_scroll: MethodProbe;
  browser_type: MethodProbe;
  commit_message: MethodProbe;
  code_frequency: MethodProbe;
  diagnostics: MethodProbe;
  edit_preview: MethodProbe;
  experiments: MethodProbe;
  heartbeat: MethodProbe;
  mcp_control: McpRPCSupport;
  mcp_enabled: MethodProbe;
  mcp_resources: MethodProbe;
  mcp_servers: MethodProbe;
  mcp_setting: MethodProbe;
  mcp_states: MethodProbe;
  memory_query: MethodProbe;
  memory_save: MethodProbe;
  repo_info: MethodProbe;
  rollback: MethodProbe;
  rules: MethodProbe;
  run_command: MethodProbe;
  trajectory_export: MethodProbe;
  trajectory_get: MethodProbe;
  trajectory_list: MethodProbe;
  validation: MethodProbe;
  workspace_track: MethodProbe;
}

export interface CoreCapabilitiesResponse {
  capabilities: CoreCapabilities;
  http_port: number;
  ready: boolean;
}

export interface CapabilityPolicy {
  browser: {
    allowInteract: boolean;
    showRead: boolean;
  };
  mcp: {
    allowInvoke: boolean;
    allowManage: boolean;
    readOnly: boolean;
    show: boolean;
    showResources: boolean;
  };
  memory: {
    allowSave: boolean;
    showQuery: boolean;
  };
  observability: {
    showCodeFrequency: boolean;
  };
  trajectory: {
    allowResume: boolean;
    allowRollback: boolean;
    showDetail: boolean;
    showList: boolean;
  };
}

const EMPTY_PROBE: MethodProbe = {
  requested: '',
  supported: false,
};

const EMPTY_MCP_CONTROL: McpRPCSupport = {
  add: EMPTY_PROBE,
  invoke: EMPTY_PROBE,
  refresh: EMPTY_PROBE,
  restart: EMPTY_PROBE,
  stop: EMPTY_PROBE,
};

export const EMPTY_CORE_CAPABILITIES: CoreCapabilities = {
  apply_edit: EMPTY_PROBE,
  browser_click: EMPTY_PROBE,
  browser_focus: EMPTY_PROBE,
  browser_list: EMPTY_PROBE,
  browser_open: EMPTY_PROBE,
  browser_screenshot: EMPTY_PROBE,
  browser_scroll: EMPTY_PROBE,
  browser_type: EMPTY_PROBE,
  commit_message: EMPTY_PROBE,
  code_frequency: EMPTY_PROBE,
  diagnostics: EMPTY_PROBE,
  edit_preview: EMPTY_PROBE,
  experiments: EMPTY_PROBE,
  heartbeat: EMPTY_PROBE,
  mcp_control: EMPTY_MCP_CONTROL,
  mcp_enabled: EMPTY_PROBE,
  mcp_resources: EMPTY_PROBE,
  mcp_servers: EMPTY_PROBE,
  mcp_setting: EMPTY_PROBE,
  mcp_states: EMPTY_PROBE,
  memory_query: EMPTY_PROBE,
  memory_save: EMPTY_PROBE,
  repo_info: EMPTY_PROBE,
  rollback: EMPTY_PROBE,
  rules: EMPTY_PROBE,
  run_command: EMPTY_PROBE,
  trajectory_export: EMPTY_PROBE,
  trajectory_get: EMPTY_PROBE,
  trajectory_list: EMPTY_PROBE,
  validation: EMPTY_PROBE,
  workspace_track: EMPTY_PROBE,
};

export const EMPTY_CORE_CAPABILITIES_RESPONSE: CoreCapabilitiesResponse = {
  capabilities: EMPTY_CORE_CAPABILITIES,
  http_port: 0,
  ready: false,
};

function normalizeMethodProbe(value: unknown): MethodProbe {
  if (!isRecord(value)) {
    return EMPTY_PROBE;
  }
  return {
    evidence: typeof value.evidence === 'string' ? value.evidence : undefined,
    requested: typeof value.requested === 'string' ? value.requested : '',
    supported: value.supported === true,
  };
}

function normalizeMcpRPCSupport(value: unknown): McpRPCSupport {
  if (!isRecord(value)) {
    return EMPTY_MCP_CONTROL;
  }
  return {
    add: normalizeMethodProbe(value.add),
    invoke: normalizeMethodProbe(value.invoke),
    refresh: normalizeMethodProbe(value.refresh),
    restart: normalizeMethodProbe(value.restart),
    stop: normalizeMethodProbe(value.stop),
  };
}

export function normalizeCoreCapabilitiesResponse(value: unknown): CoreCapabilitiesResponse {
  if (!isRecord(value)) {
    return EMPTY_CORE_CAPABILITIES_RESPONSE;
  }

  const caps = isRecord(value.capabilities) ? value.capabilities : {};

  return {
    capabilities: {
      apply_edit: normalizeMethodProbe(caps.apply_edit),
      browser_click: normalizeMethodProbe(caps.browser_click),
      browser_focus: normalizeMethodProbe(caps.browser_focus),
      browser_list: normalizeMethodProbe(caps.browser_list),
      browser_open: normalizeMethodProbe(caps.browser_open),
      browser_screenshot: normalizeMethodProbe(caps.browser_screenshot),
      browser_scroll: normalizeMethodProbe(caps.browser_scroll),
      browser_type: normalizeMethodProbe(caps.browser_type),
      commit_message: normalizeMethodProbe(caps.commit_message),
      code_frequency: normalizeMethodProbe(caps.code_frequency),
      diagnostics: normalizeMethodProbe(caps.diagnostics),
      edit_preview: normalizeMethodProbe(caps.edit_preview),
      experiments: normalizeMethodProbe(caps.experiments),
      heartbeat: normalizeMethodProbe(caps.heartbeat),
      mcp_control: normalizeMcpRPCSupport(caps.mcp_control),
      mcp_enabled: normalizeMethodProbe(caps.mcp_enabled),
      mcp_resources: normalizeMethodProbe(caps.mcp_resources),
      mcp_servers: normalizeMethodProbe(caps.mcp_servers),
      mcp_setting: normalizeMethodProbe(caps.mcp_setting),
      mcp_states: normalizeMethodProbe(caps.mcp_states),
      memory_query: normalizeMethodProbe(caps.memory_query),
      memory_save: normalizeMethodProbe(caps.memory_save),
      repo_info: normalizeMethodProbe(caps.repo_info),
      rollback: normalizeMethodProbe(caps.rollback),
      rules: normalizeMethodProbe(caps.rules),
      run_command: normalizeMethodProbe(caps.run_command),
      trajectory_export: normalizeMethodProbe(caps.trajectory_export),
      trajectory_get: normalizeMethodProbe(caps.trajectory_get),
      trajectory_list: normalizeMethodProbe(caps.trajectory_list),
      validation: normalizeMethodProbe(caps.validation),
      workspace_track: normalizeMethodProbe(caps.workspace_track),
    },
    http_port: typeof value.http_port === 'number' ? value.http_port : 0,
    ready: value.ready === true,
  };
}

export function deriveCapabilityPolicy(response: CoreCapabilitiesResponse | null): CapabilityPolicy {
  const caps = response?.capabilities ?? EMPTY_CORE_CAPABILITIES;
  const mcpRead = caps.mcp_states.supported || caps.mcp_servers.supported;
  const mcpResources = caps.mcp_resources.supported;
  const mcpManage = caps.mcp_control.add.supported || caps.mcp_control.refresh.supported || caps.mcp_control.restart.supported;
  const mcpInvoke = caps.mcp_control.invoke.supported;
  const trajectoryList = caps.trajectory_list.supported;
  const trajectoryDetail = trajectoryList && caps.trajectory_get.supported;

  return {
    browser: {
      allowInteract: caps.browser_click.supported || caps.browser_type.supported || caps.browser_scroll.supported,
      showRead: caps.browser_list.supported || caps.browser_open.supported || caps.browser_focus.supported || caps.browser_screenshot.supported,
    },
    mcp: {
      allowInvoke: mcpInvoke,
      allowManage: mcpManage,
      readOnly: (mcpRead || mcpResources) && !mcpManage && !mcpInvoke,
      show: mcpRead || mcpResources || mcpManage || mcpInvoke,
      showResources: mcpResources,
    },
    memory: {
      allowSave: caps.memory_save.supported,
      showQuery: caps.memory_query.supported,
    },
    observability: {
      showCodeFrequency: caps.code_frequency.supported,
    },
    trajectory: {
      allowResume: trajectoryDetail,
      allowRollback: trajectoryDetail && caps.rollback.supported,
      showDetail: trajectoryDetail,
      showList: trajectoryList,
    },
  };
}
