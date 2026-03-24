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
  buildTokenQuery,
  getErrorMessage,
  normalizeChatHistory,
  type ObservabilityEvent,
  type ObservabilitySummary,
  type ResumeSessionResponse,
  type VisualSelfTestSample,
} from './types';
import { useAsyncResource } from './useAsyncResource';

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
    showNotification,
    t,
    setObservabilityBridge,
    token,
  } = useAppDomain();

  const [showTrajectoryModal, setShowTrajectoryModal] = useState(false);
  const [showMemoryModal, setShowMemoryModal] = useState(false);
  const [showMcpPanel, setShowMcpPanel] = useState(false);
  const [showVisualSelfTestModal, setShowVisualSelfTestModal] = useState(false);
  const [selectedTrajectoryId, setSelectedTrajectoryId] = useState('');
  const [rollbackStepId, setRollbackStepId] = useState('');
  const [rollbackError, setRollbackError] = useState('');
  const [rollbackSuccess, setRollbackSuccess] = useState('');
  const [resumeLoadingId, setResumeLoadingId] = useState('');
  const [resumeError, setResumeError] = useState('');
  const [resumeSuccess, setResumeSuccess] = useState('');
  const [latestObservabilityEvent, setLatestObservabilityEvent] = useState<ObservabilityEvent | null>(null);
  const {
    data: trajectories,
    error: trajectoriesError,
    loading: trajectoriesLoading,
    run: loadTrajectories,
  } = useAsyncResource<TrajectorySummary[]>({ initialValue: [] });
  const {
    data: selectedTrajectoryDetail,
    error: trajectoryDetailError,
    loading: trajectoryDetailLoading,
    run: loadTrajectoryDetail,
    setData: setSelectedTrajectoryDetail,
  } = useAsyncResource<JsonRecord | null>({ initialValue: null });
  const {
    data: memories,
    error: memoriesError,
    loading: memoriesLoading,
    run: loadMemories,
  } = useAsyncResource<MemorySummary[]>({ initialValue: [] });
  const {
    data: observabilitySummary,
    error: observabilityError,
    run: loadObservabilitySummary,
  } = useAsyncResource<ObservabilitySummary | null>({ initialValue: null });
  const {
    data: taskSummary,
    error: taskSummaryError,
    run: loadTaskSummary,
  } = useAsyncResource<TaskSummaryResponse | null>({ initialValue: null });
  const {
    data: visualSelfTestSample,
    error: visualSelfTestError,
    loading: visualSelfTestLoading,
    run: loadVisualSelfTestSample,
  } = useAsyncResource<VisualSelfTestSample | null>({ initialValue: null });

  const suffix = useMemo(() => buildTokenQuery(token), [token]);
  const trajectorySteps = useMemo(
    () => normalizeTrajectorySteps(selectedTrajectoryDetail),
    [selectedTrajectoryDetail],
  );

  const fetchObservabilitySummary = useCallback(async () => {
    await loadObservabilitySummary(async () => {
      const resp = await fetch(`/api/observability/summary${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.summary_request', resp.status));
      }
      return await resp.json() as ObservabilitySummary;
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.summary_load')),
    });
  }, [loadObservabilitySummary, suffix, t]);

  const fetchTaskSummary = useCallback(async () => {
    await loadTaskSummary(async () => {
      const resp = await fetch(`/api/tasks${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.tasks_request', resp.status));
      }
      return await resp.json() as TaskSummaryResponse;
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.tasks_load')),
    });
  }, [loadTaskSummary, suffix, t]);

  const fetchTrajectoryDetail = useCallback(async (id: string, force = false) => {
    if (!id) {
      return;
    }
    if (!force && id === selectedTrajectoryId && selectedTrajectoryDetail) {
      return;
    }

    setSelectedTrajectoryId(id);
    await loadTrajectoryDetail(async () => {
      const resp = await fetch(`/api/trajectories/${encodeURIComponent(id)}${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.trajectory_detail_request', resp.status));
      }
      const data = await resp.json();
      if (!data || typeof data !== 'object') {
        throw new Error(t('observability.error.trajectory_detail_invalid'));
      }
      return data as JsonRecord;
    }, {
      onError: (error) => {
        setSelectedTrajectoryDetail(null);
        return getErrorMessage(error, t('observability.error.trajectory_detail_load'));
      },
      onSuccess: () => {
        setRollbackError('');
        setRollbackSuccess('');
      },
    });
  }, [loadTrajectoryDetail, selectedTrajectoryDetail, selectedTrajectoryId, setSelectedTrajectoryDetail, suffix, t]);

  const fetchTrajectories = useCallback(async (force = false) => {
    if (!force && trajectories.length > 0) {
      return;
    }

    const normalized = await loadTrajectories(async () => {
      const resp = await fetch(`/api/trajectories${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.trajectory_list_request', resp.status));
      }
      return normalizeTrajectories(await resp.json());
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.trajectory_list_load')),
    });
    if (!normalized) {
      return;
    }

    await fetchObservabilitySummary();

    if (normalized.length === 0) {
      setSelectedTrajectoryId('');
      setSelectedTrajectoryDetail(null);
      return;
    }

    const nextId = normalized.some((item) => item.id === selectedTrajectoryId)
      ? selectedTrajectoryId
      : normalized[0].id;
    await fetchTrajectoryDetail(nextId, force);
  }, [
    fetchObservabilitySummary,
    fetchTrajectoryDetail,
    loadTrajectories,
    selectedTrajectoryId,
    setSelectedTrajectoryDetail,
    suffix,
    t,
    trajectories.length,
  ]);

  const fetchMemories = useCallback(async (force = false) => {
    if (!force && memories.length > 0) {
      return;
    }

    const nextMemories = await loadMemories(async () => {
      const resp = await fetch(`/api/memories${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.memory_list_request', resp.status));
      }
      return normalizeMemories(await resp.json());
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.memory_list_load')),
    });
    if (!nextMemories) {
      return;
    }
    await fetchObservabilitySummary();
  }, [fetchObservabilitySummary, loadMemories, memories.length, suffix, t]);

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
        throw new Error(await resp.text() || t('observability.error.rollback_request', resp.status));
      }
      const message = t('observability.rollback.success', stepId);
      setRollbackSuccess(message);
      showNotification(message, 'success');
      await fetchTrajectories(true);
    } catch (error) {
      const message = getErrorMessage(error, t('observability.error.rollback_load'));
      setRollbackError(message);
      showNotification(message, 'error');
    } finally {
      setRollbackStepId('');
    }
  }, [fetchTrajectories, showNotification, suffix, t]);

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
        throw new Error(await resp.text() || t('observability.error.resume_request', resp.status));
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
        const message = t('observability.resume.success', trajectoryId);
        setResumeSuccess(message);
        showNotification(message, 'success');
      }
    } catch (error) {
      const message = getErrorMessage(error, t('observability.error.resume_load'));
      setResumeError(message);
      showNotification(message, 'error');
    } finally {
      setResumeLoadingId('');
    }
  }, [chatBridge, showNotification, suffix, t, token]);

  const fetchVisualSelfTestSample = useCallback(async (force = false) => {
    if (!force && visualSelfTestSample) {
      return;
    }

    await loadVisualSelfTestSample(async () => {
      const resp = await fetch(`/api/visual-self-test/sample${suffix}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.visual_request', resp.status));
      }
      return await resp.json() as VisualSelfTestSample;
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.visual_load')),
    });
  }, [loadVisualSelfTestSample, suffix, t, visualSelfTestSample]);

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
