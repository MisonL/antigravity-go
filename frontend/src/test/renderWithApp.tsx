import type { PropsWithChildren, ReactElement } from 'react';
import { render } from '@testing-library/react';
import { AppDomainProvider } from '../domains/AppDomainContext';
import { I18N_STORAGE_KEY } from '../i18n/useI18n';

function Wrapper({ children }: PropsWithChildren) {
  window.localStorage.setItem(I18N_STORAGE_KEY, 'zh-CN');
  return <AppDomainProvider initialResumeTrajectoryId="">{children}</AppDomainProvider>;
}

export function renderWithApp(ui: ReactElement) {
  return render(ui, { wrapper: Wrapper });
}
