import { useCallback, useEffect, useState } from 'react';
import { LoadingState, StateMessage } from '../components/AsyncState';
import { SkeletonRows } from '../components/Skeleton';
import { useAppDomain } from './AppDomainContext';
import {
  buildTokenQuery,
  getErrorMessage,
  type SettingsConfig,
} from './types';
import { useAsyncResource } from './useAsyncResource';

export interface SettingsDomainState {
  config: SettingsConfig;
  configLoading: boolean;
  configLoadError: string;
  handleSaveConfig: () => Promise<void>;
  saveError: string;
  saving: boolean;
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
  const { showNotification, t, token } = useAppDomain();
  const [showSettings, setShowSettings] = useState(false);
  const {
    data: config,
    error: configLoadError,
    loading: configLoading,
    run: loadConfig,
    setData: setConfigState,
  } = useAsyncResource<SettingsConfig>({
    initialLoading: true,
    initialValue: defaultConfig,
  });
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);

  const suffix = buildTokenQuery(token);

  useEffect(() => {
    void loadConfig(async () => {
      const resp = await fetch(`/api/config${suffix}`);
      if (!resp.ok) {
        throw new Error(t('settings.load_failed'));
      }
      return await resp.json() as SettingsConfig;
    }, {
      onError: (error) => getErrorMessage(error, t('settings.load_error')),
    });
  }, [loadConfig, suffix, t]);

  const setConfig = useCallback((updater: SettingsConfig | ((current: SettingsConfig) => SettingsConfig)) => {
    setConfigState((current) => (
      typeof updater === 'function'
        ? (updater as (value: SettingsConfig) => SettingsConfig)(current)
        : updater
    ));
  }, [setConfigState]);

  const handleSaveConfig = useCallback(async () => {
    setSaveError('');
    setSaving(true);

    try {
      const resp = await fetch(`/api/config${suffix}`, {
        body: JSON.stringify(config),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
      });
      if (!resp.ok) {
        throw new Error(t('settings.save_failed'));
      }
      setShowSettings(false);
      showNotification(t('settings.save_success'), 'success');
    } catch (error) {
      setSaveError(getErrorMessage(error, t('settings.save_error')));
      showNotification(getErrorMessage(error, t('settings.save_error')), 'error');
    } finally {
      setSaving(false);
    }
  }, [config, showNotification, suffix, t]);

  return {
    config,
    configLoading,
    configLoadError,
    handleSaveConfig,
    saveError,
    saving,
    setConfig,
    setShowSettings,
    showSettings,
  };
}

export function SettingsModal({ settings }: SettingsModalProps) {
  const { locale, setLocale, t } = useAppDomain();

  if (!settings.showSettings) {
    return null;
  }

  const provider = settings.config.provider;

  return (
    <div className="modal-overlay">
      <div
        className="glass-panel modal-content settings-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-modal-title"
      >
        <div className="modal-header">
          <h3 id="settings-modal-title">{t('settings.title')}</h3>
          <button aria-label={t('common.close')} onClick={() => settings.setShowSettings(false)} type="button">X</button>
        </div>
        <div className="modal-body" aria-busy={settings.configLoading || settings.saving}>
          {settings.configLoading ? (
            <LoadingState
              message={t('settings.loading')}
              skeleton={(
                <>
                  <SkeletonRows lines={3} />
                  <SkeletonRows lines={3} />
                  <SkeletonRows lines={3} />
                  <SkeletonRows lines={3} />
                </>
              )}
            />
          ) : (
            <>
              <StateMessage kind="error" message={settings.configLoadError} />
          <div className="form-group">
            <label htmlFor="provider">{t('settings.provider')}</label>
            <select
              id="provider"
              disabled={settings.saving}
              onChange={(event) => settings.setConfig((current) => ({ ...current, provider: event.target.value }))}
              value={provider}
            >
              <option value="openai">{t('settings.provider.openai')}</option>
              <option value="openai-legacy">{t('settings.provider.openai_legacy')}</option>
              <option value="anthropic">{t('settings.provider.anthropic')}</option>
              <option value="gemini">{t('settings.provider.gemini')}</option>
              <option value="ollama">{t('settings.provider.ollama')}</option>
              <option value="lmstudio">{t('settings.provider.lmstudio')}</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="language">{t('settings.language')}</label>
            <select id="language" disabled={settings.saving} onChange={(event) => setLocale(event.target.value)} value={locale}>
              <option value="zh-CN">{t('settings.language.zh')}</option>
              <option value="en-US">{t('settings.language.en')}</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="model">{t('settings.model')}</label>
            <input
              id="model"
              disabled={settings.saving}
              onChange={(event) => settings.setConfig((current) => ({ ...current, model: event.target.value }))}
              placeholder={
                provider === 'ollama'
                  ? t('settings.placeholder.model.ollama')
                  : provider === 'anthropic'
                    ? t('settings.placeholder.model.anthropic')
                    : t('settings.placeholder.model.default')
              }
              type="text"
              value={settings.config.model}
            />
          </div>

          <div className="form-group">
            <label htmlFor="base_url">{t('settings.base_url')}</label>
            <input
              id="base_url"
              disabled={settings.saving}
              onChange={(event) => settings.setConfig((current) => ({ ...current, base_url: event.target.value }))}
              placeholder={
                provider === 'ollama'
                  ? 'http://localhost:11434/v1'
                  : provider === 'lmstudio'
                    ? 'http://localhost:1234/v1'
                    : provider === 'anthropic'
                      ? 'https://api.anthropic.com'
                      : t('settings.placeholder.base_url.default')
              }
              type="text"
              value={settings.config.base_url}
            />
          </div>

          <div className="form-group">
            <label htmlFor="api_key">{t('settings.api_key')}</label>
            <input
              id="api_key"
              disabled={settings.saving}
              onChange={(event) => settings.setConfig((current) => ({ ...current, api_key: event.target.value }))}
              placeholder={t('settings.placeholder.api_key')}
              type="password"
              value={settings.config.api_key}
            />
          </div>
              <StateMessage kind="error" message={settings.saveError} />
            </>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" disabled={settings.saving} onClick={() => settings.setShowSettings(false)} type="button">{t('common.cancel')}</button>
          <button
            className={`btn-primary${settings.saving ? ' is-busy' : ''}`}
            disabled={settings.saving || settings.configLoading}
            onClick={() => void settings.handleSaveConfig()}
            type="button"
            aria-busy={settings.saving}
          >
            <span>{settings.saving ? t('common.saving') : t('settings.button.save')}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
