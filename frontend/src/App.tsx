import { ApprovalModal } from './components/ApprovalModal';
import { CodeViewer } from './components/CodeViewer';
import { FileTree } from './components/FileTree';
import { McpPanel } from './components/McpPanel';
import { MemoryModal } from './components/MemoryModal';
import { ScoreboardPanel } from './components/ScoreboardPanel';
import { TerminalPanel } from './components/TerminalPanel';
import { TrajectoryModal } from './components/TrajectoryModal';
import { VisualSelfTestModal } from './components/VisualSelfTestModal';
import { AppDomainProvider, useAppDomain } from './domains/AppDomainContext';
import { ChatWorkspace, useChatDomain } from './domains/ChatDomain';
import { useObservabilityDomain } from './domains/ObservabilityDomain';
import { SettingsModal, useSettingsDomain } from './domains/SettingsDomain';
import './index.css';

function LayoutShell() {
  const app = useAppDomain();
  const chat = useChatDomain();
  const observability = useObservabilityDomain();
  const settings = useSettingsDomain();

  return (
    <div className="container">
      <header className="glass-header" data-testid="dashboard-header">
        <div className="logo">
          <span className="logo-mark">AG</span>
          <div>
            <h1>Antigravity <span className="highlight">控制台</span></h1>
            <p className="logo-subtitle">Phase 6B Domain Shell</p>
          </div>
        </div>

        <div className="status-bar">
          <ScoreboardPanel error={observability.taskSummaryError} summary={observability.taskSummary} />
          <button className="badge badge-btn" data-testid="open-trajectory" onClick={() => void observability.handleOpenTrajectoryModal()} type="button">
            轨迹树 {observability.observabilitySummary ? `(${observability.observabilitySummary.trajectories.count})` : ''}
          </button>
          <button className="badge badge-btn" data-testid="open-memory" onClick={() => void observability.handleOpenMemoryModal()} type="button">
            系统记忆 {observability.observabilitySummary ? `(${observability.observabilitySummary.memories.count})` : ''}
          </button>
          <button className="badge badge-btn" onClick={() => observability.setShowMcpPanel(true)} type="button">MCP</button>
          <button className="badge badge-btn" data-testid="open-visual-self-test" onClick={() => void observability.handleOpenVisualSelfTestModal()} type="button">视觉自测</button>
          <button className="badge badge-btn" onClick={() => settings.setShowSettings(true)} type="button">设置</button>

          {observability.latestObservabilityEvent && (
            <span className="badge info" title={observability.latestObservabilityEvent.timestamp}>
              {observability.latestObservabilityEvent.message}
            </span>
          )}
          {observability.observabilityError && <span className="badge error">{observability.observabilityError}</span>}
          {chat.indexStatus && !chat.indexStatus.includes('complete') && <span className="badge processing">索引中</span>}
          {chat.indexStatus && chat.indexStatus.includes('complete') && <span className="badge success-dim" title={chat.indexStatus}>已索引</span>}
          {chat.status && chat.connected ? (
            <>
              <span className="badge success">系统在线</span>
              <span className="badge info">端口 {chat.status.core_port}</span>
              <span className="badge warning">Tokens {chat.status.token_usage}</span>
            </>
          ) : (
            <span className="badge error">重连中</span>
          )}
        </div>
      </header>

      <main className="ide-layout">
        <aside className="sidebar glass-panel">
          <FileTree onSelectFile={app.setCurrentFile} />
        </aside>

        <section className="editor-container">
          <div className="editor-area glass-panel" style={{ flex: app.showTerminal ? '1 1 60%' : '1 1 100%' }}>
            <CodeViewer
              currentFile={app.currentFile}
              lastModified={app.fileRefreshTrigger}
              onCodeAction={chat.handleCodeAction}
            />
          </div>

          {app.showTerminal ? (
            <div className="terminal-area glass-panel">
              <div className="panel-header terminal-header">
                <span>终端</span>
                <button onClick={() => app.setShowTerminal(false)} type="button">关闭</button>
              </div>
              <TerminalPanel />
            </div>
          ) : (
            <button className="terminal-toggle" onClick={() => app.setShowTerminal(true)} type="button">
              打开终端
            </button>
          )}
        </section>

        <aside className="right-panel">
          <ChatWorkspace
            chat={chat}
            onOpenVisualSelfTest={() => void observability.handleOpenVisualSelfTestModal()}
            visualSelfTestSample={observability.visualSelfTestSample}
          />
        </aside>
      </main>

      {chat.approvalReq && <ApprovalModal onDecision={chat.handleApprovalDecision} request={chat.approvalReq} />}

      <SettingsModal settings={settings} />

      {observability.showTrajectoryModal && (
        <TrajectoryModal
          detailError={observability.trajectoryDetailError}
          detailLoading={observability.trajectoryDetailLoading}
          isLoading={observability.trajectoriesLoading}
          items={observability.trajectories}
          listError={observability.trajectoriesError}
          onClose={() => observability.setShowTrajectoryModal(false)}
          onRefresh={() => void observability.fetchTrajectories(true)}
          onResume={(id) => void observability.resumeTrajectorySession(id)}
          onRollback={(stepId) => void observability.rollbackToStep(stepId)}
          onSelect={(id) => void observability.fetchTrajectoryDetail(id, true)}
          resumeError={observability.resumeError}
          resumeLoadingId={observability.resumeLoadingId}
          resumeSuccess={observability.resumeSuccess}
          rollbackError={observability.rollbackError}
          rollbackStepId={observability.rollbackStepId}
          rollbackSuccess={observability.rollbackSuccess}
          selectedDetail={observability.selectedTrajectoryDetail}
          selectedId={observability.selectedTrajectoryId}
          steps={observability.trajectorySteps}
        />
      )}

      {observability.showMemoryModal && (
        <MemoryModal
          isLoading={observability.memoriesLoading}
          items={observability.memories}
          listError={observability.memoriesError}
          onClose={() => observability.setShowMemoryModal(false)}
          onRefresh={() => void observability.fetchMemories(true)}
        />
      )}

      {observability.showMcpPanel && (
        <McpPanel onClose={() => observability.setShowMcpPanel(false)} token={app.token} />
      )}

      {observability.showVisualSelfTestModal && (
        <VisualSelfTestModal
          error={observability.visualSelfTestError}
          isLoading={observability.visualSelfTestLoading}
          onClose={() => observability.setShowVisualSelfTestModal(false)}
          onInsertTask={observability.setVisualSelfTestTask}
          onRefresh={() => void observability.fetchVisualSelfTestSample(true)}
          sample={observability.visualSelfTestSample}
        />
      )}
    </div>
  );
}

export default function App() {
  const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
  const token = searchParams?.get('token')?.trim() || '';
  const initialResumeTrajectoryId = searchParams?.get('resume_trajectory')?.trim() || '';

  return (
    <AppDomainProvider initialResumeTrajectoryId={initialResumeTrajectoryId} token={token}>
      <LayoutShell />
    </AppDomainProvider>
  );
}
