import {
  createContext,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react';
import type {
  ChatDomainBridge,
  ObservabilityDomainBridge,
} from './types';

interface AppDomainProviderProps extends PropsWithChildren {
  initialResumeTrajectoryId: string;
  token: string;
}

interface AppDomainContextValue {
  chatBridge: ChatDomainBridge | null;
  currentFile: string | null;
  fileRefreshTrigger: number;
  initialResumeTrajectoryId: string;
  observabilityBridge: ObservabilityDomainBridge | null;
  resumeTrajectoryId: string;
  resumeWebSocketURL: string;
  setChatBridge: (bridge: ChatDomainBridge | null) => void;
  setCurrentFile: (path: string | null) => void;
  setObservabilityBridge: (bridge: ObservabilityDomainBridge | null) => void;
  setResumeTrajectoryId: (id: string) => void;
  setResumeWebSocketURL: (url: string) => void;
  setShowTerminal: (show: boolean) => void;
  showTerminal: boolean;
  token: string;
  touchFileRefresh: () => void;
}

const AppDomainContext = createContext<AppDomainContextValue | null>(null);

export function AppDomainProvider({
  children,
  initialResumeTrajectoryId,
  token,
}: AppDomainProviderProps) {
  const [currentFile, setCurrentFile] = useState<string | null>(null);
  const [showTerminal, setShowTerminal] = useState(true);
  const [fileRefreshTrigger, setFileRefreshTrigger] = useState(0);
  const [resumeTrajectoryId, setResumeTrajectoryId] = useState(initialResumeTrajectoryId);
  const [resumeWebSocketURL, setResumeWebSocketURL] = useState('');
  const [chatBridge, setChatBridge] = useState<ChatDomainBridge | null>(null);
  const [observabilityBridge, setObservabilityBridge] = useState<ObservabilityDomainBridge | null>(null);

  const value = useMemo<AppDomainContextValue>(() => ({
    chatBridge,
    currentFile,
    fileRefreshTrigger,
    initialResumeTrajectoryId,
    observabilityBridge,
    resumeTrajectoryId,
    resumeWebSocketURL,
    setChatBridge,
    setCurrentFile,
    setObservabilityBridge,
    setResumeTrajectoryId,
    setResumeWebSocketURL,
    setShowTerminal,
    showTerminal,
    token,
    touchFileRefresh: () => setFileRefreshTrigger(Date.now()),
  }), [
    chatBridge,
    currentFile,
    fileRefreshTrigger,
    initialResumeTrajectoryId,
    observabilityBridge,
    resumeTrajectoryId,
    resumeWebSocketURL,
    showTerminal,
    token,
  ]);

  return (
    <AppDomainContext.Provider value={value}>
      {children}
    </AppDomainContext.Provider>
  );
}

export function useAppDomain() {
  const context = useContext(AppDomainContext);
  if (!context) {
    throw new Error('useAppDomain 必须在 AppDomainProvider 内使用');
  }
  return context;
}
