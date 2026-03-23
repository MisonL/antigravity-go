import React, { memo, useCallback, useEffect, useState } from 'react';
import { 
  Folder, FolderOpen, File, FileCode, FileJson, 
  FileText, ChevronRight, ChevronDown 
} from 'lucide-react';
import { SkeletonRows } from './Skeleton';
import { useAppDomain } from '../domains/AppDomainContext';

interface FileNode {
  name: string;
  path: string;
  type: 'file' | 'dir';
  children?: FileNode[];
}

interface FileTreeProps {
  onSelectFile: (path: string) => void;
}

const FileIcon = memo(function FileIcon({ name }: { name: string }) {
  const style: React.CSSProperties = { color: "rgba(16,24,40,0.55)" };
  if (name.endsWith('.go')) style.color = "#0ea5e9";
  else if (name.endsWith('.ts') || name.endsWith('.tsx')) style.color = "#2563eb";
  else if (name.endsWith('.js') || name.endsWith('.jsx')) style.color = "#d97706";
  else if (name.endsWith('.json')) style.color = "#f97316";
  else if (name.endsWith('.md')) style.color = "rgba(16,24,40,0.55)";

  if (name.endsWith('.json')) return <FileJson size={16} style={style} />;
  if (name.endsWith('.md')) return <FileText size={16} style={style} />;
  if (name.endsWith('.go') || name.endsWith('.ts') || name.endsWith('.tsx') || name.endsWith('.js') || name.endsWith('.jsx')) {
    return <FileCode size={16} style={style} />;
  }
  return <File size={16} style={style} />;
});

const TreeNode = memo(function TreeNode({
  node,
  level,
  onSelect,
  onLoadChildren,
  isLoadingPath,
}: {
  node: FileNode;
  level: number;
  onSelect: (path: string) => void;
  onLoadChildren: (path: string) => void;
  isLoadingPath: (path: string) => boolean;
}) {
  const { t } = useAppDomain();
  const [isOpen, setIsOpen] = useState(false);

  const handleClick = () => {
    if (node.type === 'file') {
      onSelect(node.path);
      return;
    }
    const next = !isOpen;
    setIsOpen(next);
    if (next && node.children == null) {
      onLoadChildren(node.path);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      handleClick();
    }
  };

  return (
    <div>
      <div 
        className="filetree-node"
        style={{ paddingLeft: 10 + level * 16 }}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        role="treeitem"
        aria-expanded={node.type === 'dir' ? isOpen : undefined}
        tabIndex={0}
      >
        {node.type === 'dir' && (
          <span className="filetree-chevron">
            {isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </span>
        )}
        {node.type === 'dir' ? (
          isOpen ? <FolderOpen size={16} style={{ color: "#2563eb" }} /> : <Folder size={16} style={{ color: "#2563eb" }} />
        ) : (
          <FileIcon name={node.name} />
        )}
        <span className="filetree-name">{node.name}</span>
        {node.type === 'dir' && isOpen && isLoadingPath(node.path) && (
          <span className="filetree-loading">{t('filetree.loading')}</span>
        )}
      </div>
      {isOpen && node.children && (
        <div className="filetree-children" role="group"> 
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              level={level + 1}
              onSelect={onSelect}
              onLoadChildren={onLoadChildren}
              isLoadingPath={isLoadingPath}
            />
          ))}
        </div>
      )}
    </div>
  );
});

export const FileTree: React.FC<FileTreeProps> = ({ onSelectFile }) => {
  const { t } = useAppDomain();
  const token = (typeof window !== 'undefined'
    ? new URLSearchParams(window.location.search).get('token')?.trim() || ''
    : '');

  const [root, setRoot] = useState<FileNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loadingPaths, setLoadingPaths] = useState<Record<string, boolean>>({});

  const apiFetchTree = useCallback(async (path: string) => {
    const res = await fetch(`/api/fs/tree?path=${encodeURIComponent(path)}&depth=1`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    });
    if (!res.ok) {
      throw new Error(await res.text());
    }
    return (await res.json()) as FileNode;
  }, [token]);

  const setChildrenByPath = useCallback((node: FileNode, targetPath: string, children: FileNode[]): FileNode => {
    if (node.path === targetPath) {
      return { ...node, children };
    }
    if (!node.children || node.children.length === 0) {
      return node;
    }
    return {
      ...node,
      children: node.children.map((c) => setChildrenByPath(c, targetPath, children)),
    };
  }, []);

  const handleLoadChildren = useCallback(async (path: string) => {
    if (!root) return;
    if (loadingPaths[path]) return;

    setLoadingPaths((p) => ({ ...p, [path]: true }));
    try {
      const dirNode = await apiFetchTree(path);
      const children = dirNode.children ?? [];
      setRoot((prev) => (prev ? setChildrenByPath(prev, path, children) : prev));
    } catch (e) {
      setError(e instanceof Error && e.message.trim() ? e.message : t('filetree.error'));
    } finally {
      setLoadingPaths((p) => {
        const next = { ...p };
        delete next[path];
        return next;
      });
    }
  }, [apiFetchTree, loadingPaths, root, setChildrenByPath, t]);

  useEffect(() => {
    apiFetchTree(".")
      .then((data) => {
        setRoot(data);
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error && err.message.trim() ? err.message : t('filetree.error'));
        setLoading(false);
      });
  }, [apiFetchTree, t]);

  const isLoadingPath = useCallback((path: string) => loadingPaths[path] === true, [loadingPaths]);

  if (loading) {
    return (
      <div className="filetree">
        <h3 className="filetree-title">{t('filetree.title')}</h3>
        <div className="loading-shell">
          <div className="filetree-state">{t('filetree.workspace_loading')}</div>
          <SkeletonRows lines={6} />
        </div>
      </div>
    );
  }
  if (error) return <div className="filetree-state filetree-state--error">{error}</div>;
  if (!root) return null;

  return (
    <div className="filetree" role="tree" aria-label={t('filetree.title')}>
      <h3 className="filetree-title">{t('filetree.title')}</h3>
      <TreeNode
        node={root}
        level={0}
        onSelect={onSelectFile}
        onLoadChildren={handleLoadChildren}
        isLoadingPath={isLoadingPath}
      />
    </div>
  );
};
