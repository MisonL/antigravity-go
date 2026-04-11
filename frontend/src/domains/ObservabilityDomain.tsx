import { useCallback, useEffect, useMemo, useState } from 'react';
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
  deriveCapabilityPolicy,
  normalizeCoreCapabilitiesResponse,
  type CapabilityPolicy,
  type CoreCapabilitiesResponse,
} from './coreCapabilities';
import {
  normalizeCodeFrequencyResponse,
  getErrorMessage,
  normalizeChatHistory,
  normalizeExecutionDetail,
  normalizeExecutionSummary,
  type CodeFrequencyResponse,
  type ExecutionDetailResponse,
  type ExecutionSummaryResponse,
  type ObservabilityEvent,
  type ObservabilitySummary,
  type ResumeSessionResponse,
  type VisualSelfTestSample,
} from './types';
import { useAsyncResource } from './useAsyncResource';

export interface ObservabilityDomainState {
  codeFrequency: CodeFrequencyResponse | null;
  codeFrequencyError: string;
  codeFrequencyLoading: boolean;
  capabilityPolicy: CapabilityPolicy;
  coreCapabilities: CoreCapabilitiesResponse | null;
  coreCapabilitiesError: string;
  coreCapabilitiesLoading: boolean;
  executionDetail: ExecutionDetailResponse | null;
  executionDetailError: string;
  executionDetailLoading: boolean;
  executionSummaryLoading: boolean;
  handleOpenMemoryModal: () => Promise<void>;
  handleOpenExecutionModal: () => Promise<void>;
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
  showExecutionModal: boolean;
  showMemoryModal: boolean;
  showTrajectoryModal: boolean;
  showVisualSelfTestModal: boolean;
  taskSummary: ExecutionSummaryResponse | null;
  taskSummaryError: string;
  executionSummary: ExecutionSummaryResponse | null;
  selectedExecutionId: string;
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
  fetchExecutionDetail: (id: string, force?: boolean) => Promise<void>;
  fetchCoreCapabilities: (force?: boolean) => Promise<void>;
  fetchCodeFrequency: (force?: boolean) => Promise<void>;
  setShowMemoryModal: (show: boolean) => void;
  setShowExecutionModal: (show: boolean) => void;
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
  } = useAppDomain();

  const [showTrajectoryModal, setShowTrajectoryModal] = useState(false);
  const [showMemoryModal, setShowMemoryModal] = useState(false);
  const [showMcpPanel, setShowMcpPanel] = useState(false);
  const [showExecutionModal, setShowExecutionModal] = useState(false);
  const [showVisualSelfTestModal, setShowVisualSelfTestModal] = useState(false);
  const [selectedExecutionId, setSelectedExecutionId] = useState('');
  const [selectedTrajectoryId, setSelectedTrajectoryId] = useState('');
  const [rollbackStepId, setRollbackStepId] = useState('');
  const [rollbackError, setRollbackError] = useState('');
  const [rollbackSuccess, setRollbackSuccess] = useState('');
  const [resumeLoadingId, setResumeLoadingId] = useState('');
  const [resumeError, setResumeError] = useState('');
  const [resumeSuccess, setResumeSuccess] = useState('');
  const [latestObservabilityEvent, setLatestObservabilityEvent] = useState<ObservabilityEvent | null>(null);
  const {
    data: coreCapabilities,
    error: coreCapabilitiesError,
    loading: coreCapabilitiesLoading,
    run: loadCoreCapabilities,
  } = useAsyncResource<CoreCapabilitiesResponse | null>({ initialValue: null });
  const {
    data: codeFrequency,
    error: codeFrequencyError,
    loading: codeFrequencyLoading,
    run: loadCodeFrequency,
  } = useAsyncResource<CodeFrequencyResponse | null>({ initialValue: null });
  const {
    data: executionDetail,
    error: executionDetailError,
    loading: executionDetailLoading,
    run: loadExecutionDetail,
    setData: setExecutionDetail,
  } = useAsyncResource<ExecutionDetailResponse | null>({ initialValue: null });
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
    data: executionSummary,
    error: executionSummaryError,
    loading: executionSummaryLoading,
    run: loadExecutionSummary,
  } = useAsyncResource<ExecutionSummaryResponse | null>({ initialValue: null });
  const {
    data: visualSelfTestSample,
    error: visualSelfTestError,
    loading: visualSelfTestLoading,
    run: loadVisualSelfTestSample,
  } = useAsyncResource<VisualSelfTestSample | null>({ initialValue: null });

  const trajectorySteps = useMemo(
    () => normalizeTrajectorySteps(selectedTrajectoryDetail),
    [selectedTrajectoryDetail],
  );
  const capabilityPolicy = useMemo(
    () => deriveCapabilityPolicy(coreCapabilities),
    [coreCapabilities],
  );

  const fetchCoreCapabilities = useCallback(async (force = false) => {
    if (!force && coreCapabilities) {
      return;
    }

    await loadCoreCapabilities(async () => {
      const resp = await fetch('/api/core/capabilities');
      if (!resp.ok) {
        throw new Error(t('capabilities.error.request', resp.status));
      }
      return normalizeCoreCapabilitiesResponse(await resp.json());
    }, {
      onError: (error) => getErrorMessage(error, t('capabilities.error.load')),
    });
  }, [coreCapabilities, loadCoreCapabilities, t]);

  const fetchObservabilitySummary = useCallback(async () => {
    await loadObservabilitySummary(async () => {
      const resp = await fetch('/api/observability/summary');
      if (!resp.ok) {
        throw new Error(t('observability.error.summary_request', resp.status));
      }
      return await resp.json() as ObservabilitySummary;
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.summary_load')),
    });
  }, [loadObservabilitySummary, t]);

  const fetchCodeFrequency = useCallback(async (force = false) => {
    if (!force && codeFrequency) {
      return;
    }

    await loadCodeFrequency(async () => {
      const resp = await fetch('/api/observability/code-frequency');
      if (!resp.ok) {
        throw new Error(t('code_frequency.error.request', resp.status));
      }
      return normalizeCodeFrequencyResponse(await resp.json());
    }, {
      onError: (error) => getErrorMessage(error, t('code_frequency.error.load')),
    });
  }, [codeFrequency, loadCodeFrequency, t]);

  const fetchExecutionSummary = useCallback(async () => {
    await loadExecutionSummary(async () => {
      const endpoints = ['/api/executions/summary', '/api/tasks'];
      let lastStatus = 0;
      let lastBody = '';

      for (const endpoint of endpoints) {
        const resp = await fetch(endpoint);
        if (resp.ok) {
          return normalizeExecutionSummary(await resp.json());
        }

        lastStatus = resp.status;
        lastBody = await resp.text();
        if (resp.status !== 404 && resp.status !== 405) {
          break;
        }
      }

      throw new Error(lastBody.trim() || t('observability.error.tasks_request', lastStatus || 404));
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.tasks_load')),
    });
  }, [loadExecutionSummary, t]);

  const fetchExecutionDetail = useCallback(async (id: string, force = false) => {
    if (!id) {
      return;
    }
    if (!force && id === selectedExecutionId && executionDetail) {
      return;
    }

    setSelectedExecutionId(id);
    await loadExecutionDetail(async () => {
      const resp = await fetch(`/api/executions/${encodeURIComponent(id)}`);
      if (!resp.ok) {
        throw new Error(t('observability.error.execution_detail_request', resp.status));
      }
      return normalizeExecutionDetail(await resp.json());
    }, {
      onError: (error) => {
        setExecutionDetail(null);
        return getErrorMessage(error, t('observability.error.execution_detail_load'));
      },
    });
  }, [executionDetail, loadExecutionDetail, selectedExecutionId, setExecutionDetail, t]);

  const fetchTrajectoryDetail = useCallback(async (id: string, force = false) => {
    if (!id) {
      return;
    }
    if (!capabilityPolicy.trajectory.showDetail) {
      setSelectedTrajectoryId(id);
      setSelectedTrajectoryDetail(null);
      return;
    }
    if (!force && id === selectedTrajectoryId && selectedTrajectoryDetail) {
      return;
    }

    setSelectedTrajectoryId(id);
    await loadTrajectoryDetail(async () => {
      const resp = await fetch(`/api/trajectories/${encodeURIComponent(id)}`);
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
  }, [
    capabilityPolicy.trajectory.showDetail,
    loadTrajectoryDetail,
    selectedTrajectoryDetail,
    selectedTrajectoryId,
    setSelectedTrajectoryDetail,
    t,
  ]);

  const fetchTrajectories = useCallback(async (force = false) => {
    if (!force && trajectories.length > 0) {
      return;
    }

    const normalized = await loadTrajectories(async () => {
      const resp = await fetch('/api/trajectories');
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

    if (!capabilityPolicy.trajectory.showDetail) {
      setSelectedTrajectoryId(normalized[0].id);
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
    t,
    trajectories.length,
    capabilityPolicy.trajectory.showDetail,
  ]);

  const fetchMemories = useCallback(async (force = false) => {
    if (!force && memories.length > 0) {
      return;
    }

    const nextMemories = await loadMemories(async () => {
      const resp = await fetch('/api/memories');
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
  }, [fetchObservabilitySummary, loadMemories, memories.length, t]);

  const rollbackToStep = useCallback(async (stepId: string) => {
    if (!stepId) {
      return;
    }
    if (!capabilityPolicy.trajectory.allowRollback) {
      const message = t('trajectory.error.rollback_unsupported');
      setRollbackError(message);
      showNotification(message, 'error');
      return;
    }

    setRollbackStepId(stepId);
    setRollbackError('');
    setRollbackSuccess('');

    try {
      const resp = await fetch('/api/rollback', {
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
  }, [capabilityPolicy.trajectory.allowRollback, fetchTrajectories, showNotification, t]);

  const resumeTrajectorySession = useCallback(async (
    trajectoryId: string,
    syncLocation = true,
    showSuccess = true,
  ) => {
    if (!trajectoryId) {
      return;
    }
    if (!capabilityPolicy.trajectory.allowResume) {
      const message = t('trajectory.error.resume_unsupported');
      setResumeError(message);
      showNotification(message, 'error');
      return;
    }

    setResumeLoadingId(trajectoryId);
    setResumeError('');
    if (showSuccess) {
      setResumeSuccess('');
    }

    try {
      const resp = await fetch('/api/sessions/resume', {
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
  }, [capabilityPolicy.trajectory.allowResume, chatBridge, showNotification, t]);

  const fetchVisualSelfTestSample = useCallback(async (force = false) => {
    if (!force && visualSelfTestSample) {
      return;
    }

    await loadVisualSelfTestSample(async () => {
      const resp = await fetch('/api/visual-self-test/sample');
      if (!resp.ok) {
        throw new Error(t('observability.error.visual_request', resp.status));
      }
      return await resp.json() as VisualSelfTestSample;
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.visual_load')),
    });
  }, [loadVisualSelfTestSample, t, visualSelfTestSample]);

  const handleOpenTrajectoryModal = useCallback(async () => {
    if (!capabilityPolicy.trajectory.showList) {
      return;
    }
    setShowTrajectoryModal(true);
    await fetchTrajectories();
  }, [capabilityPolicy.trajectory.showList, fetchTrajectories]);

  const handleOpenExecutionModal = useCallback(async () => {
    setShowExecutionModal(true);
    const summary = await loadExecutionSummary(async () => {
      const endpoints = ['/api/executions/summary', '/api/tasks'];
      let lastStatus = 0;
      let lastBody = '';

      for (const endpoint of endpoints) {
        const resp = await fetch(endpoint);
        if (resp.ok) {
          return normalizeExecutionSummary(await resp.json());
        }

        lastStatus = resp.status;
        lastBody = await resp.text();
        if (resp.status !== 404 && resp.status !== 405) {
          break;
        }
      }

      throw new Error(lastBody.trim() || t('observability.error.tasks_request', lastStatus || 404));
    }, {
      onError: (error) => getErrorMessage(error, t('observability.error.tasks_load')),
    });
    const nextId = summary?.current_execution?.id || summary?.executions[0]?.id || selectedExecutionId;
    if (nextId) {
      await fetchExecutionDetail(nextId, true);
    }
  }, [fetchExecutionDetail, loadExecutionSummary, selectedExecutionId, t]);

  const handleOpenMemoryModal = useCallback(async () => {
    if (!capabilityPolicy.memory.showQuery) {
      return;
    }
    setShowMemoryModal(true);
    await fetchMemories();
  }, [capabilityPolicy.memory.showQuery, fetchMemories]);

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
    fetchCoreCapabilities();
    fetchObservabilitySummary();
    fetchExecutionSummary();
  }, [fetchCoreCapabilities, fetchExecutionSummary, fetchObservabilitySummary]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      fetchExecutionSummary();
    }, 3000);
    return () => {
      window.clearInterval(timer);
    };
  }, [fetchExecutionSummary]);

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
    void fetchExecutionSummary();

    if (showExecutionModal && selectedExecutionId) {
      void fetchExecutionDetail(selectedExecutionId, true);
    }

    if ((latestObservabilityEvent.plane === 'trajectory' || latestObservabilityEvent.plane === 'workspace') && showTrajectoryModal) {
      void fetchTrajectories(true);
    }
    if (latestObservabilityEvent.plane === 'memory' && showMemoryModal) {
      void fetchMemories(true);
    }
  }, [
    fetchExecutionDetail,
    fetchMemories,
    fetchObservabilitySummary,
    fetchExecutionSummary,
    fetchTrajectories,
    latestObservabilityEvent,
    selectedExecutionId,
    showExecutionModal,
    showMemoryModal,
    showTrajectoryModal,
  ]);

  return {
    codeFrequency,
    codeFrequencyError,
    codeFrequencyLoading,
    capabilityPolicy,
    coreCapabilities,
    coreCapabilitiesError,
    coreCapabilitiesLoading,
    executionDetail,
    executionDetailError,
    executionDetailLoading,
    executionSummaryLoading,
    fetchMemories,
    fetchExecutionDetail,
    fetchCoreCapabilities,
    fetchCodeFrequency,
    fetchTrajectories,
    fetchTrajectoryDetail,
    fetchVisualSelfTestSample,
    handleOpenExecutionModal,
    handleOpenMemoryModal,
    handleOpenTrajectoryModal,
    handleOpenVisualSelfTestModal,
    executionSummary,
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
    selectedExecutionId,
    selectedTrajectoryDetail,
    selectedTrajectoryId,
    setShowExecutionModal,
    setShowMcpPanel,
    setShowMemoryModal,
    setShowTrajectoryModal,
    setShowVisualSelfTestModal,
    showExecutionModal,
    setVisualSelfTestTask,
    showMcpPanel,
    showMemoryModal,
    showTrajectoryModal,
    showVisualSelfTestModal,
    taskSummary: executionSummary,
    taskSummaryError: executionSummaryError,
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
