import React, { useState, useEffect, useRef, useCallback } from 'react';
import { FileTree } from './components/FileTree';
import { CodeViewer } from './components/CodeViewer';
import { TerminalPanel } from './components/TerminalPanel';
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
  const [permissionReq, setPermissionReq] = useState<{ id: string; tool: string; args: string } | null>(null);

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
        } else if (msg.type === 'permission_request') {
            setPermissionReq({
              id: msg.data?.id || '',
              tool: msg.data?.tool || '',
              args: msg.data?.args || '',
            });
        } else if (msg.type === 'permission_timeout') {
            const id = msg.data?.id || '';
            setPermissionReq((prev) => (prev && prev.id === id ? null : prev));
            setChatMessages((msgs) => [...msgs, { role: 'assistant', content: '⚠️ 授权请求已超时，已自动拒绝。请重新触发一次操作。' }]);
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
    if (!input.trim() || !wsRef.current || isThinking || permissionReq) return;

    setChatMessages(msgs => [...msgs, { role: 'user', content: input }]);
    wsRef.current.send(JSON.stringify({ type: 'chat', payload: input }));
    setInput('');
  }, [input, isThinking, permissionReq]);

  const handlePermissionDecision = (allow: boolean) => {
    if (!wsRef.current || !permissionReq) return;
    wsRef.current.send(JSON.stringify({
      type: 'permission_response',
      payload: JSON.stringify({ id: permissionReq.id, allow }),
    }));
    setPermissionReq(null);
  };

  const handleKeyDown = (e: React.KeyboardEvent | KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  return (
    <div className="container">
      <header className="glass-header">
        <div className="logo">
          <span className="logo-icon">🚀</span>
          <h1>Antigravity <span className="highlight">控制台</span></h1>
        </div>
        <div className="status-bar">
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
	                <button className="send-button" onClick={handleSendMessage} disabled={isThinking || !input.trim() || permissionReq != null}>
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

      {permissionReq && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.35)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 9999,
          }}
        >
          <div
            className="glass-panel"
            style={{
              width: "min(720px, calc(100vw - 32px))",
              padding: "16px",
              borderRadius: "12px",
              border: "1px solid rgba(0,0,0,0.08)",
              background: "rgba(255,255,255,0.85)",
              backdropFilter: "blur(10px)",
              boxShadow: "0 24px 64px rgba(0,0,0,0.25)",
            }}
          >
            <div style={{ fontWeight: 700, marginBottom: 8 }}>需要授权执行工具</div>
            <div style={{ fontSize: 12, opacity: 0.8, marginBottom: 10 }}>
              工具：<code>{permissionReq.tool}</code>
            </div>
            <pre
              style={{
                margin: 0,
                padding: 12,
                borderRadius: 10,
                background: "rgba(0,0,0,0.06)",
                maxHeight: 240,
                overflow: "auto",
                fontSize: 12,
                whiteSpace: "pre-wrap",
                wordBreak: "break-all",
              }}
            >
              {permissionReq.args}
            </pre>
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8, marginTop: 12 }}>
              <button
                onClick={() => handlePermissionDecision(false)}
                style={{
                  padding: "8px 12px",
                  borderRadius: 10,
                  border: "1px solid rgba(0,0,0,0.12)",
                  background: "rgba(255,255,255,0.9)",
                  cursor: "pointer",
                }}
              >
                拒绝
              </button>
              <button
                onClick={() => handlePermissionDecision(true)}
                style={{
                  padding: "8px 12px",
                  borderRadius: 10,
                  border: "1px solid rgba(47,129,247,0.35)",
                  background: "rgba(47,129,247,0.14)",
                  cursor: "pointer",
                  fontWeight: 600,
                }}
              >
                允许
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
