import { useEffect, useState } from 'react';
import { ApprovalModal } from './components/ApprovalModal';
import { CodeViewer } from './components/CodeViewer';
import { ErrorBoundary } from './components/ErrorBoundary';
import { FileTree } from './components/FileTree';
import { McpPanel } from './components/McpPanel';
import { MemoryModal } from './components/MemoryModal';
import { TerminalPanel } from './components/TerminalPanel';
import { ToastCenter } from './components/ToastCenter';
import { TrajectoryModal } from './components/TrajectoryModal';
import { VisualSelfTestModal } from './components/VisualSelfTestModal';
import { AppDomainProvider, useAppDomain } from './domains/AppDomainContext';
import { ChatWorkspace, useChatDomain } from './domains/ChatDomain';
import { useObservabilityDomain } from './domains/ObservabilityDomain';
import { SettingsModal, useSettingsDomain } from './domains/SettingsDomain';
import './index.css';

function truncatePath(path: string | null) {
  if (!path) {
    return '';
  }
  if (path.length <= 56) {
    return path;
  }
  return `...${path.slice(-53)}`;
}

function LayoutShell() {
  const app = useAppDomain();
  const chat = useChatDomain();
  const observability = useObservabilityDomain();
  const settings = useSettingsDomain();
  const { t } = app;
  const [showDataPlane, setShowDataPlane] = useState(false);
  const currentFileLabel = truncatePath(app.currentFile);
  const dataPlaneAnchored = Boolean(app.currentFile?.trim());
  const statusLabel = chat.status && chat.connected ? t('app.status.online') : t('app.status.reconnecting');

  useEffect(() => {
    if (!showDataPlane) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowDataPlane(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [showDataPlane]);

  return (
    <div className="container commander-shell">
      <header className="command-hud" data-testid="dashboard-header">
        <div className="command-hud__brand">
          <span className="logo-mark">AG</span>
          <div className="command-hud__brand-copy">
            <h1>Antigravity-Go <span className="highlight">{t('app.title.suffix')}</span></h1>
            <p className="logo-subtitle">{t('app.header.subtitle')}</p>
          </div>
          <div className="command-hud__status-cluster" aria-label={t('app.control_plane.title')}>
            <span className={`command-hud__status-pill${chat.status && chat.connected ? ' is-online' : ' is-offline'}`}>
              <span aria-hidden="true" className="command-hud__status-dot" />
              <span>{statusLabel}</span>
            </span>
            <span
              className={`command-hud__status-pill${dataPlaneAnchored ? ' is-anchored' : ''}`}
              title={app.currentFile ?? undefined}
            >
              <span
                aria-hidden="true"
                className={`data-plane-indicator${dataPlaneAnchored ? ' is-anchored' : ''}`}
              />
              <span>{dataPlaneAnchored ? 'Anchored' : 'Standby'}</span>
            </span>
            {chat.indexStatus && !chat.indexStatus.includes('complete') && (
              <span className="command-hud__status-pill is-processing">{t('app.status.indexing')}</span>
            )}
          </div>
        </div>

        <div className="command-hud__actions">
          <button
            className="btn-primary command-button command-button--primary"
            data-testid="toggle-data-plane"
            onClick={() => setShowDataPlane(true)}
            type="button"
          >
            {t('app.action.open_data_plane')}
          </button>
          <button
            className="btn-secondary command-button"
            onClick={() => {
              setShowDataPlane(true);
              app.setShowTerminal(!app.showTerminal);
            }}
            type="button"
          >
            {app.showTerminal ? t('app.action.hide_terminal_drawer') : t('app.action.show_terminal_drawer')}
          </button>
          <button className="badge badge-btn" data-testid="open-trajectory" onClick={() => void observability.handleOpenTrajectoryModal()} type="button">
            {t('app.action.trajectory')} {observability.observabilitySummary ? `(${observability.observabilitySummary.trajectories.count})` : ''}
          </button>
          <button className="badge badge-btn" data-testid="open-memory" onClick={() => void observability.handleOpenMemoryModal()} type="button">
            {t('app.action.memory')}
          </button>
          <button className="badge badge-btn" onClick={() => observability.setShowMcpPanel(true)} type="button">MCP</button>
          <button className="badge badge-btn" data-testid="open-visual-self-test" onClick={() => void observability.handleOpenVisualSelfTestModal()} type="button">{t('app.action.visual_self_test')}</button>
          <button className="badge badge-btn" onClick={() => settings.setShowSettings(true)} type="button">{t('app.action.settings')}</button>
        </div>
      </header>

      <main className="commander-layout">
        <section className="control-plane">
          <div className="control-plane__intro">
            <div className="control-plane__heading">
              <span className="control-plane__kicker">{t('app.commander.kicker')}</span>
              <h2>{t('app.commander.title')}</h2>
              <p>{t('app.commander.subtitle')}</p>
            </div>
            <div className="control-plane__actions">
              <button className="btn-primary command-button" onClick={() => setShowDataPlane(true)} type="button">
                {t('app.action.open_data_plane')}
              </button>
              <button className="btn-secondary command-button" onClick={() => void observability.handleOpenTrajectoryModal()} type="button">
                {t('app.action.trajectory')}
              </button>
            </div>
          </div>

          <div className="control-plane__workspace">
            <ChatWorkspace
              chat={chat}
              memoryCount={observability.observabilitySummary?.memories.count ?? null}
              onOpenVisualSelfTest={() => void observability.handleOpenVisualSelfTestModal()}
              scoreboardError={observability.taskSummaryError}
              scoreboardSummary={observability.taskSummary}
              visualSelfTestSample={observability.visualSelfTestSample}
            />
          </div>
        </section>
      </main>

      <div
        aria-hidden={!showDataPlane}
        className={`data-plane-overlay${showDataPlane ? ' is-open' : ''}`}
        onClick={() => setShowDataPlane(false)}
      >
        <aside
          aria-label={t('app.data_plane.title')}
          aria-modal="true"
          className={`data-plane-drawer${showDataPlane ? ' is-open' : ''}`}
          onClick={(event) => event.stopPropagation()}
          role="dialog"
        >
          <div className="data-plane-drawer__header">
            <div className="data-plane-drawer__heading">
              <span className="data-plane-drawer__eyebrow">{t('app.data_plane.title')}</span>
              <h3>{t('app.data_plane.subtitle')}</h3>
              <p title={app.currentFile ?? undefined}>
                {app.currentFile ? t('app.data_plane.file', currentFileLabel) : t('app.data_plane.empty')}
              </p>
            </div>
            <div className="data-plane-drawer__actions">
              <button
                className="btn-secondary command-button"
                onClick={() => app.setShowTerminal(!app.showTerminal)}
                type="button"
              >
                {app.showTerminal ? t('app.action.hide_terminal_drawer') : t('app.action.show_terminal_drawer')}
              </button>
              <button className="btn-secondary command-button" onClick={() => setShowDataPlane(false)} type="button">
                {t('app.action.close_data_plane')}
              </button>
            </div>
          </div>

          <div className="data-plane-drawer__body">
            <aside className="data-plane-drawer__explorer glass-panel">
              <FileTree onSelectFile={app.setCurrentFile} />
            </aside>

            <section className="data-plane-drawer__workspace">
              <div className="data-plane-drawer__editor glass-panel">
                <CodeViewer
                  currentFile={app.currentFile}
                  lastModified={app.fileRefreshTrigger}
                  onCodeAction={chat.handleCodeAction}
                />
              </div>

              {app.showTerminal && (
                <div className="data-plane-drawer__terminal glass-panel">
                  <div className="panel-header terminal-header">
                    <span>{t('app.panel.terminal')}</span>
                    <button onClick={() => app.setShowTerminal(false)} type="button">{t('app.action.close')}</button>
                  </div>
                  <TerminalPanel />
                </div>
              )}
            </section>
          </div>
        </aside>
      </div>

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

      <ToastCenter />
    </div>
  );
}

export default function App() {
  const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
  const token = searchParams?.get('token')?.trim() || '';
  const initialResumeTrajectoryId = searchParams?.get('resume_trajectory')?.trim() || '';

  return (
    <AppDomainProvider initialResumeTrajectoryId={initialResumeTrajectoryId} token={token}>
      <ErrorBoundary>
        <LayoutShell />
      </ErrorBoundary>
    </AppDomainProvider>
  );
}
