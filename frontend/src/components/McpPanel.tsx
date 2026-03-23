import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { SkeletonCardList, SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

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
  const { t } = useAppDomain();
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

  const fetchServers = useCallback(async () => {
    setLoading(true);
    setError('');

    try {
      const resp = await fetch(`/api/mcp${suffix}`);
      if (!resp.ok) {
        throw new Error(`MCP request failed: ${resp.status}`);
      }

      const data = await resp.json() as McpResponse;
      setServers(data.servers ?? []);
      setCapabilities(data.capabilities ?? {});
      setWarning(data.warning ?? '');
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : t('mcp.error.fetch'));
    } finally {
      setLoading(false);
    }
  }, [suffix, t]);

  useEffect(() => {
    void fetchServers();
  }, [fetchServers]);

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
        throw new Error(await resp.text() || `Save failed: ${resp.status}`);
      }

      setFormName('');
      setFormCommand('');
      setFormArgs('');
      setFormEnv('');
      await fetchServers();
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t('mcp.error.save'));
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
        throw new Error(await resp.text() || `Delete failed: ${resp.status}`);
      }
      await fetchServers();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : t('mcp.error.delete'));
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
        throw new Error(await resp.text() || `Restart failed: ${resp.status}`);
      }
      await fetchServers();
    } catch (restartError) {
      setError(restartError instanceof Error ? restartError.message : t('mcp.error.restart'));
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
      <div
        className="glass-panel modal-content data-modal mcp-modal"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="mcp-modal-title"
      >
        <div className="modal-header">
          <h3 id="mcp-modal-title">{t('mcp.title')}</h3>
          <button type="button" onClick={onClose} aria-label={t('common.close')}>X</button>
        </div>

        <div className="data-modal-shell">
          <section className="data-list-panel">
            <div className="data-list-toolbar">
              <div>
                <div className="data-section-title">{t('mcp.list.title')}</div>
                <div className="data-section-subtitle">{t('mcp.list.subtitle')}</div>
              </div>
              <button type="button" className="btn-secondary" onClick={fetchServers} disabled={loading || saving}>
                {t('common.refresh')}
              </button>
            </div>

            <div className="mcp-capability-grid">
              <span className={`badge ${capabilities.add?.supported ? 'success' : 'error'}`}>Add: {capabilities.add?.requested ?? '-'}</span>
              <span className={`badge ${capabilities.restart?.supported ? 'success' : 'error'}`}>Restart: {capabilities.restart?.requested ?? '-'}</span>
              <span className={`badge ${capabilities.invoke?.supported ? 'success' : 'error'}`}>Invoke: {capabilities.invoke?.requested ?? '-'}</span>
            </div>

            {warning && <div className="data-state">{warning}</div>}
            {error && <div className="data-state data-state-error">{error}</div>}
            {loading && servers.length === 0 && (
              <div className="loading-shell">
                <div className="data-state">{t('mcp.loading')}</div>
                <SkeletonCardList cards={3} lines={3} />
              </div>
            )}
            {!loading && servers.length === 0 && <div className="data-state">{t('mcp.empty')}</div>}

            <div className="mcp-server-list">
              {servers.map((server) => (
                <article key={server.name} className="mcp-server-card">
                  <div className="mcp-server-card__header">
                    <div>
                      <div className="mcp-server-card__title">{server.name}</div>
                      <div className="mcp-server-card__meta">
                        {server.command || t('mcp.command.hidden')} {server.status ? `| ${server.status}` : ''}
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
                    <button type="button" className="btn-secondary" onClick={() => fillForm(server)}>{t('mcp.form.load')}</button>
                    <button type="button" className="btn-secondary" onClick={() => restartServer(server.name)} disabled={saving}>{t('mcp.action.restart')}</button>
                    <button type="button" className="btn-secondary btn-danger" onClick={() => deleteServer(server.name)} disabled={saving}>{t('mcp.action.delete')}</button>
                  </div>
                </article>
              ))}
            </div>
          </section>

          <section className="data-detail-panel">
            <div className="data-detail-header">
              <div>
                <div className="data-section-title">{t('mcp.form.title')}</div>
                <div className="data-section-subtitle">{t('mcp.form.subtitle')}</div>
              </div>
            </div>

            <form className="mcp-form" onSubmit={submitServer} aria-busy={saving}>
              {loading && servers.length === 0 && <SkeletonRows lines={4} />}
              <label className="form-group">
                <span>{t('mcp.form.name')}</span>
                <input id="mcp-name" name="mcp-name" value={formName} onChange={(event) => setFormName(event.target.value)} placeholder={t('mcp.form.name.placeholder')} disabled={saving} />
              </label>

              <label className="form-group">
                <span>{t('mcp.form.command')}</span>
                <input id="mcp-command" name="mcp-command" value={formCommand} onChange={(event) => setFormCommand(event.target.value)} placeholder={t('mcp.form.command.placeholder')} disabled={saving} />
              </label>

              <label className="form-group">
                <span>{t('mcp.form.args')}</span>
                <input id="mcp-args" name="mcp-args" value={formArgs} onChange={(event) => setFormArgs(event.target.value)} placeholder={t('mcp.form.args.placeholder')} disabled={saving} />
              </label>

              <label className="form-group">
                <span>{t('mcp.form.env')}</span>
                <textarea
                  id="mcp-env"
                  name="mcp-env"
                  value={formEnv}
                  onChange={(event) => setFormEnv(event.target.value)}
                  placeholder={t('mcp.form.env.placeholder')}
                  rows={8}
                  disabled={saving}
                />
              </label>

              <div className="modal-footer">
                <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>{t('common.close')}</button>
                <button type="submit" className={`btn-primary${saving ? ' is-busy' : ''}`} disabled={saving} aria-busy={saving}>
                  <span>{saving ? t('common.saving') : t('mcp.form.submit')}</span>
                </button>
              </div>
            </form>
          </section>
        </div>
      </div>
    </div>
  );
}
