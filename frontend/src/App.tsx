import React, { useState, useEffect, useRef, useCallback } from 'react';
import { FileTree } from './components/FileTree';
import { CodeViewer } from './components/CodeViewer';
import { TerminalPanel } from './components/TerminalPanel';
import { ApprovalModal, ApprovalRequest } from './components/ApprovalModal';
import { MemoryModal } from './components/MemoryModal';
import { TrajectoryModal } from './components/TrajectoryModal';
import { VisualSelfTestModal } from './components/VisualSelfTestModal';
import {
  isRecord,
  JsonRecord,
  MemorySummary,
  normalizeMemories,
  normalizeTrajectorySteps,
  normalizeTrajectories,
  TrajectorySummary,
} from './components/planeData';
import './index.css';

interface ServerStatus {
  ready: boolean;
  core_port: number;
  token_usage: number;
}

interface ToolData {
  name: string;
  args: string;
  result?: string;
  status: 'running' | 'completed' | 'error';
}

interface ChatMessage {
  role: 'user' | 'assistant' | 'tool';
  content?: string;
  toolData?: ToolData;
}

interface PlaneSnapshot {
  count: number;
  latest_id?: string;
  latest_updated_at?: string;
}

interface ObservabilitySummary {
  generated_at: string;
  memories: PlaneSnapshot;
  trajectories: PlaneSnapshot;
}

interface ObservabilityEvent {
  message: string;
  plane: string;
  status: string;
  timestamp: string;
  tool: string;
}

interface SelfTestChecklistItem {
  label: string;
  selector: string;
}

interface VisualSelfTestSample {
  checklist: SelfTestChecklistItem[];
  task: string;
  title: string;
  url: string;
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

function normalizeApprovalRequest(payload: unknown): ApprovalRequest {
  const data = isRecord(payload) ? payload : {};
  return {
    id: typeof data.id === 'string' ? data.id : '',
    tool: typeof data.tool === 'string' ? data.tool : '',
    category: typeof data.category === 'string' ? data.category : 'tool_execution',
    summary: typeof data.summary === 'string' ? data.summary : '工具请求执行，需要人工确认。',
    args: typeof data.args === 'string' ? data.args : '',
    preview: typeof data.preview === 'string' ? data.preview : '',
    metadata: isRecord(data.metadata) ? data.metadata : {},
  };
}

function App() {
  const token = (typeof window !== 'undefined'
    ? new URLSearchParams(window.location.search).get('token')?.trim() || ''
    : '');

  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [connected, setConnected] = useState(false);
  const [indexStatus, setIndexStatus] = useState<string>('');
  const [logs, setLogs] = useState<string[]>([]);
  const logsEndRef = useRef<HTMLDivElement>(null);

  // Layout State
  const [currentFile, setCurrentFile] = useState<string | null>(null);
  const [showTerminal, setShowTerminal] = useState(true);
  const [fileRefreshTrigger, setFileRefreshTrigger] = useState(0);

  // Chat State
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [isThinking, setIsThinking] = useState(false);
  const [streamingResponse, setStreamingResponse] = useState('');
  const chatEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [approvalReq, setApprovalReq] = useState<ApprovalRequest | null>(null);

  // Settings State
  const [showSettings, setShowSettings] = useState(false);
  const [config, setConfig] = useState<{
    provider: string;
    model: string;
    base_url: string;
    api_key: string;
  }>({
    provider: 'openai',
    model: '',
    base_url: '',
    api_key: '',
  });
  const [showTrajectoryModal, setShowTrajectoryModal] = useState(false);
  const [showMemoryModal, setShowMemoryModal] = useState(false);
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
  const [memories, setMemories] = useState<MemorySummary[]>([]);
  const [memoriesLoading, setMemoriesLoading] = useState(false);
  const [memoriesError, setMemoriesError] = useState('');
  const [observabilitySummary, setObservabilitySummary] = useState<ObservabilitySummary | null>(null);
  const [observabilityError, setObservabilityError] = useState('');
  const [latestObservabilityEvent, setLatestObservabilityEvent] = useState<ObservabilityEvent | null>(null);
  const [showVisualSelfTestModal, setShowVisualSelfTestModal] = useState(false);
  const [visualSelfTestLoading, setVisualSelfTestLoading] = useState(false);
  const [visualSelfTestError, setVisualSelfTestError] = useState('');
  const [visualSelfTestSample, setVisualSelfTestSample] = useState<VisualSelfTestSample | null>(null);
  const trajectorySteps = normalizeTrajectorySteps(selectedTrajectoryDetail);

  async function fetchObservabilitySummary() {
    setObservabilityError('');

    try {
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/observability/summary${suffix}`);
      if (!resp.ok) {
        throw new Error(`可观测性摘要请求失败: ${resp.status}`);
      }

      const data = await resp.json();
      setObservabilitySummary(data as ObservabilitySummary);
    } catch (error) {
      setObservabilityError(getErrorMessage(error, '加载可观测性摘要失败。'));
    }
  }

  async function fetchTrajectoryDetail(id: string, force = false) {
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
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/trajectories/${encodeURIComponent(id)}${suffix}`);
      if (!resp.ok) {
        throw new Error(`轨迹详情请求失败: ${resp.status}`);
      }

      const data = await resp.json();
      if (!isRecord(data)) {
        throw new Error('轨迹详情格式无效');
      }

      setSelectedTrajectoryDetail(data);
      setRollbackError('');
      setRollbackSuccess('');
    } catch (error) {
      setSelectedTrajectoryDetail(null);
      setTrajectoryDetailError(getErrorMessage(error, '加载轨迹详情失败。'));
    } finally {
      setTrajectoryDetailLoading(false);
    }
  }

  async function fetchTrajectories(force = false) {
    if (!force && trajectories.length > 0) {
      return;
    }

    setTrajectoriesLoading(true);
    setTrajectoriesError('');

    try {
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/trajectories${suffix}`);
      if (!resp.ok) {
        throw new Error(`轨迹列表请求失败: ${resp.status}`);
      }

      const data = await resp.json();
      const normalized = normalizeTrajectories(data);
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
  }

  async function fetchMemories(force = false) {
    if (!force && memories.length > 0) {
      return;
    }

    setMemoriesLoading(true);
    setMemoriesError('');

    try {
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/memories${suffix}`);
      if (!resp.ok) {
        throw new Error(`记忆列表请求失败: ${resp.status}`);
      }

      const data = await resp.json();
      setMemories(normalizeMemories(data));
      await fetchObservabilitySummary();
    } catch (error) {
      setMemoriesError(getErrorMessage(error, '加载系统记忆失败。'));
    } finally {
      setMemoriesLoading(false);
    }
  }

  async function rollbackToStep(stepId: string) {
    if (!stepId) {
      return;
    }

    setRollbackStepId(stepId);
    setRollbackError('');
    setRollbackSuccess('');

    try {
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/rollback${suffix}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ step_id: stepId }),
      });
      if (!resp.ok) {
        const message = await resp.text();
        throw new Error(message || `回滚失败: ${resp.status}`);
      }

      setRollbackSuccess(`已提交回滚请求: ${stepId}`);
      await fetchTrajectories(true);
    } catch (error) {
      setRollbackError(getErrorMessage(error, '轨迹回滚失败。'));
    } finally {
      setRollbackStepId('');
    }
  }

  async function fetchVisualSelfTestSample(force = false) {
    if (!force && visualSelfTestSample) {
      return;
    }

    setVisualSelfTestLoading(true);
    setVisualSelfTestError('');

    try {
      const suffix = token ? `?token=${encodeURIComponent(token)}` : '';
      const resp = await fetch(`/api/visual-self-test/sample${suffix}`);
      if (!resp.ok) {
        throw new Error(`视觉自测任务请求失败: ${resp.status}`);
      }

      const data = await resp.json();
      setVisualSelfTestSample(data as VisualSelfTestSample);
    } catch (error) {
      setVisualSelfTestError(getErrorMessage(error, '加载视觉自测任务失败。'));
    } finally {
      setVisualSelfTestLoading(false);
    }
  }

  async function handleOpenTrajectoryModal() {
    setShowTrajectoryModal(true);
    await fetchTrajectories();
  }

  async function handleOpenMemoryModal() {
    setShowMemoryModal(true);
    await fetchMemories();
  }

  async function handleOpenVisualSelfTestModal() {
    setShowVisualSelfTestModal(true);
    await fetchVisualSelfTestSample();
  }

  const fetchConfig = async () => {
    try {
      const resp = await fetch(`/api/config${token ? `?token=${encodeURIComponent(token)}` : ''}`);
      if (resp.ok) {
        const data = await resp.json();
        setConfig(data);
      }
    } catch (e) {
      console.error('Failed to fetch config', e);
    }
  };

  useEffect(() => {
    fetchConfig();
    fetchObservabilitySummary();
  }, []);

  useEffect(() => {
    if (!latestObservabilityEvent) {
      return;
    }
    if (latestObservabilityEvent.status === 'running') {
      return;
    }

    fetchObservabilitySummary();

    if ((latestObservabilityEvent.plane === 'trajectory' || latestObservabilityEvent.plane === 'workspace') && showTrajectoryModal) {
      fetchTrajectories(true);
    }
    if (latestObservabilityEvent.plane === 'memory' && showMemoryModal) {
      fetchMemories(true);
    }
  }, [latestObservabilityEvent, showMemoryModal, showTrajectoryModal]);

  const handleSaveConfig = async () => {
    try {
      const resp = await fetch(`/api/config${token ? `?token=${encodeURIComponent(token)}` : ''}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      if (resp.ok) {
        alert('配置已保存并应用。');
        setShowSettings(false);
      } else {
        alert('保存失败。');
      }
    } catch (e) {
      alert(`保存出错: ${e}`);
    }
  };

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages, streamingResponse]);

  // Handle file change propagation from WS to CodeViewer
  const currentFileRef = useRef(currentFile);
  useEffect(() => { currentFileRef.current = currentFile; }, [currentFile]);

  useEffect(() => {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const wsURL = `${proto}://${window.location.host}/ws${token ? `?token=${encodeURIComponent(token)}` : ''}`;
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      console.log('Connected to WS');
    };

    ws.onclose = () => {
      setConnected(false);
      console.log('Disconnected from WS');
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'status') {
          setStatus({
            ready: msg.ready,
            core_port: msg.core_port,
            token_usage: msg.token_usage
          });
        } else if (msg.type === 'log') {
          setLogs(prev => [...prev.slice(-100), msg.data]);
        } else if (msg.type === 'chat_start') {
          setIsThinking(true);
          setStreamingResponse('');
        } else if (msg.type === 'chat_chunk') {
          setStreamingResponse(prev => prev + msg.chunk);
        } else if (msg.type === 'chat_done') {
          setIsThinking(false);
          setStreamingResponse(prev => {
            if (prev) {
              setChatMessages(msgs => [...msgs, { role: 'assistant', content: prev }]);
            }
            return '';
          });
        } else if (msg.type === 'chat_error') {
          setIsThinking(false);
          setChatMessages(msgs => [...msgs, { role: 'assistant', content: `错误：${msg.error}` }]);
          setStreamingResponse('');
        } else if (msg.type === 'file_change') {
           if (currentFileRef.current === msg.path) {
               setFileRefreshTrigger(Date.now());
           }
        } else if (msg.type === 'index_status') {
           setIndexStatus(msg.status);
        } else if (msg.type === 'tool_start') {
            setChatMessages(prev => [...prev, {
                role: 'tool',
                toolData: {
                    name: msg.data.name,
                    args: msg.data.args,
                    status: 'running'
                }
            }]);
        } else if (msg.type === 'tool_end') {
            setChatMessages(prev => {
                const newMsgs = [...prev];
                // Find the last running tool
                // Traverse backwards
                for (let i = newMsgs.length - 1; i >= 0; i--) {
                    if (newMsgs[i].role === 'tool' && newMsgs[i].toolData?.status === 'running' && newMsgs[i].toolData?.name === msg.data.name) {
                        newMsgs[i] = {
                            ...newMsgs[i],
                            toolData: {
                                ...newMsgs[i].toolData!,
                                status: 'completed',
                                result: msg.data.result
                            }
                        };
                        break;
                    }
                }
                return newMsgs;
            });
        } else if (msg.type === 'tool_error') {
             setChatMessages(prev => {
                const newMsgs = [...prev];
                for (let i = newMsgs.length - 1; i >= 0; i--) {
                    if (newMsgs[i].role === 'tool' && newMsgs[i].toolData?.status === 'running' && newMsgs[i].toolData?.name === msg.data.name) {
                        newMsgs[i] = {
                            ...newMsgs[i],
                            toolData: {
                                ...newMsgs[i].toolData!,
                                status: 'error',
                                result: msg.data.result // Error message
                            }
                        };
                        break;
                    }
                }
                return newMsgs;
            });
        } else if (msg.type === 'approval_request' || msg.type === 'permission_request') {
            setApprovalReq(normalizeApprovalRequest(msg.data));
        } else if (msg.type === 'approval_timeout' || msg.type === 'permission_timeout') {
            const id = msg.data?.id || '';
            setApprovalReq((prev) => (prev && prev.id === id ? null : prev));
            setChatMessages((msgs) => [...msgs, { role: 'assistant', content: '⚠️ 授权请求已超时，已自动拒绝。请重新触发一次操作。' }]);
        } else if (msg.type === 'observability_event') {
            setLatestObservabilityEvent(msg.data as ObservabilityEvent);
        }
      } catch (e) {
        console.error('WS parse error', e);
      }
    };

    return () => {
      ws.close();
    };
  }, []); // Run once

  const sendFeedback = (index: number, type: 'positive' | 'negative') => {
    if (!wsRef.current) return;
    const feedback = {
      messageIndex: index,
      score: type === 'positive' ? 1 : -1,
      timestamp: new Date().toISOString()
    };
    wsRef.current.send(JSON.stringify({
      type: 'feedback',
      payload: JSON.stringify(feedback)
    }));
    // Simple UI feedback
    alert(type === 'positive' ? "感谢反馈，我会继续优化。" : "收到，我会改进输出质量。");
  };

  const handleSendMessage = useCallback(() => {
    if (!input.trim() || !wsRef.current || isThinking || approvalReq) return;

    setChatMessages(msgs => [...msgs, { role: 'user', content: input }]);
    wsRef.current.send(JSON.stringify({ type: 'chat', payload: input }));
    setInput('');
  }, [approvalReq, input, isThinking]);

  const handleApprovalDecision = (allow: boolean) => {
    if (!wsRef.current || !approvalReq) return;
    wsRef.current.send(JSON.stringify({
      type: 'approval_response',
      payload: JSON.stringify({ id: approvalReq.id, allow }),
    }));
    setApprovalReq(null);
  };

  const handleKeyDown = (e: React.KeyboardEvent | KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  return (
    <div className="container">
      <header className="glass-header" data-testid="dashboard-header">
        <div className="logo">
          <span className="logo-icon">🚀</span>
          <h1>Antigravity <span className="highlight">控制台</span></h1>
        </div>
        <div className="status-bar">
          <button className="badge badge-btn" data-testid="open-trajectory" type="button" onClick={handleOpenTrajectoryModal}>
            轨迹树 {observabilitySummary ? `(${observabilitySummary.trajectories.count})` : ''}
          </button>
          <button className="badge badge-btn" data-testid="open-memory" type="button" onClick={handleOpenMemoryModal}>
            系统记忆 {observabilitySummary ? `(${observabilitySummary.memories.count})` : ''}
          </button>
          <button className="badge badge-btn" data-testid="open-visual-self-test" type="button" onClick={handleOpenVisualSelfTestModal}>
            视觉自测
          </button>
          <button className="badge badge-btn" type="button" onClick={() => setShowSettings(true)}>
            设置
          </button>
          {latestObservabilityEvent && (
            <span className="badge info" title={latestObservabilityEvent.timestamp}>
              {latestObservabilityEvent.message}
            </span>
          )}
          {observabilityError && <span className="badge error">{observabilityError}</span>}
          {indexStatus && !indexStatus.includes('complete') && (
              <span className="badge processing pulse">🔍 索引中…</span>
          )}
          {indexStatus && indexStatus.includes('complete') && (
              <span className="badge success-dim" title={indexStatus}>🧠 已索引</span>
          )}
          {status && connected ? (
            <>
              <span className="badge success">系统：在线</span>
              <span className="badge info">端口：{status.core_port}</span>
              <span className="badge warning">Tokens：{status.token_usage}</span>
            </>
          ) : (
            <span className="badge error">重连中…</span>
          )}
        </div>
      </header>

      <main className="ide-layout">
        <aside className="sidebar glass-panel">
            <FileTree onSelectFile={setCurrentFile} />
        </aside>
        
        <section className="editor-container">
          <div className="editor-area glass-panel" style={{ flex: showTerminal ? '1 1 60%' : '1 1 100%' }}>
	               <CodeViewer 
	                 currentFile={currentFile} 
	                 lastModified={fileRefreshTrigger}
	                 onCodeAction={(code) => {
	                   setInput(prev => {
	                     const prompt = `请解释或重构这段代码，并说明关键改动与风险点：\n\n\`\`\`${currentFile?.split('.').pop() || ''}\n${code}\n\`\`\`\n\n`;
	                     return prompt;
	                   });
	                 }}
	               />
	          </div>
          
	          {showTerminal && (
	            <div className="terminal-area glass-panel">
	              <div className="panel-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
	                <span>终端</span>
	                <button 
	                  onClick={() => setShowTerminal(false)}
	                  style={{ background: 'none', border: 'none', color: '#666', cursor: 'pointer', fontSize: '16px' }}
	                >×</button>
	              </div>
	              <TerminalPanel />
	            </div>
	          )}
          
	          {!showTerminal && (
	            <button 
	              className="terminal-toggle"
	              onClick={() => setShowTerminal(true)}
	            >
	              ▲ 终端
	            </button>
	          )}
	        </section>

	        <aside className="right-panel">
	            <div className="card glass-panel chat-panel" style={{flex: 1}}>
	            <div className="panel-header">对话</div>
	            <div className="chat-messages">
                {chatMessages.length === 0 ? (
            <div className="welcome-screen">
	                <div className="welcome-icon">🚀</div>
	                <h2>Antigravity <span className="highlight">Go</span></h2>
	                <p>你的 AI 编码搭档。随时待命，开始干活吧。</p>
	                <div className="suggestion-chips">
	                    <button onClick={() => setInput("请先用 get_project_summary 总结项目结构，并给出关键模块说明。")}>🗺️ 项目地图</button>
	                    <button onClick={() => setInput("请审查 internal/agent 包，找出潜在 bug/边界条件/并发风险。")}>🐛 找 Bug</button>
	                    <button onClick={() => setInput("请基于 cmd/agy/main.go 解释系统整体架构与关键数据流。")}>🏗️ 架构说明</button>
                      <button onClick={() => visualSelfTestSample ? setInput(visualSelfTestSample.task) : handleOpenVisualSelfTestModal()}>视觉自测</button>
	                </div>
	            </div>
	          ) : (
            chatMessages.map((msg, i) => (
                <div key={i} className={`chat-message ${msg.role}`}>
                    {msg.role === 'tool' ? (
	                        <div className={`tool-card ${msg.toolData?.status} ${msg.toolData?.name === 'ask_specialist' ? 'specialist' : ''}`}>
	                            <div className="tool-header">
                                <span className="tool-icon">
                                    {msg.toolData?.name === 'ask_specialist' ? (
                                        (() => {
                                            try {
                                                const args = JSON.parse(msg.toolData.args);
                                                switch(args.role) {
                                                    case 'reviewer': return '🧐';
                                                    case 'architect': return '🏛️';
                                                    case 'security': return '🛡️';
                                                    default: return '👥';
                                                }
                                            } catch(e) { return '👥'; }
                                        })()
                                    ) : (
                                        msg.toolData?.status === 'running' ? '⏳' : (msg.toolData?.status === 'error' ? '❌' : '✅')
                                    )}
                                </span>
	                                <span className="tool-name">
	                                    {msg.toolData?.name === 'ask_specialist' ? 
	                                        `调用专家：${JSON.parse(msg.toolData.args).role}` : 
	                                        msg.toolData?.name}
	                                </span>
                                <span className="tool-args">
                                    {msg.toolData?.name === 'ask_specialist' ? 
                                        JSON.parse(msg.toolData.args).task : 
                                        msg.toolData?.args}
                                </span>
                            </div>
	                            {msg.toolData?.status !== 'running' && (
	                                <div className="tool-result">
	                                    <details>
	                                        <summary>{msg.toolData?.name === 'ask_specialist' ? '查看报告' : '结果'}</summary>
	                                        <pre>{msg.toolData?.result}</pre>
	                                    </details>
	                                </div>
	                            )}
	                        </div>
                    ) : (
                        <>
                        <span className="role-icon">{msg.role === 'user' ? '👤' : '🤖'}</span>
                        <div className="message-content">
                            {msg.content}
	                            {msg.role === 'assistant' && !streamingResponse && (
	                                <div className="message-actions">
	                                    <button onClick={() => sendFeedback(i, 'positive')} title="有帮助">👍</button>
	                                    <button onClick={() => sendFeedback(i, 'negative')} title="没帮助">👎</button>
	                                </div>
	                            )}
	                        </div>
	                        </>
	                    )}
                </div>
                )))}
                {streamingResponse && (
                <div className="chat-message assistant streaming">
                    <span className="role-icon">🤖</span>
                    <div className="message-content">{streamingResponse}</div>
                </div>
                )}
                <div ref={chatEndRef} />
            </div>
	            <div className="chat-input-area">
	                <textarea
	                className="chat-input"
	                placeholder="输入任务/问题（Enter 发送，Shift+Enter 换行）"
	                value={input}
	                onChange={(e) => setInput(e.target.value)}
	                onKeyDown={handleKeyDown}
	                disabled={isThinking}
	                />
	                <button className="send-button" onClick={handleSendMessage} disabled={isThinking || !input.trim() || approvalReq != null}>
	                发送
	                </button>
	            </div>
	            </div>
	            <div className="card glass-panel logs-panel" style={{height: '30%', minHeight: '150px'}}>
	                <div className="panel-header">日志</div>
	                <div className="logs-container">
	                    {logs.map((log, i) => (
	                    <div key={i} className="log-line">{log}</div>
	                    ))}
                    <div ref={logsEndRef} />
                </div>
            </div>
        </aside>
      </main>

      {approvalReq && (
        <ApprovalModal request={approvalReq} onDecision={handleApprovalDecision} />
      )}
      {showSettings && (
        <div className="modal-overlay">
          <div className="glass-panel modal-content settings-modal">
            <div className="modal-header">
              <h3>AI 渠道配置</h3>
              <button onClick={() => setShowSettings(false)}>×</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label>AI 渠道类型</label>
                <select 
                  value={config.provider} 
                  onChange={e => setConfig({...config, provider: e.target.value})}
                >
                  <option value="openai">OpenAI 兼容 (Chat v1)</option>
                  <option value="openai-legacy">OpenAI 兼容 (Legacy)</option>
                  <option value="anthropic">Anthropic 兼容</option>
                  <option value="gemini">Google Gemini</option>
                  <option value="ollama">Ollama (本地)</option>
                  <option value="lmstudio">LM Studio (本地)</option>
                </select>
              </div>
              <div className="form-group">
                <label>模型名称 (Model)</label>
                <input 
                  type="text" 
                  value={config.model} 
                  onChange={e => setConfig({...config, model: e.target.value})}
                  placeholder={
                    config.provider === 'ollama' ? 'e.g. qwen2.5-coder:7b' :
                    config.provider === 'anthropic' ? 'e.g. claude-3-5-sonnet-latest' :
                    'e.g. gpt-4o, qwen3-max'
                  }
                />
              </div>
              <div className="form-group">
                <label>接口地址 (Base URL)</label>
                <input 
                  type="text" 
                  value={config.base_url} 
                  onChange={e => setConfig({...config, base_url: e.target.value})}
                  placeholder={
                    config.provider === 'ollama' ? 'http://localhost:11434/v1' :
                    config.provider === 'lmstudio' ? 'http://localhost:1234/v1' :
                    config.provider === 'anthropic' ? 'https://api.anthropic.com' :
                    'e.g. https://api.openai.com/v1'
                  }
                />
              </div>
              <div className="form-group">
                <label>API 密钥 (API Key)</label>
                <input 
                  type="password" 
                  value={config.api_key} 
                  onChange={e => setConfig({...config, api_key: e.target.value})}
                  placeholder="填入您的 API Key (本地模型可为空)"
                />
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn-secondary" onClick={() => setShowSettings(false)}>取消</button>
              <button className="btn-primary" onClick={handleSaveConfig}>保存配置</button>
            </div>
          </div>
        </div>
      )}
      {showTrajectoryModal && (
        <TrajectoryModal
          detailError={trajectoryDetailError}
          detailLoading={trajectoryDetailLoading}
          isLoading={trajectoriesLoading}
          items={trajectories}
          listError={trajectoriesError}
          onClose={() => setShowTrajectoryModal(false)}
          onRefresh={() => fetchTrajectories(true)}
          onRollback={rollbackToStep}
          onSelect={(id) => fetchTrajectoryDetail(id, true)}
          rollbackError={rollbackError}
          rollbackStepId={rollbackStepId}
          rollbackSuccess={rollbackSuccess}
          selectedDetail={selectedTrajectoryDetail}
          selectedId={selectedTrajectoryId}
          steps={trajectorySteps}
        />
      )}
      {showMemoryModal && (
        <MemoryModal
          isLoading={memoriesLoading}
          items={memories}
          listError={memoriesError}
          onClose={() => setShowMemoryModal(false)}
          onRefresh={() => fetchMemories(true)}
        />
      )}
      {showVisualSelfTestModal && (
        <VisualSelfTestModal
          error={visualSelfTestError}
          isLoading={visualSelfTestLoading}
          onClose={() => setShowVisualSelfTestModal(false)}
          onInsertTask={(task) => {
            setInput(task);
            setShowVisualSelfTestModal(false);
          }}
          onRefresh={() => fetchVisualSelfTestSample(true)}
          sample={visualSelfTestSample}
        />
      )}
    </div>
  );
}

export default App;
