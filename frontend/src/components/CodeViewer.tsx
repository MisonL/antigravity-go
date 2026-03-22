import React, { useEffect, useRef, useState } from 'react';
import Editor, { useMonaco, loader } from '@monaco-editor/react';

// Configure loader to use CDN or local
loader.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' } });

interface CodeViewerProps {
  currentFile: string | null;
  onCodeAction: (code: string) => void;
  lastModified?: number; // timestamp to trigger refresh
}

export const CodeViewer: React.FC<CodeViewerProps> = ({ currentFile, onCodeAction, lastModified }) => {
  const token = (typeof window !== 'undefined'
    ? new URLSearchParams(window.location.search).get('token')?.trim() || ''
    : '');

  const [code, setCode] = useState<string>('// 请选择一个文件进行查看与编辑');
  const [language, setLanguage] = useState('go');
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [externalChange, setExternalChange] = useState(false);
  
  const monaco = useMonaco();
  const editorRef = useRef<any>(null);
  const currentFileRef = useRef<string | null>(currentFile);
  const isDirtyRef = useRef(false);
  const suppressDirtyOnceRef = useRef(false);

  useEffect(() => {
    currentFileRef.current = currentFile;
  }, [currentFile]);

  useEffect(() => {
    isDirtyRef.current = isDirty;
  }, [isDirty]);

  // 切换文件：重置编辑状态（只在 currentFile 变化时触发）
  useEffect(() => {
    if (!currentFile) return;
    setSaving(false);
    setExternalChange(false);
    setIsDirty(false);
    isDirtyRef.current = false;
    suppressDirtyOnceRef.current = true;
    setCode('// 加载中…');
  }, [currentFile]);

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
        if (!res.ok) throw new Error(`加载失败（${res.status}）`);
        const data = await res.json();

        // 防止竞态：请求返回时文件已切换
        if (currentFileRef.current !== requestedFile) return;

        const nextContent = typeof data?.content === 'string' ? data.content : '';

        const currentVal = editorRef.current ? String(editorRef.current.getValue() ?? '') : code;
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

      } catch (e) {
        if ((e as any)?.name === 'AbortError') return;
        console.error("加载失败", e);
        setCode(`// 加载失败：${String(e)}`);
      }
    };

    fetchContent();
    return () => controller.abort();
  }, [currentFile, lastModified]); // Depend on lastModified to re-fetch

  const handleEditorDidMount = (editor: any, monacoInstance: any) => {
    editorRef.current = editor;
    
    // Add Save Action (Ctrl+S / Cmd+S)
    editor.addCommand(monacoInstance.KeyMod.CtrlCmd | monacoInstance.KeyCode.KeyS, () => {
        handleSave();
    });

    // Add Context Menu Action: "Ask AI"
    editor.addAction({
        id: 'ask-ai',
        label: '询问 AI',
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
              console.log("Saved.");
          } else {
              console.error("保存失败");
          }
      } catch (e) {
          console.error("保存失败", e);
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
    setExternalChange(false);
    fetch(`/api/fs/content?path=${encodeURIComponent(currentFile)}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
      .then(async (r) => {
        if (!r.ok) throw new Error(`加载失败（${r.status}）`);
        return r.json();
      })
      .then((d) => {
        suppressDirtyOnceRef.current = true;
        setCode(typeof d?.content === 'string' ? d.content : '');
        setIsDirty(false); // 接受磁盘版本，重置 dirty
        isDirtyRef.current = false;
      })
      .catch((e) => {
        console.error("重新加载失败", e);
      });
  };

  if (!currentFile) {
      return (
          <div className="codeviewer-empty">
              <div className="codeviewer-empty__inner">
                  <div className="codeviewer-empty__icon">⚛️</div>
                  <div>选择一个文件开始编辑</div>
              </div>
          </div>
      );
  }

  return (
    <div className="codeviewer">
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
            {isDirty && <span className="status-pill status-pill--warn">未保存</span>}
            {saving && <span className="status-pill status-pill--info">保存中…</span>}
            {externalChange && (
                <button 
                  onClick={handleReload}
                  className="status-pill status-pill--danger"
                >
                    文件已在磁盘变更（点击重新加载）
                </button>
            )}
        </div>
    </div>
  );
};
