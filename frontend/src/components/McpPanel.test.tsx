import { screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithApp } from '../test/renderWithApp';
import { McpPanel } from './McpPanel';

describe('McpPanel', () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('switches to read-only mode when management is unavailable', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({
      capabilities: {
        add: { requested: 'AddMcpServer', supported: false },
        invoke: { requested: 'InvokeMcpTool', supported: false },
        restart: { requested: 'RestartMcpServer', supported: false },
      },
      servers: [
        {
          name: 'filesystem',
          status: 'running',
          tool_count: 1,
          tools: [{ name: 'read_file' }],
        },
      ],
    }), {
      headers: { 'Content-Type': 'application/json' },
      status: 200,
    }));

    renderWithApp(
      <McpPanel
        access={{
          allowInvoke: false,
          allowManage: false,
          readOnly: true,
          show: true,
          showResources: false,
        }}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId('mcp-read-only')).toBeInTheDocument();
    });

    expect(screen.getByText('当前内核未验证支持 MCP 管理动作，面板已切换为只读模式。')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '重启' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '保存并刷新工具' })).not.toBeInTheDocument();
  });
});
