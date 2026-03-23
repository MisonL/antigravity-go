import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import type { ApprovalRequest } from '../components/ApprovalModal';
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
  handleApprovalDecision: (allow: boolean) => void;
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
  onOpenVisualSelfTest: () => void;
  visualSelfTestSample: VisualSelfTestSample | null;
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

function toolDisplay(toolData?: ToolData) {
  if (!toolData) {
    return { args: '', badge: '', name: 'tool' };
  }
  if (toolData.name !== 'ask_specialist') {
    return {
      args: toolData.args,
      badge: toolData.status === 'running' ? '运行中' : toolData.status === 'error' ? '失败' : '完成',
      name: toolData.name,
    };
  }

  const args = parseSpecialistToolArgs(toolData.args);
  return {
    args: typeof args.task === 'string' ? args.task : toolData.args,
    badge: typeof args.role === 'string' ? args.role : 'specialist',
    name: typeof args.role === 'string' ? `专家调用 ${args.role}` : '专家调用',
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
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages, streamingResponse]);

  useEffect(() => {
    const wsURL = buildWebSocketURL(token, resumeTrajectoryId, resumeWebSocketURL);
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
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
              content: `错误：${String(msg.error || '未知错误')}`,
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
              content: '授权请求已超时，系统已自动拒绝，请重新触发操作。',
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
  }, [resumeTrajectoryId, resumeWebSocketURL, token, touchFileRefresh]);

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
    window.alert(type === 'positive' ? '感谢反馈，我会继续优化。' : '收到反馈，我会改进输出质量。');
  }, []);

  const handleSendMessage = useCallback(() => {
    if (!input.trim() || !wsRef.current || isThinking || approvalReq) {
      return;
    }
    setChatMessages((messages) => [...messages, { role: 'user', content: input }]);
    wsRef.current.send(JSON.stringify({ payload: input, type: 'chat' }));
    setInputState('');
  }, [approvalReq, input, isThinking]);

  const handleApprovalDecision = useCallback((allow: boolean) => {
    if (!wsRef.current || !approvalReq) {
      return;
    }
    wsRef.current.send(JSON.stringify({
      type: 'approval_response',
      payload: JSON.stringify({ allow, id: approvalReq.id }),
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
    setInputState(formatCodeActionPrompt(currentFile, code));
  }, [currentFile]);

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
  onOpenVisualSelfTest,
  visualSelfTestSample,
}: ChatWorkspaceProps) {
  return (
    <>
      <div className="card glass-panel chat-panel">
        <div className="panel-header">对话</div>
        <div className="chat-messages">
          {chat.chatMessages.length === 0 ? (
            <div className="welcome-screen">
              <div className="welcome-kicker">Antigravity Go</div>
              <h2>控制面已就绪</h2>
              <p>从任务、文件或自测入口开始，右侧对话区会统一承接控制流。</p>
              <div className="suggestion-chips">
                <button type="button" onClick={() => chat.setInput('请先用 get_project_summary 总结项目结构，并给出关键模块说明。')}>项目地图</button>
                <button type="button" onClick={() => chat.setInput('请审查 internal/agent 包，找出潜在 bug、边界条件和并发风险。')}>风险审查</button>
                <button type="button" onClick={() => chat.setInput('请基于 cmd/agy/main.go 解释系统整体架构与关键数据流。')}>架构说明</button>
                <button type="button" onClick={() => visualSelfTestSample ? chat.setInput(visualSelfTestSample.task) : onOpenVisualSelfTest()}>视觉自测</button>
              </div>
            </div>
          ) : (
            chat.chatMessages.map((message, index) => {
              if (message.role === 'tool') {
                const display = toolDisplay(message.toolData);
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
                            <summary>查看结果</summary>
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
                        <button type="button" onClick={() => chat.sendFeedback(index, 'positive')}>有帮助</button>
                        <button type="button" onClick={() => chat.sendFeedback(index, 'negative')}>需改进</button>
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
            onChange={(event) => chat.setInput(event.target.value)}
            onKeyDown={chat.handleKeyDown}
            placeholder="输入任务或问题。Enter 发送，Shift+Enter 换行。"
            value={chat.input}
          />
          <button
            className="send-button"
            disabled={chat.isThinking || !chat.input.trim() || chat.approvalReq !== null}
            onClick={chat.handleSendMessage}
            type="button"
          >
            发送
          </button>
        </div>
      </div>

      <div className="card glass-panel logs-panel">
        <div className="panel-header">日志</div>
        <div className="logs-container">
          {chat.logs.map((line, index) => (
            <div key={`${line}-${index}`} className="log-line">{line}</div>
          ))}
          <div ref={chat.logsEndRef} />
        </div>
      </div>
    </>
  );
}
