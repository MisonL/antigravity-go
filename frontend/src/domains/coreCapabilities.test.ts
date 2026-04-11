import { describe, expect, it } from 'vitest';
import {
  deriveCapabilityPolicy,
  normalizeCoreCapabilitiesResponse,
} from './coreCapabilities';

describe('coreCapabilities', () => {
  it('derives conservative surface policy from raw probes', () => {
    const response = normalizeCoreCapabilitiesResponse({
      capabilities: {
        mcp_control: {
          refresh: { requested: 'RefreshMcpServers', supported: true },
        },
        code_frequency: { requested: 'GetCodeFrequencyForRepo', supported: true },
        mcp_resources: { requested: 'ListMcpResources', supported: true },
        mcp_states: { requested: 'GetMcpServerStates', supported: true },
        memory_query: { requested: 'GetUserMemories', supported: true },
        memory_save: { requested: 'UpdateCascadeMemory', supported: true },
        rollback: { requested: 'RevertToCascadeStep', supported: true },
        trajectory_get: { requested: 'GetCascadeTrajectory', supported: false },
        trajectory_list: { requested: 'GetAllCascadeTrajectories', supported: true },
      },
      http_port: 9999,
      ready: true,
    });

    const policy = deriveCapabilityPolicy(response);

    expect(policy.trajectory.showList).toBe(true);
    expect(policy.trajectory.showDetail).toBe(false);
    expect(policy.trajectory.allowResume).toBe(false);
    expect(policy.trajectory.allowRollback).toBe(false);
    expect(policy.memory.showQuery).toBe(true);
    expect(policy.memory.allowSave).toBe(true);
    expect(policy.observability.showCodeFrequency).toBe(true);
    expect(policy.mcp.show).toBe(true);
    expect(policy.mcp.readOnly).toBe(false);
    expect(policy.mcp.allowManage).toBe(true);
    expect(policy.mcp.allowInvoke).toBe(false);
    expect(policy.mcp.showResources).toBe(true);
  });
});
