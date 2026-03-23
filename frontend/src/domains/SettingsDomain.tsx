import { useCallback, useEffect, useState } from 'react';
import { useAppDomain } from './AppDomainContext';
import {
  getErrorMessage,
  type SettingsConfig,
} from './types';

export interface SettingsDomainState {
  config: SettingsConfig;
  handleSaveConfig: () => Promise<void>;
  saveError: string;
  setConfig: (updater: SettingsConfig | ((current: SettingsConfig) => SettingsConfig)) => void;
  setShowSettings: (show: boolean) => void;
  showSettings: boolean;
}

interface SettingsModalProps {
  settings: SettingsDomainState;
}

const defaultConfig: SettingsConfig = {
  api_key: '',
  base_url: '',
  model: '',
  provider: 'openai',
};

export function useSettingsDomain(): SettingsDomainState {
  const { token } = useAppDomain();
  const [showSettings, setShowSettings] = useState(false);
  const [config, setConfigState] = useState<SettingsConfig>(defaultConfig);
  const [saveError, setSaveError] = useState('');

  const suffix = token ? `?token=${encodeURIComponent(token)}` : '';

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const resp = await fetch(`/api/config${suffix}`);
        if (!resp.ok) {
          return;
        }
        setConfigState(await resp.json() as SettingsConfig);
      } catch (error) {
        console.error('Failed to fetch config', error);
      }
    };

    void fetchConfig();
  }, [suffix]);

  const setConfig = useCallback((updater: SettingsConfig | ((current: SettingsConfig) => SettingsConfig)) => {
    setConfigState((current) => (
      typeof updater === 'function'
        ? (updater as (value: SettingsConfig) => SettingsConfig)(current)
        : updater
    ));
  }, []);

  const handleSaveConfig = useCallback(async () => {
    setSaveError('');

    try {
      const resp = await fetch(`/api/config${suffix}`, {
        body: JSON.stringify(config),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
      });
      if (!resp.ok) {
        throw new Error('保存失败。');
      }
      setShowSettings(false);
      window.alert('配置已保存并应用。');
    } catch (error) {
      setSaveError(getErrorMessage(error, '保存配置失败。'));
    }
  }, [config, suffix]);

  return {
    config,
    handleSaveConfig,
    saveError,
    setConfig,
    setShowSettings,
    showSettings,
  };
}

export function SettingsModal({ settings }: SettingsModalProps) {
  if (!settings.showSettings) {
    return null;
  }

  const provider = settings.config.provider;

  return (
    <div className="modal-overlay">
      <div className="glass-panel modal-content settings-modal">
        <div className="modal-header">
          <h3>AI 渠道配置</h3>
          <button onClick={() => settings.setShowSettings(false)} type="button">X</button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label htmlFor="provider">AI 渠道类型</label>
            <select
              id="provider"
              onChange={(event) => settings.setConfig((current) => ({ ...current, provider: event.target.value }))}
              value={provider}
            >
              <option value="openai">OpenAI 兼容 (Chat v1)</option>
              <option value="openai-legacy">OpenAI 兼容 (Legacy)</option>
              <option value="anthropic">Anthropic 兼容</option>
              <option value="gemini">Google Gemini</option>
              <option value="ollama">Ollama (本地)</option>
              <option value="lmstudio">LM Studio (本地)</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="model">模型名称</label>
            <input
              id="model"
              onChange={(event) => settings.setConfig((current) => ({ ...current, model: event.target.value }))}
              placeholder={
                provider === 'ollama'
                  ? '例如 qwen2.5-coder:7b'
                  : provider === 'anthropic'
                    ? '例如 claude-3-5-sonnet-latest'
                    : '例如 gpt-4o 或 qwen3-max'
              }
              type="text"
              value={settings.config.model}
            />
          </div>

          <div className="form-group">
            <label htmlFor="base_url">接口地址</label>
            <input
              id="base_url"
              onChange={(event) => settings.setConfig((current) => ({ ...current, base_url: event.target.value }))}
              placeholder={
                provider === 'ollama'
                  ? 'http://localhost:11434/v1'
                  : provider === 'lmstudio'
                    ? 'http://localhost:1234/v1'
                    : provider === 'anthropic'
                      ? 'https://api.anthropic.com'
                      : '例如 https://api.openai.com/v1'
              }
              type="text"
              value={settings.config.base_url}
            />
          </div>

          <div className="form-group">
            <label htmlFor="api_key">API 密钥</label>
            <input
              id="api_key"
              onChange={(event) => settings.setConfig((current) => ({ ...current, api_key: event.target.value }))}
              placeholder="填入 API Key，本地模型可留空"
              type="password"
              value={settings.config.api_key}
            />
          </div>
          {settings.saveError && <div className="data-state data-state-error">{settings.saveError}</div>}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" onClick={() => settings.setShowSettings(false)} type="button">取消</button>
          <button className="btn-primary" onClick={() => void settings.handleSaveConfig()} type="button">保存配置</button>
        </div>
      </div>
    </div>
  );
}
