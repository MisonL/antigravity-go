import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from 'react';
import { useI18n, type TranslateFn } from '../i18n/useI18n';
import type {
  ChatDomainBridge,
  NotificationItem,
  NotificationKind,
  ObservabilityDomainBridge,
} from './types';

interface AppDomainProviderProps extends PropsWithChildren {
  initialResumeTrajectoryId: string;
}

interface AppDomainContextValue {
  chatBridge: ChatDomainBridge | null;
  currentFile: string | null;
  fileRefreshTrigger: number;
  initialResumeTrajectoryId: string;
  observabilityBridge: ObservabilityDomainBridge | null;
  resumeTrajectoryId: string;
  resumeWebSocketURL: string;
  locale: string;
  showNotification: (message: string, kind?: NotificationKind) => void;
  setChatBridge: (bridge: ChatDomainBridge | null) => void;
  setCurrentFile: (path: string | null) => void;
  setLocale: (locale: string) => void;
  setObservabilityBridge: (bridge: ObservabilityDomainBridge | null) => void;
  setResumeTrajectoryId: (id: string) => void;
  setResumeWebSocketURL: (url: string) => void;
  setShowTerminal: (show: boolean) => void;
  showTerminal: boolean;
  t: TranslateFn;
  touchFileRefresh: () => void;
}

interface NotificationContextValue {
  dismissNotification: (id: number) => void;
  notifications: NotificationItem[];
  showNotification: (message: string, kind?: NotificationKind) => void;
}

const AppDomainContext = createContext<AppDomainContextValue | null>(null);
const NotificationContext = createContext<NotificationContextValue | null>(null);

export function AppDomainProvider({
  children,
  initialResumeTrajectoryId,
}: AppDomainProviderProps) {
  const [currentFile, setCurrentFile] = useState<string | null>(null);
  const [showTerminal, setShowTerminal] = useState(false);
  const [fileRefreshTrigger, setFileRefreshTrigger] = useState(0);
  const [resumeTrajectoryId, setResumeTrajectoryId] = useState(initialResumeTrajectoryId);
  const [resumeWebSocketURL, setResumeWebSocketURL] = useState('');
  const [chatBridge, setChatBridge] = useState<ChatDomainBridge | null>(null);
  const [observabilityBridge, setObservabilityBridge] = useState<ObservabilityDomainBridge | null>(null);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const { locale, setLocale, t } = useI18n();
  const notificationTimersRef = useRef<Record<number, number>>({});
  const touchFileRefresh = useCallback(() => {
    setFileRefreshTrigger(Date.now());
  }, []);

  const dismissNotification = useCallback((id: number) => {
    const timer = notificationTimersRef.current[id];
    if (timer) {
      window.clearTimeout(timer);
      delete notificationTimersRef.current[id];
    }
    setNotifications((current) => current.filter((item) => item.id !== id));
  }, []);

  const showNotification = useCallback((message: string, kind: NotificationKind = 'info') => {
    const id = Date.now() + Math.floor(Math.random() * 1000);
    setNotifications((current) => [...current, { id, kind, message }]);
    notificationTimersRef.current[id] = window.setTimeout(() => {
      delete notificationTimersRef.current[id];
      setNotifications((current) => current.filter((item) => item.id !== id));
    }, 3200);
  }, []);

  const notificationValue = useMemo<NotificationContextValue>(() => ({
    dismissNotification,
    notifications,
    showNotification,
  }), [dismissNotification, notifications, showNotification]);

  const value = useMemo<AppDomainContextValue>(() => ({
    chatBridge,
    currentFile,
    fileRefreshTrigger,
    initialResumeTrajectoryId,
    locale,
    observabilityBridge,
    resumeTrajectoryId,
    resumeWebSocketURL,
    showNotification,
    setChatBridge,
    setCurrentFile,
    setLocale,
    setObservabilityBridge,
    setResumeTrajectoryId,
    setResumeWebSocketURL,
    setShowTerminal,
    showTerminal,
    t,
    touchFileRefresh,
  }), [
    chatBridge,
    currentFile,
    fileRefreshTrigger,
    initialResumeTrajectoryId,
    locale,
    observabilityBridge,
    resumeTrajectoryId,
    resumeWebSocketURL,
    setLocale,
    showNotification,
    showTerminal,
    t,
    touchFileRefresh,
  ]);

  return (
    <NotificationContext.Provider value={notificationValue}>
      <AppDomainContext.Provider value={value}>
        {children}
      </AppDomainContext.Provider>
    </NotificationContext.Provider>
  );
}

export function useAppDomain() {
  const context = useContext(AppDomainContext);
  if (!context) {
    throw new Error('useAppDomain must be used within AppDomainProvider');
  }
  return context;
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within AppDomainProvider');
  }
  return context;
}
