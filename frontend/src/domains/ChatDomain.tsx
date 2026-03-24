import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import type { ApprovalDecisionInput, ApprovalRequest } from '../components/ApprovalModal';
import { ScoreboardPanel, type TaskSummaryResponse } from '../components/ScoreboardPanel';
import { useAppDomain } from './AppDomainContext';
import {
  buildWebSocketURL,
  formatCodeActionPrompt,
  normalizeApprovalRequest,
  parseSpecialistToolArgs,
  type ChatMessage,
  type ServerStatus,
  type ToolData,
  type VisualSelfTestSample,
} from './types';

export interface ChatDomainState {
  approvalReq: ApprovalRequest | null;
  chatEndRef: RefObject<HTMLDivElement | null>;
  chatMessages: ChatMessage[];
  connected: boolean;
  handleApprovalDecision: (decision: ApprovalDecisionInput) => void;
  handleCodeAction: (code: string) => void;
  handleKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  handleSendMessage: () => void;
  indexStatus: string;
  input: string;
  isThinking: boolean;
  logs: string[];
  logsEndRef: RefObject<HTMLDivElement | null>;
  sendFeedback: (index: number, type: 'positive' | 'negative') => void;
  setInput: (value: string) => void;
  status: ServerStatus | null;
  streamingResponse: string;
}

interface ChatWorkspaceProps {
  chat: ChatDomainState;
  memoryCount: number | null;
  onOpenVisualSelfTest: () => void;
  scoreboardError: string;
  scoreboardSummary: TaskSummaryResponse | null;
  visualSelfTestSample: VisualSelfTestSample | null;
}

function smoothScrollToRef(ref: RefObject<HTMLDivElement | null>) {
  if (typeof window === 'undefined') {
    return;
  }
  window.requestAnimationFrame(() => {
    ref.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  });
}

function replaceLatestRunningTool(
  messages: ChatMessage[],
  name: string,
  toolData: Partial<ToolData>,
): ChatMessage[] {
  const next = [...messages];
  for (let index = next.length - 1; index >= 0; index -= 1) {
    const message = next[index];
    if (message.role !== 'tool' || message.toolData?.status !== 'running' || message.toolData?.name !== name) {
      continue;
    }
    next[index] = {
      ...message,
      toolData: {
        ...message.toolData,
        ...toolData,
      },
    };
    break;
  }
  return next;
}

function toolDisplay(t: (key: string, ...args: unknown[]) => string, toolData?: ToolData) {
  if (!toolData) {
    return { args: '', badge: '', name: 'tool' };
  }
  if (toolData.name !== 'ask_specialist') {
    return {
      args: toolData.args,
      badge: toolData.status === 'running' ? t('chat.badge.running') : toolData.status === 'error' ? t('chat.badge.failed') : t('chat.badge.completed'),
      name: toolData.name,
    };
  }

  const args = parseSpecialistToolArgs(toolData.args);
  return {
    args: typeof args.task === 'string' ? args.task : toolData.args,
    badge: typeof args.role === 'string' ? args.role : t('chat.badge.specialist'),
    name: typeof args.role === 'string' ? t('chat.tool.specialist_role', args.role) : t('chat.tool.specialist'),
  };
}

export function useChatDomain(): ChatDomainState {
  const {
    currentFile,
    observabilityBridge,
    resumeTrajectoryId,
    resumeWebSocketURL,
    setChatBridge,
    setResumeTrajectoryId,
    setResumeWebSocketURL,
    locale,
    showNotification,
    t,
    token,
    touchFileRefresh,
  } = useAppDomain();

  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [connected, setConnected] = useState(false);
  const [indexStatus, setIndexStatus] = useState('');
  const [logs, setLogs] = useState<string[]>([]);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [input, setInputState] = useState('');
  const [isThinking, setIsThinking] = useState(false);
  const [streamingResponse, setStreamingResponse] = useState('');
  const [approvalReq, setApprovalReq] = useState<ApprovalRequest | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const currentFileRef = useRef(currentFile);
  const observabilityBridgeRef = useRef(observabilityBridge);

  useEffect(() => {
    currentFileRef.current = currentFile;
  }, [currentFile]);

  useEffect(() => {
    observabilityBridgeRef.current = observabilityBridge;
  }, [observabilityBridge]);

  const setInput = useCallback((value: string) => {
    setInputState(value);
  }, []);

  const hydrateResumeSession = useCallback((payload: {
    messages: ChatMessage[];
    trajectoryId: string;
    websocketURL: string;
  }) => {
    setChatMessages(payload.messages);
    setStreamingResponse('');
    setIsThinking(false);
    setApprovalReq(null);
    setInputState('');
    setResumeTrajectoryId(payload.trajectoryId);
    setResumeWebSocketURL(payload.websocketURL);
  }, [setResumeTrajectoryId, setResumeWebSocketURL]);

  useEffect(() => {
    setChatBridge({
      hydrateResumeSession,
      insertPrompt: setInput,
    });
    return () => {
      setChatBridge(null);
    };
  }, [hydrateResumeSession, setChatBridge, setInput]);

  useEffect(() => {
    smoothScrollToRef(logsEndRef);
  }, [logs]);

  useEffect(() => {
    smoothScrollToRef(chatEndRef);
  }, [chatMessages, streamingResponse]);

  useEffect(() => {
    const wsURL = buildWebSocketURL(token, resumeTrajectoryId, locale, resumeWebSocketURL);
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      ws.send(JSON.stringify({
        type: 'set_locale',
        payload: JSON.stringify({ locale }),
      }));
    };

    ws.onclose = () => {
      setConnected(false);
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as Record<string, any>;
        switch (msg.type) {
          case 'status':
            setStatus({
              ready: Boolean(msg.ready),
              core_port: Number(msg.core_port || 0),
              token_usage: Number(msg.token_usage || 0),
            });
            return;
          case 'log':
            setLogs((prev) => [...prev.slice(-100), String(msg.data || '')]);
            return;
          case 'chat_start':
            setIsThinking(true);
            setStreamingResponse('');
            return;
          case 'chat_chunk':
            setStreamingResponse((prev) => prev + String(msg.chunk || ''));
            return;
          case 'chat_done':
            setIsThinking(false);
            setStreamingResponse((prev) => {
              if (prev) {
                setChatMessages((messages) => [...messages, { role: 'assistant', content: prev }]);
              }
              return '';
            });
            return;
          case 'chat_error':
            setIsThinking(false);
            setStreamingResponse('');
            setChatMessages((messages) => [...messages, {
              role: 'assistant',
              content: t('common.error.prefix', String(msg.error || t('common.unknown_error'))),
            }]);
            return;
          case 'file_change':
            if (currentFileRef.current === msg.path) {
              touchFileRefresh();
            }
            return;
          case 'index_status':
            setIndexStatus(String(msg.status || ''));
            return;
          case 'tool_start':
            setChatMessages((messages) => [...messages, {
              role: 'tool',
              toolData: {
                args: String(msg.data?.args || ''),
                name: String(msg.data?.name || 'tool'),
                status: 'running',
              },
            }]);
            return;
          case 'tool_end':
            setChatMessages((messages) => replaceLatestRunningTool(messages, String(msg.data?.name || ''), {
              result: String(msg.data?.result || ''),
              status: 'completed',
            }));
            return;
          case 'tool_error':
            setChatMessages((messages) => replaceLatestRunningTool(messages, String(msg.data?.name || ''), {
              result: String(msg.data?.result || ''),
              status: 'error',
            }));
            return;
          case 'approval_request':
          case 'permission_request':
            setApprovalReq(normalizeApprovalRequest(msg.data));
            return;
          case 'approval_timeout':
          case 'permission_timeout': {
            const requestId = typeof msg.data?.id === 'string' ? msg.data.id : '';
            setApprovalReq((prev) => (prev && prev.id === requestId ? null : prev));
            setChatMessages((messages) => [...messages, {
              role: 'assistant',
              content: t('chat.approval_timeout'),
            }]);
            return;
          }
          case 'observability_event':
            observabilityBridgeRef.current?.handleEvent(msg.data);
            return;
          default:
            return;
        }
      } catch (error) {
        console.error('WS parse error', error);
      }
    };

    return () => {
      ws.close();
    };
  }, [locale, resumeTrajectoryId, resumeWebSocketURL, t, token, touchFileRefresh]);

  const sendFeedback = useCallback((index: number, type: 'positive' | 'negative') => {
    if (!wsRef.current) {
      return;
    }
    wsRef.current.send(JSON.stringify({
      type: 'feedback',
      payload: JSON.stringify({
        messageIndex: index,
        score: type === 'positive' ? 1 : -1,
        timestamp: new Date().toISOString(),
      }),
    }));
    showNotification(
      type === 'positive' ? t('chat.feedback.positive') : t('chat.feedback.negative'),
      type === 'positive' ? 'success' : 'info',
    );
  }, [showNotification, t]);

  const handleSendMessage = useCallback(() => {
    if (!input.trim() || !wsRef.current || isThinking || approvalReq) {
      return;
    }
    setChatMessages((messages) => [...messages, { role: 'user', content: input }]);
    wsRef.current.send(JSON.stringify({ payload: input, type: 'chat' }));
    setInputState('');
  }, [approvalReq, input, isThinking]);

  const handleApprovalDecision = useCallback((decision: ApprovalDecisionInput) => {
    if (!wsRef.current || !approvalReq) {
      return;
    }
    wsRef.current.send(JSON.stringify({
      type: 'approval_response',
      payload: JSON.stringify({
        allow: decision.allow,
        approved_chunk_ids: decision.approvedChunkIDs,
        id: approvalReq.id,
      }),
    }));
    setApprovalReq(null);
  }, [approvalReq]);

  const handleKeyDown = useCallback((event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      handleSendMessage();
    }
  }, [handleSendMessage]);

  const handleCodeAction = useCallback((code: string) => {
    setInputState(formatCodeActionPrompt(t, currentFile, code));
  }, [currentFile, t]);

  return {
    approvalReq,
    chatEndRef,
    chatMessages,
    connected,
    handleApprovalDecision,
    handleCodeAction,
    handleKeyDown,
    handleSendMessage,
    indexStatus,
    input,
    isThinking,
    logs,
    logsEndRef,
    sendFeedback,
    setInput,
    status,
    streamingResponse,
  };
}

export function ChatWorkspace({
  chat,
  memoryCount,
  onOpenVisualSelfTest,
  scoreboardError,
  scoreboardSummary,
  visualSelfTestSample,
}: ChatWorkspaceProps) {
  const { t } = useAppDomain();
  const tokenUsage = chat.status ? chat.status.token_usage.toLocaleString() : '--';
  const memoryUsage = memoryCount === null ? '--' : memoryCount.toLocaleString();

  return (
    <>
      <div className="card glass-panel chat-panel">
        <div className="panel-header chat-panel__header">
          <span>{t('chat.panel.title')}</span>
          <div className="chat-panel__telemetry" aria-label={t('chat.telemetry.title')}>
            <span className="chat-panel__metric">
              <span className="chat-panel__metric-label">{t('chat.telemetry.tokens')}</span>
              <span className="chat-panel__metric-value">{tokenUsage}</span>
            </span>
            <span className="chat-panel__metric">
              <span className="chat-panel__metric-label">{t('chat.telemetry.memory')}</span>
              <span className="chat-panel__metric-value">{memoryUsage}</span>
            </span>
          </div>
        </div>
        <div className="chat-messages" aria-live="polite">
          {chat.chatMessages.length === 0 ? (
            <div className="welcome-screen">
              <div className="welcome-kicker">Antigravity-Go</div>
              <div className="welcome-screen__scoreboard">
                <ScoreboardPanel error={scoreboardError} summary={scoreboardSummary} />
              </div>
              <h2>{t('chat.hero.title')}</h2>
              <p>{t('chat.hero.subtitle')}</p>
              <div className="suggestion-chips">
                <button type="button" onClick={() => chat.setInput(t('chat.prompt.project_map'))}>{t('chat.hero.project_map')}</button>
                <button type="button" onClick={() => chat.setInput(t('chat.prompt.risk_review'))}>{t('chat.hero.risk_review')}</button>
                <button type="button" onClick={() => chat.setInput(t('chat.prompt.architecture'))}>{t('chat.hero.architecture')}</button>
                <button type="button" onClick={() => visualSelfTestSample ? chat.setInput(visualSelfTestSample.task) : onOpenVisualSelfTest()}>{t('chat.hero.visual_test')}</button>
              </div>
            </div>
          ) : (
            chat.chatMessages.map((message, index) => {
              if (message.role === 'tool') {
                const display = toolDisplay(t, message.toolData);
                return (
                  <div key={`${message.role}-${index}`} className="chat-message tool">
                    <div className={`tool-card ${message.toolData?.status || 'completed'}`}>
                      <div className="tool-header">
                        <span className="tool-name">{display.name}</span>
                        <span className="tool-badge">{display.badge}</span>
                      </div>
                      {display.args && <div className="tool-args">{display.args}</div>}
                      {message.toolData?.status !== 'running' && message.toolData?.result && (
                        <div className="tool-result">
                          <details>
                            <summary>{t('chat.details.result')}</summary>
                            <pre>{message.toolData.result}</pre>
                          </details>
                        </div>
                      )}
                    </div>
                  </div>
                );
              }

              return (
                <div key={`${message.role}-${index}`} className={`chat-message ${message.role}`}>
                  <span className="role-icon">{message.role === 'user' ? 'USER' : 'AI'}</span>
                  <div className="message-content">
                    {message.content}
                    {message.role === 'assistant' && !chat.streamingResponse && (
                      <div className="message-actions">
                        <button type="button" onClick={() => chat.sendFeedback(index, 'positive')}>{t('chat.feedback.helpful')}</button>
                        <button type="button" onClick={() => chat.sendFeedback(index, 'negative')}>{t('chat.feedback.improve')}</button>
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          )}

          {chat.streamingResponse && (
            <div className="chat-message assistant streaming">
              <span className="role-icon">AI</span>
              <div className="message-content">{chat.streamingResponse}</div>
            </div>
          )}
          <div ref={chat.chatEndRef} />
        </div>

        <div className="chat-input-area">
          <textarea
            className="chat-input"
            disabled={chat.isThinking}
            id="chat-input"
            name="chat-input"
            onChange={(event) => chat.setInput(event.target.value)}
            onKeyDown={chat.handleKeyDown}
            placeholder={t('chat.input.placeholder')}
            value={chat.input}
            aria-label={t('chat.input.placeholder')}
            aria-busy={chat.isThinking}
          />
          <button
            className={`send-button${chat.isThinking ? ' is-busy' : ''}`}
            disabled={chat.isThinking || !chat.input.trim() || chat.approvalReq !== null}
            onClick={chat.handleSendMessage}
            type="button"
            aria-busy={chat.isThinking}
          >
            <span>{chat.isThinking ? t('chat.button.thinking') : t('chat.button.send')}</span>
          </button>
        </div>
      </div>

      <div className="card glass-panel logs-panel">
        <div className="panel-header">{t('chat.panel.logs')}</div>
        <div className="logs-container" aria-live="polite">
          {chat.logs.length === 0 && <div className="log-line log-line-empty">{t('chat.logs.empty')}</div>}
          {chat.logs.map((line, index) => (
            <div key={`${line}-${index}`} className="log-line">{line}</div>
          ))}
          <div ref={chat.logsEndRef} />
        </div>
      </div>
    </>
  );
}
