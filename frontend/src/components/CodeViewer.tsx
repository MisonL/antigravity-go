import React, { useEffect, useRef, useState } from 'react';
import Editor, { loader } from '@monaco-editor/react';
import { SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

// Configure loader to use CDN or local
loader.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' } });

interface CodeViewerProps {
  currentFile: string | null;
  onCodeAction: (code: string) => void;
  lastModified?: number; // timestamp to trigger refresh
}

export const CodeViewer: React.FC<CodeViewerProps> = ({ currentFile, onCodeAction, lastModified }) => {
  const { showNotification, t } = useAppDomain();
  const token = (typeof window !== 'undefined'
    ? new URLSearchParams(window.location.search).get('token')?.trim() || ''
    : '');

  const [code, setCode] = useState<string>(t('codeviewer.placeholder.none'));
  const [language, setLanguage] = useState('go');
  const [isDirty, setIsDirty] = useState(false);
  const [loadingContent, setLoadingContent] = useState(false);
  const [saving, setSaving] = useState(false);
  const [externalChange, setExternalChange] = useState(false);
  const [loadError, setLoadError] = useState('');

  const editorRef = useRef<any>(null);
  const codeRef = useRef(code);
  const currentFileRef = useRef<string | null>(currentFile);
  const isDirtyRef = useRef(false);
  const suppressDirtyOnceRef = useRef(false);

  useEffect(() => {
    currentFileRef.current = currentFile;
  }, [currentFile]);

  useEffect(() => {
    isDirtyRef.current = isDirty;
  }, [isDirty]);

  useEffect(() => {
    codeRef.current = code;
  }, [code]);

  // 切换文件：重置编辑状态（只在 currentFile 变化时触发）
  useEffect(() => {
    if (!currentFile) return;
    setSaving(false);
    setLoadError('');
    setLoadingContent(true);
    setExternalChange(false);
    setIsDirty(false);
    isDirtyRef.current = false;
    suppressDirtyOnceRef.current = true;
    setCode(t('codeviewer.placeholder.loading'));
  }, [currentFile, t]);

  // Determine language from extension
  useEffect(() => {
    if (!currentFile) return;
    const ext = currentFile.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'go': setLanguage('go'); break;
      case 'ts': setLanguage('typescript'); break;
      case 'tsx': setLanguage('typescript'); break;
      case 'js': setLanguage('javascript'); break;
      case 'jsx': setLanguage('javascript'); break;
      case 'html': setLanguage('html'); break;
      case 'css': setLanguage('css'); break;
      case 'json': setLanguage('json'); break;
      case 'md': setLanguage('markdown'); break;
      default: setLanguage('plaintext');
    }
  }, [currentFile]);

  // Fetch content
  useEffect(() => {
    if (!currentFile) return;

    const controller = new AbortController();
    const requestedFile = currentFile;

    const fetchContent = async () => {
      try {
        const res = await fetch(`/api/fs/content?path=${encodeURIComponent(requestedFile)}`, {
          signal: controller.signal,
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        });
        if (!res.ok) throw new Error(`Load failed (${res.status})`);
        const data = await res.json();

        // 防止竞态：请求返回时文件已切换
        if (currentFileRef.current !== requestedFile) return;

        const nextContent = typeof data?.content === 'string' ? data.content : '';

        const currentVal = editorRef.current ? String(editorRef.current.getValue() ?? '') : codeRef.current;
        if (currentVal === nextContent) {
          setExternalChange(false);
          return;
        }

        // 若当前有未保存修改，则不覆盖，提示“磁盘已变更”
        if (isDirtyRef.current) {
          setExternalChange(true);
          return;
        }

        suppressDirtyOnceRef.current = true;
        setCode(nextContent);
        setIsDirty(false);
        isDirtyRef.current = false;
        setExternalChange(false);
        setLoadError('');

      } catch (e) {
        if ((e as any)?.name === 'AbortError') return;
        const message = t('codeviewer.error.load', String(e));
        setCode(message);
        setLoadError(message);
      } finally {
        setLoadingContent(false);
      }
    };

    fetchContent();
    return () => controller.abort();
  }, [currentFile, lastModified, t, token]); // Depend on lastModified to re-fetch

  const handleEditorDidMount = (editor: any, monacoInstance: any) => {
    editorRef.current = editor;
    
    // Add Save Action (Ctrl+S / Cmd+S)
    editor.addCommand(monacoInstance.KeyMod.CtrlCmd | monacoInstance.KeyCode.KeyS, () => {
        handleSave();
    });

    // Add Context Menu Action: "Ask AI"
    editor.addAction({
        id: 'ask-ai',
        label: t('codeviewer.context.ask_ai'),
        contextMenuGroupId: 'navigation',
        contextMenuOrder: 1.5,
        run: (ed: any) => {
            const selection = ed.getSelection();
            const text = ed.getModel().getValueInRange(selection);
            if (text) {
                // Pass to parent to handle chat interaction
                onCodeAction(text);
            }
        }
    });

    // Register Hover Provider for LSP (One-time or when monaco changes)
    monacoInstance.languages.registerHoverProvider('go', {
        provideHover: async (model: any, position: any) => {
            const file = currentFileRef.current;
            if (!file) return null;
             const result = await fetch('/api/lsp/hover', {
                 method: 'POST',
                headers: {
                  'Content-Type': 'application/json',
                  ...(token ? { Authorization: `Bearer ${token}` } : {}),
                },
                body: JSON.stringify({
                    file,
                    line: position.lineNumber - 1,
                    character: position.column - 1
                })
            });
            if (!result.ok) return null;
            const data = await result.json();
            if (!data) return null;
            return {
                contents: [
                    { value: data.markdown || "" }
                ]
            };
        }
    });
  };

  const handleSave = async () => {
      if (!currentFile || !editorRef.current) return;
      const content = editorRef.current.getValue();
      
      setSaving(true);
      try {
          const resp = await fetch('/api/fs/content', {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                ...(token ? { Authorization: `Bearer ${token}` } : {}),
              },
              body: JSON.stringify({
                  path: currentFile,
                  content: content
              })
          });
          if (resp.ok) {
              setIsDirty(false);
              isDirtyRef.current = false;
              setExternalChange(false);
              showNotification(t('codeviewer.save_success'), 'success');
          } else {
              throw new Error(t('codeviewer.save_error'));
          }
      } catch {
          showNotification(t('codeviewer.save_error'), 'error');
      } finally {
          setSaving(false);
      }
  };

  const handleChange = (value: string | undefined) => {
      if (value !== undefined) {
        setCode(value);
        if (suppressDirtyOnceRef.current) {
          suppressDirtyOnceRef.current = false;
          return;
        }
        setIsDirty(true);
        isDirtyRef.current = true;
      }
  };
  
  const handleReload = () => {
    if (!currentFile) return;
    setLoadingContent(true);
    setExternalChange(false);
    fetch(`/api/fs/content?path=${encodeURIComponent(currentFile)}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
      .then(async (r) => {
        if (!r.ok) throw new Error(`Load failed (${r.status})`);
        return r.json();
      })
      .then((d) => {
        suppressDirtyOnceRef.current = true;
        setCode(typeof d?.content === 'string' ? d.content : '');
        setLoadError('');
        setIsDirty(false); // 接受磁盘版本，重置 dirty
        isDirtyRef.current = false;
      })
      .catch((error) => {
        const message = t('codeviewer.error.load', String(error));
        setCode(message);
        setLoadError(message);
      })
      .finally(() => {
        setLoadingContent(false);
      });
  };

  if (!currentFile) {
      return (
          <div className="codeviewer-empty">
              <div className="codeviewer-empty__inner">
                  <div className="codeviewer-empty__icon">AG</div>
                  <div>{t('codeviewer.empty')}</div>
              </div>
          </div>
      );
  }

  return (
    <div className="codeviewer">
        {loadingContent && (
          <div className="codeviewer-loading">
            <div className="data-state">{t('common.loading')}</div>
            <SkeletonRows lines={8} />
          </div>
        )}
        <Editor
            height="100%"
            language={language}
            value={code}
            theme="vs"
            path={currentFile} // Key for model caching
            onChange={handleChange}
            onMount={handleEditorDidMount}
            options={{
                readOnly: false,
                minimap: { enabled: false },
                fontSize: 14,
                fontFamily: "'JetBrains Mono', monospace",
                scrollBeyondLastLine: false,
                automaticLayout: true,
            }}
        />
        {/* Status Overlay */}
        <div className="codeviewer-status">
            {loadError && <span className="status-pill status-pill--danger">{loadError}</span>}
            {isDirty && <span className="status-pill status-pill--warn">{t('codeviewer.status.unsaved')}</span>}
            {saving && <span className="status-pill status-pill--info">{t('codeviewer.status.saving')}</span>}
            {externalChange && (
                <button 
                  onClick={handleReload}
                  className="status-pill status-pill--danger"
                >
                    {t('codeviewer.status.external_change')}
                </button>
            )}
        </div>
    </div>
  );
};
