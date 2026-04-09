import { useCallback, useEffect, useState } from 'react';
import { LoadingState, StateMessage } from './AsyncState';
import { SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';
import { useAsyncResource } from '../domains/useAsyncResource';
import { getErrorMessage, type SettingsConfig } from '../domains/types';

interface SettingsModalProps {
  open: boolean;
  onClose: () => void;
}

const defaultConfig: SettingsConfig = {
  api_key: '',
  base_url: '',
  model: '',
  provider: 'openai',
};

export function SettingsModal({ open, onClose }: SettingsModalProps) {
  const { locale, setLocale, showNotification, t } = useAppDomain();
  const {
    data: config,
    error: configLoadError,
    loading: configLoading,
    run: loadConfig,
    setData: setConfigState,
  } = useAsyncResource<SettingsConfig>({
    initialLoading: false,
    initialValue: defaultConfig,
  });
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);
  const [hasLoaded, setHasLoaded] = useState(false);

  useEffect(() => {
    if (!open || hasLoaded) {
      return;
    }
    void loadConfig(async () => {
      const resp = await fetch('/api/config');
      if (!resp.ok) {
        throw new Error(t('settings.load_failed'));
      }
      return await resp.json() as SettingsConfig;
    }, {
      onError: (error) => getErrorMessage(error, t('settings.load_error')),
      onSuccess: async () => {
        setHasLoaded(true);
      },
    });
  }, [hasLoaded, loadConfig, open, t]);

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
      const resp = await fetch('/api/config', {
        body: JSON.stringify(config),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
      });
      if (!resp.ok) {
        throw new Error(t('settings.save_failed'));
      }
      onClose();
      showNotification(t('settings.save_success'), 'success');
    } catch (error) {
      const message = getErrorMessage(error, t('settings.save_error'));
      setSaveError(message);
      showNotification(message, 'error');
    } finally {
      setSaving(false);
    }
  }, [config, onClose, showNotification, t]);

  if (!open) {
    return null;
  }

  const provider = config.provider;

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
          <button aria-label={t('common.close')} onClick={onClose} type="button">X</button>
        </div>
        <div className="modal-body" aria-busy={configLoading || saving}>
          {configLoading ? (
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
              <StateMessage kind="error" message={configLoadError} />
              <div className="form-group">
                <label htmlFor="provider">{t('settings.provider')}</label>
                <select
                  id="provider"
                  disabled={saving}
                  onChange={(event) => setConfig((current) => ({ ...current, provider: event.target.value }))}
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
                <select id="language" disabled={saving} onChange={(event) => setLocale(event.target.value)} value={locale}>
                  <option value="zh-CN">{t('settings.language.zh')}</option>
                  <option value="en-US">{t('settings.language.en')}</option>
                </select>
              </div>

              <div className="form-group">
                <label htmlFor="model">{t('settings.model')}</label>
                <input
                  id="model"
                  disabled={saving}
                  onChange={(event) => setConfig((current) => ({ ...current, model: event.target.value }))}
                  placeholder={
                    provider === 'ollama'
                      ? t('settings.placeholder.model.ollama')
                      : provider === 'anthropic'
                        ? t('settings.placeholder.model.anthropic')
                        : t('settings.placeholder.model.default')
                  }
                  type="text"
                  value={config.model}
                />
              </div>

              <div className="form-group">
                <label htmlFor="base_url">{t('settings.base_url')}</label>
                <input
                  id="base_url"
                  disabled={saving}
                  onChange={(event) => setConfig((current) => ({ ...current, base_url: event.target.value }))}
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
                  value={config.base_url}
                />
              </div>

              <div className="form-group">
                <label htmlFor="api_key">{t('settings.api_key')}</label>
                <input
                  id="api_key"
                  disabled={saving}
                  onChange={(event) => setConfig((current) => ({ ...current, api_key: event.target.value }))}
                  placeholder={t('settings.placeholder.api_key')}
                  type="password"
                  value={config.api_key}
                />
              </div>
              <StateMessage kind="error" message={saveError} />
            </>
          )}
        </div>
        <div className="modal-footer">
          <button className="btn-secondary" disabled={saving} onClick={onClose} type="button">{t('common.cancel')}</button>
          <button
            className={`btn-primary${saving ? ' is-busy' : ''}`}
            disabled={saving || configLoading}
            onClick={() => void handleSaveConfig()}
            type="button"
            aria-busy={saving}
          >
            <span>{saving ? t('common.saving') : t('settings.button.save')}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
