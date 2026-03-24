import type { ReactNode } from 'react';

interface StateMessageProps {
  kind?: 'error' | 'info' | 'success';
  message: string;
}

interface LoadingStateProps {
  message: string;
  skeleton?: ReactNode;
}

interface AsyncContentProps {
  children?: ReactNode;
  emptyMessage?: string;
  error?: string;
  hasContent: boolean;
  loading: boolean;
  loadingMessage: string;
  skeleton?: ReactNode;
}

export function StateMessage({ kind = 'info', message }: StateMessageProps) {
  if (!message) {
    return null;
  }

  const className = kind === 'info'
    ? 'data-state'
    : `data-state data-state-${kind}`;

  return <div className={className}>{message}</div>;
}

export function LoadingState({ message, skeleton }: LoadingStateProps) {
  return (
    <div className="loading-shell">
      <StateMessage message={message} />
      {skeleton}
    </div>
  );
}

export function AsyncContent({
  children,
  emptyMessage = '',
  error = '',
  hasContent,
  loading,
  loadingMessage,
  skeleton,
}: AsyncContentProps) {
  const showLoading = loading && !hasContent;

  return (
    <>
      {showLoading && <LoadingState message={loadingMessage} skeleton={skeleton} />}
      {!showLoading && error && <StateMessage kind="error" message={error} />}
      {!showLoading && !error && !hasContent && emptyMessage && <StateMessage message={emptyMessage} />}
      {hasContent ? children : null}
    </>
  );
}
