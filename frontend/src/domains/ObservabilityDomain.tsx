import { useCallback, useEffect, useMemo, useState } from 'react';
import type { TaskSummaryResponse } from '../components/ScoreboardPanel';
import {
  normalizeMemories,
  normalizeTrajectories,
  normalizeTrajectorySteps,
  type JsonRecord,
  type MemorySummary,
  type TrajectorySummary,
} from '../components/planeData';
import { useAppDomain } from './AppDomainContext';
import {
  getErrorMessage,
  normalizeChatHistory,
  type ObservabilityEvent,
  type ObservabilitySummary,
  type ResumeSessionResponse,
  type VisualSelfTestSample,
} from './types';

export interface ObservabilityDomainState {
  handleOpenMemoryModal: () => Promise<void>;
  handleOpenTrajectoryModal: () => Promise<void>;
  handleOpenVisualSelfTestModal: () => Promise<void>;
  latestObservabilityEvent: ObservabilityEvent | null;
  memories: MemorySummary[];
  memoriesError: string;
  memoriesLoading: boolean;
  observabilityError: string;
  observabilitySummary: ObservabilitySummary | null;
  resumeError: string;
  resumeLoadingId: string;
  resumeSuccess: string;
  rollbackError: string;
  rollbackStepId: string;
  rollbackSuccess: string;
  selectedTrajectoryDetail: JsonRecord | null;
  selectedTrajectoryId: string;
  setShowMcpPanel: (show: boolean) => void;
  showMcpPanel: boolean;
  showMemoryModal: boolean;
  showTrajectoryModal: boolean;
  showVisualSelfTestModal: boolean;
  taskSummary: TaskSummaryResponse | null;
  taskSummaryError: string;
  trajectoryDetailError: string;
  trajectoryDetailLoading: boolean;
  trajectorySteps: ReturnType<typeof normalizeTrajectorySteps>;
  trajectories: TrajectorySummary[];
  trajectoriesError: string;
  trajectoriesLoading: boolean;
  visualSelfTestError: string;
  visualSelfTestLoading: boolean;
  visualSelfTestSample: VisualSelfTestSample | null;
  fetchMemories: (force?: boolean) => Promise<void>;
  fetchTrajectories: (force?: boolean) => Promise<void>;
  fetchVisualSelfTestSample: (force?: boolean) => Promise<void>;
  resumeTrajectorySession: (trajectoryId: string, syncLocation?: boolean, showSuccess?: boolean) => Promise<void>;
  rollbackToStep: (stepId: string) => Promise<void>;
  setShowMemoryModal: (show: boolean) => void;
  setShowTrajectoryModal: (show: boolean) => void;
  setShowVisualSelfTestModal: (show: boolean) => void;
  setVisualSelfTestTask: (task: string) => void;
  fetchTrajectoryDetail: (id: string, force?: boolean) => Promise<void>;
}

export function useObservabilityDomain(): ObservabilityDomainState {
  const {
    chatBridge,
    initialResumeTrajectoryId,
    setObservabilityBridge,
    token,
  } = useAppDomain();

  const [showTrajectoryModal, setShowTrajectoryModal] = useState(false);
  const [showMemoryModal, setShowMemoryModal] = useState(false);
  const [showMcpPanel, setShowMcpPanel] = useState(false);
  const [showVisualSelfTestModal, setShowVisualSelfTestModal] = useState(false);
  const [trajectories, setTrajectories] = useState<TrajectorySummary[]>([]);
  const [trajectoriesLoading, setTrajectoriesLoading] = useState(false);
  const [trajectoriesError, setTrajectoriesError] = useState('');
  const [selectedTrajectoryId, setSelectedTrajectoryId] = useState('');
  const [selectedTrajectoryDetail, setSelectedTrajectoryDetail] = useState<JsonRecord | null>(null);
  const [trajectoryDetailLoading, setTrajectoryDetailLoading] = useState(false);
  const [trajectoryDetailError, setTrajectoryDetailError] = useState('');
  const [rollbackStepId, setRollbackStepId] = useState('');
  const [rollbackError, setRollbackError] = useState('');
  const [rollbackSuccess, setRollbackSuccess] = useState('');
  const [resumeLoadingId, setResumeLoadingId] = useState('');
  const [resumeError, setResumeError] = useState('');
  const [resumeSuccess, setResumeSuccess] = useState('');
  const [memories, setMemories] = useState<MemorySummary[]>([]);
  const [memoriesLoading, setMemoriesLoading] = useState(false);
  const [memoriesError, setMemoriesError] = useState('');
  const [observabilitySummary, setObservabilitySummary] = useState<ObservabilitySummary | null>(null);
  const [observabilityError, setObservabilityError] = useState('');
  const [taskSummary, setTaskSummary] = useState<TaskSummaryResponse | null>(null);
  const [taskSummaryError, setTaskSummaryError] = useState('');
  const [latestObservabilityEvent, setLatestObservabilityEvent] = useState<ObservabilityEvent | null>(null);
  const [visualSelfTestLoading, setVisualSelfTestLoading] = useState(false);
  const [visualSelfTestError, setVisualSelfTestError] = useState('');
  const [visualSelfTestSample, setVisualSelfTestSample] = useState<VisualSelfTestSample | null>(null);

  const suffix = useMemo(() => (token ? `?token=${encodeURIComponent(token)}` : ''), [token]);
  const trajectorySteps = useMemo(
    () => normalizeTrajectorySteps(selectedTrajectoryDetail),
    [selectedTrajectoryDetail],
  );

  const fetchObservabilitySummary = useCallback(async () => {
    setObservabilityError('');
    try {
      const resp = await fetch(`/api/observability/summary${suffix}`);
      if (!resp.ok) {
        throw new Error(`可观测性摘要请求失败: ${resp.status}`);
      }
      setObservabilitySummary(await resp.json() as ObservabilitySummary);
    } catch (error) {
      setObservabilityError(getErrorMessage(error, '加载可观测性摘要失败。'));
    }
  }, [suffix]);

  const fetchTaskSummary = useCallback(async () => {
    setTaskSummaryError('');
    try {
      const resp = await fetch(`/api/tasks${suffix}`);
      if (!resp.ok) {
        throw new Error(`任务摘要请求失败: ${resp.status}`);
      }
      setTaskSummary(await resp.json() as TaskSummaryResponse);
    } catch (error) {
      setTaskSummaryError(getErrorMessage(error, '加载任务摘要失败。'));
    }
  }, [suffix]);

  const fetchTrajectoryDetail = useCallback(async (id: string, force = false) => {
    if (!id) {
      return;
    }
    if (!force && id === selectedTrajectoryId && selectedTrajectoryDetail) {
      return;
    }

    setSelectedTrajectoryId(id);
    setTrajectoryDetailLoading(true);
    setTrajectoryDetailError('');

    try {
      const resp = await fetch(`/api/trajectories/${encodeURIComponent(id)}${suffix}`);
      if (!resp.ok) {
        throw new Error(`轨迹详情请求失败: ${resp.status}`);
      }
      const data = await resp.json();
      if (!data || typeof data !== 'object') {
        throw new Error('轨迹详情格式无效');
      }
      setSelectedTrajectoryDetail(data as JsonRecord);
      setRollbackError('');
      setRollbackSuccess('');
    } catch (error) {
      setSelectedTrajectoryDetail(null);
      setTrajectoryDetailError(getErrorMessage(error, '加载轨迹详情失败。'));
    } finally {
      setTrajectoryDetailLoading(false);
    }
  }, [selectedTrajectoryDetail, selectedTrajectoryId, suffix]);

  const fetchTrajectories = useCallback(async (force = false) => {
    if (!force && trajectories.length > 0) {
      return;
    }

    setTrajectoriesLoading(true);
    setTrajectoriesError('');

    try {
      const resp = await fetch(`/api/trajectories${suffix}`);
      if (!resp.ok) {
        throw new Error(`轨迹列表请求失败: ${resp.status}`);
      }
      const normalized = normalizeTrajectories(await resp.json());
      setTrajectories(normalized);
      await fetchObservabilitySummary();

      if (normalized.length === 0) {
        setSelectedTrajectoryId('');
        setSelectedTrajectoryDetail(null);
        setTrajectoryDetailError('');
        return;
      }

      const nextId = normalized.some((item) => item.id === selectedTrajectoryId)
        ? selectedTrajectoryId
        : normalized[0].id;
      await fetchTrajectoryDetail(nextId, force);
    } catch (error) {
      setTrajectoriesError(getErrorMessage(error, '加载轨迹列表失败。'));
    } finally {
      setTrajectoriesLoading(false);
    }
  }, [fetchObservabilitySummary, fetchTrajectoryDetail, selectedTrajectoryId, suffix, trajectories.length]);

  const fetchMemories = useCallback(async (force = false) => {
    if (!force && memories.length > 0) {
      return;
    }

    setMemoriesLoading(true);
    setMemoriesError('');

    try {
      const resp = await fetch(`/api/memories${suffix}`);
      if (!resp.ok) {
        throw new Error(`记忆列表请求失败: ${resp.status}`);
      }
      setMemories(normalizeMemories(await resp.json()));
      await fetchObservabilitySummary();
    } catch (error) {
      setMemoriesError(getErrorMessage(error, '加载系统记忆失败。'));
    } finally {
      setMemoriesLoading(false);
    }
  }, [fetchObservabilitySummary, memories.length, suffix]);

  const rollbackToStep = useCallback(async (stepId: string) => {
    if (!stepId) {
      return;
    }

    setRollbackStepId(stepId);
    setRollbackError('');
    setRollbackSuccess('');

    try {
      const resp = await fetch(`/api/rollback${suffix}`, {
        body: JSON.stringify({ step_id: stepId }),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
      });
      if (!resp.ok) {
        throw new Error(await resp.text() || `回滚失败: ${resp.status}`);
      }
      setRollbackSuccess(`已提交回滚请求: ${stepId}`);
      await fetchTrajectories(true);
    } catch (error) {
      setRollbackError(getErrorMessage(error, '轨迹回滚失败。'));
    } finally {
      setRollbackStepId('');
    }
  }, [fetchTrajectories, suffix]);

  const resumeTrajectorySession = useCallback(async (
    trajectoryId: string,
    syncLocation = true,
    showSuccess = true,
  ) => {
    if (!trajectoryId) {
      return;
    }

    setResumeLoadingId(trajectoryId);
    setResumeError('');
    if (showSuccess) {
      setResumeSuccess('');
    }

    try {
      const resp = await fetch(`/api/sessions/resume${suffix}`, {
        body: JSON.stringify({ trajectory_id: trajectoryId }),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
      });
      if (!resp.ok) {
        throw new Error(await resp.text() || `会话恢复失败: ${resp.status}`);
      }

      const data = await resp.json() as ResumeSessionResponse;
      chatBridge?.hydrateResumeSession({
        messages: normalizeChatHistory(data.messages),
        trajectoryId,
        websocketURL: typeof data.websocket_url === 'string' ? data.websocket_url : '',
      });
      setShowTrajectoryModal(false);

      if (syncLocation) {
        const nextURL = typeof data.redirect_url === 'string' && data.redirect_url
          ? data.redirect_url
          : (() => {
              const params = new URLSearchParams();
              if (token) {
                params.set('token', token);
              }
              params.set('resume_trajectory', trajectoryId);
              return `/?${params.toString()}`;
            })();
        window.history.replaceState({}, '', nextURL);
      }

      if (showSuccess) {
        setResumeSuccess(`已恢复会话: ${trajectoryId}`);
      }
    } catch (error) {
      setResumeError(getErrorMessage(error, '恢复会话失败。'));
    } finally {
      setResumeLoadingId('');
    }
  }, [chatBridge, suffix, token]);

  const fetchVisualSelfTestSample = useCallback(async (force = false) => {
    if (!force && visualSelfTestSample) {
      return;
    }

    setVisualSelfTestLoading(true);
    setVisualSelfTestError('');

    try {
      const resp = await fetch(`/api/visual-self-test/sample${suffix}`);
      if (!resp.ok) {
        throw new Error(`视觉自测任务请求失败: ${resp.status}`);
      }
      setVisualSelfTestSample(await resp.json() as VisualSelfTestSample);
    } catch (error) {
      setVisualSelfTestError(getErrorMessage(error, '加载视觉自测任务失败。'));
    } finally {
      setVisualSelfTestLoading(false);
    }
  }, [suffix, visualSelfTestSample]);

  const handleOpenTrajectoryModal = useCallback(async () => {
    setShowTrajectoryModal(true);
    await fetchTrajectories();
  }, [fetchTrajectories]);

  const handleOpenMemoryModal = useCallback(async () => {
    setShowMemoryModal(true);
    await fetchMemories();
  }, [fetchMemories]);

  const handleOpenVisualSelfTestModal = useCallback(async () => {
    setShowVisualSelfTestModal(true);
    await fetchVisualSelfTestSample();
  }, [fetchVisualSelfTestSample]);

  const setVisualSelfTestTask = useCallback((task: string) => {
    chatBridge?.insertPrompt(task);
    setShowVisualSelfTestModal(false);
  }, [chatBridge]);

  useEffect(() => {
    setObservabilityBridge({
      handleEvent: setLatestObservabilityEvent,
    });
    return () => {
      setObservabilityBridge(null);
    };
  }, [setObservabilityBridge]);

  useEffect(() => {
    fetchObservabilitySummary();
    fetchTaskSummary();
  }, [fetchObservabilitySummary, fetchTaskSummary]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      fetchTaskSummary();
    }, 3000);
    return () => {
      window.clearInterval(timer);
    };
  }, [fetchTaskSummary]);

  useEffect(() => {
    if (!initialResumeTrajectoryId || !chatBridge) {
      return;
    }
    void resumeTrajectorySession(initialResumeTrajectoryId, false, false);
  }, [chatBridge, initialResumeTrajectoryId, resumeTrajectorySession]);

  useEffect(() => {
    if (!latestObservabilityEvent || latestObservabilityEvent.status === 'running') {
      return;
    }

    void fetchObservabilitySummary();
    void fetchTaskSummary();

    if ((latestObservabilityEvent.plane === 'trajectory' || latestObservabilityEvent.plane === 'workspace') && showTrajectoryModal) {
      void fetchTrajectories(true);
    }
    if (latestObservabilityEvent.plane === 'memory' && showMemoryModal) {
      void fetchMemories(true);
    }
  }, [
    fetchMemories,
    fetchObservabilitySummary,
    fetchTaskSummary,
    fetchTrajectories,
    latestObservabilityEvent,
    showMemoryModal,
    showTrajectoryModal,
  ]);

  return {
    fetchMemories,
    fetchTrajectories,
    fetchTrajectoryDetail,
    fetchVisualSelfTestSample,
    handleOpenMemoryModal,
    handleOpenTrajectoryModal,
    handleOpenVisualSelfTestModal,
    latestObservabilityEvent,
    memories,
    memoriesError,
    memoriesLoading,
    observabilityError,
    observabilitySummary,
    resumeError,
    resumeLoadingId,
    resumeSuccess,
    resumeTrajectorySession,
    rollbackError,
    rollbackStepId,
    rollbackSuccess,
    rollbackToStep,
    selectedTrajectoryDetail,
    selectedTrajectoryId,
    setShowMcpPanel,
    setShowMemoryModal,
    setShowTrajectoryModal,
    setShowVisualSelfTestModal,
    setVisualSelfTestTask,
    showMcpPanel,
    showMemoryModal,
    showTrajectoryModal,
    showVisualSelfTestModal,
    taskSummary,
    taskSummaryError,
    trajectoryDetailError,
    trajectoryDetailLoading,
    trajectorySteps,
    trajectories,
    trajectoriesError,
    trajectoriesLoading,
    visualSelfTestError,
    visualSelfTestLoading,
    visualSelfTestSample,
  };
}
