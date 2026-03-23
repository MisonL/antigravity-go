import React, { useEffect, useMemo, useState } from 'react';

interface McpToolInfo {
  name: string;
  description?: string;
}

interface McpServerInfo {
  name: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  status?: string;
  tool_count: number;
  tools?: McpToolInfo[];
}

interface MethodProbe {
  requested: string;
  supported: boolean;
  evidence?: string;
}

interface McpCapabilities {
  add?: MethodProbe;
  stop?: MethodProbe;
  restart?: MethodProbe;
  invoke?: MethodProbe;
}

interface McpResponse {
  servers?: McpServerInfo[];
  capabilities?: McpCapabilities;
  warning?: string;
}

interface McpPanelProps {
  onClose: () => void;
  token: string;
}

function parseEnvText(raw: string): Record<string, string> {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'))
    .reduce<Record<string, string>>((acc, line) => {
      const index = line.indexOf('=');
      if (index <= 0) {
        return acc;
      }
      const key = line.slice(0, index).trim();
      const value = line.slice(index + 1).trim();
      if (key) {
        acc[key] = value;
      }
      return acc;
    }, {});
}

function formatEnvText(env?: Record<string, string>): string {
  if (!env) {
    return '';
  }
  return Object.entries(env)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('\n');
}

export function McpPanel({ onClose, token }: McpPanelProps) {
  const [servers, setServers] = useState<McpServerInfo[]>([]);
  const [capabilities, setCapabilities] = useState<McpCapabilities>({});
  const [warning, setWarning] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [formName, setFormName] = useState('');
  const [formCommand, setFormCommand] = useState('');
  const [formArgs, setFormArgs] = useState('');
  const [formEnv, setFormEnv] = useState('');

  const suffix = useMemo(() => (
    token ? `?token=${encodeURIComponent(token)}` : ''
  ), [token]);

  async function fetchServers() {
    setLoading(true);
    setError('');

    try {
      const resp = await fetch(`/api/mcp${suffix}`);
      if (!resp.ok) {
        throw new Error(`MCP 列表请求失败: ${resp.status}`);
      }

      const data = await resp.json() as McpResponse;
      setServers(data.servers ?? []);
      setCapabilities(data.capabilities ?? {});
      setWarning(data.warning ?? '');
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : '加载 MCP 配置失败。');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchServers();
  }, [suffix]);

  async function submitServer(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError('');

    try {
      const resp = await fetch(`/api/mcp${suffix}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: formName.trim(),
          command: formCommand.trim(),
          args: formArgs.split(/\s+/).map((item) => item.trim()).filter(Boolean),
          env: parseEnvText(formEnv),
        }),
      });
      if (!resp.ok) {
        throw new Error(await resp.text() || `保存失败: ${resp.status}`);
      }

      setFormName('');
      setFormCommand('');
      setFormArgs('');
      setFormEnv('');
      await fetchServers();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : '保存 MCP 配置失败。');
    } finally {
      setSaving(false);
    }
  }

  async function deleteServer(name: string) {
    setSaving(true);
    setError('');

    try {
      const resp = await fetch(`/api/mcp${suffix}`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      });
      if (!resp.ok) {
        throw new Error(await resp.text() || `删除失败: ${resp.status}`);
      }
      await fetchServers();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : '删除 MCP 服务失败。');
    } finally {
      setSaving(false);
    }
  }

  async function restartServer(name: string) {
    setSaving(true);
    setError('');

    try {
      const resp = await fetch(`/api/mcp${suffix}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'restart', name }),
      });
      if (!resp.ok) {
        throw new Error(await resp.text() || `重启失败: ${resp.status}`);
      }
      await fetchServers();
    } catch (restartError) {
      setError(restartError instanceof Error ? restartError.message : '重启 MCP 服务失败。');
    } finally {
      setSaving(false);
    }
  }

  function fillForm(server: McpServerInfo) {
    setFormName(server.name);
    setFormCommand(server.command ?? '');
    setFormArgs((server.args ?? []).join(' '));
    setFormEnv(formatEnvText(server.env));
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="glass-panel modal-content data-modal mcp-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <h3>MCP 能力管理</h3>
          <button type="button" onClick={onClose}>X</button>
        </div>

        <div className="data-modal-shell">
          <section className="data-list-panel">
            <div className="data-list-toolbar">
              <div>
                <div className="data-section-title">服务列表</div>
                <div className="data-section-subtitle">展示当前内核识别到的 MCP 服务与工具数量</div>
              </div>
              <button type="button" className="btn-secondary" onClick={fetchServers} disabled={loading || saving}>
                刷新
              </button>
            </div>

            <div className="mcp-capability-grid">
              <span className={`badge ${capabilities.add?.supported ? 'success' : 'error'}`}>Add: {capabilities.add?.requested ?? '-'}</span>
              <span className={`badge ${capabilities.restart?.supported ? 'success' : 'error'}`}>Restart: {capabilities.restart?.requested ?? '-'}</span>
              <span className={`badge ${capabilities.invoke?.supported ? 'success' : 'error'}`}>Invoke: {capabilities.invoke?.requested ?? '-'}</span>
            </div>

            {warning && <div className="data-state">{warning}</div>}
            {error && <div className="data-state data-state-error">{error}</div>}
            {loading && servers.length === 0 && <div className="data-state">正在加载 MCP 服务...</div>}
            {!loading && servers.length === 0 && <div className="data-state">当前没有 MCP 服务。</div>}

            <div className="mcp-server-list">
              {servers.map((server) => (
                <article key={server.name} className="mcp-server-card">
                  <div className="mcp-server-card__header">
                    <div>
                      <div className="mcp-server-card__title">{server.name}</div>
                      <div className="mcp-server-card__meta">
                        {server.command || '未暴露 command'} {server.status ? `| ${server.status}` : ''}
                      </div>
                    </div>
                    <span className="badge info">tools: {server.tool_count}</span>
                  </div>

                  {server.args && server.args.length > 0 && (
                    <pre className="data-json">{JSON.stringify(server.args, null, 2)}</pre>
                  )}

                  {server.tools && server.tools.length > 0 && (
                    <div className="mcp-tool-list">
                      {server.tools.slice(0, 6).map((tool) => (
                        <div key={`${server.name}-${tool.name}`} className="mcp-tool-chip">
                          {tool.name}
                        </div>
                      ))}
                    </div>
                  )}

                  <div className="mcp-server-card__actions">
                    <button type="button" className="btn-secondary" onClick={() => fillForm(server)}>载入表单</button>
                    <button type="button" className="btn-secondary" onClick={() => restartServer(server.name)} disabled={saving}>重启</button>
                    <button type="button" className="btn-secondary btn-danger" onClick={() => deleteServer(server.name)} disabled={saving}>删除</button>
                  </div>
                </article>
              ))}
            </div>
          </section>

          <section className="data-detail-panel">
            <div className="data-detail-header">
              <div>
                <div className="data-section-title">新增或更新服务</div>
                <div className="data-section-subtitle">通过声明式配置写入 core，并触发动态工具刷新</div>
              </div>
            </div>

            <form className="mcp-form" onSubmit={submitServer}>
              <label className="form-group">
                <span>服务名称</span>
                <input value={formName} onChange={(event) => setFormName(event.target.value)} placeholder="例如 postgres" />
              </label>

              <label className="form-group">
                <span>命令</span>
                <input value={formCommand} onChange={(event) => setFormCommand(event.target.value)} placeholder="例如 npx" />
              </label>

              <label className="form-group">
                <span>参数</span>
                <input value={formArgs} onChange={(event) => setFormArgs(event.target.value)} placeholder="按空格分隔，例如 -y @modelcontextprotocol/server-postgres" />
              </label>

              <label className="form-group">
                <span>环境变量</span>
                <textarea
                  value={formEnv}
                  onChange={(event) => setFormEnv(event.target.value)}
                  placeholder={'一行一个 KEY=VALUE\n例如 DATABASE_URL=postgres://user:pass@host/db'}
                  rows={8}
                />
              </label>

              <div className="modal-footer">
                <button type="button" className="btn-secondary" onClick={onClose}>关闭</button>
                <button type="submit" className="btn-primary" disabled={saving}>保存并刷新工具</button>
              </div>
            </form>
          </section>
        </div>
      </div>
    </div>
  );
}
