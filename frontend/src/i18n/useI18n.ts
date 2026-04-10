import { useEffect, useMemo, useState } from 'react';
import enUS from './en-US.json';
import zhCN from './zh-CN.json';

export const I18N_STORAGE_KEY = 'ago.locale';

export const SUPPORTED_LOCALES = ['zh-CN', 'en-US'] as const;

export type Locale = typeof SUPPORTED_LOCALES[number];
export type TranslateFn = (key: string, ...args: unknown[]) => string;

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': zhCN,
  'en-US': enUS,
};

export function normalizeLocale(value: string | null | undefined): Locale {
  const raw = (value ?? '').trim().replaceAll('_', '-').toLowerCase();
  if (raw.startsWith('zh')) {
    return 'zh-CN';
  }
  return 'en-US';
}

function detectInitialLocale(): Locale {
  if (typeof window === 'undefined') {
    return 'zh-CN';
  }
  const stored = window.localStorage.getItem(I18N_STORAGE_KEY);
  if (stored) {
    return normalizeLocale(stored);
  }
  return normalizeLocale(window.navigator.language);
}

function format(template: string, args: unknown[]): string {
  let result = template;
  for (const [index, value] of args.entries()) {
    result = result.replaceAll(`{${index}}`, String(value));
  }
  return result;
}

function createTranslator(locale: Locale): TranslateFn {
  return (key: string, ...args: unknown[]) => {
    const template = messages[locale][key] ?? messages['en-US'][key] ?? key;
    return args.length === 0 ? template : format(template, args);
  };
}

export function useI18n() {
  const [locale, setLocaleState] = useState<Locale>(() => detectInitialLocale());

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    window.localStorage.setItem(I18N_STORAGE_KEY, locale);
    document.documentElement.lang = locale;
    document.title = messages[locale]['app.meta.title'] ?? messages['en-US']['app.meta.title'] ?? document.title;
  }, [locale]);

  const setLocale = (nextLocale: string) => {
    setLocaleState(normalizeLocale(nextLocale));
  };

  const t = useMemo(() => createTranslator(locale), [locale]);

  return {
    locale,
    setLocale,
    supportedLocales: SUPPORTED_LOCALES,
    t,
  };
}
