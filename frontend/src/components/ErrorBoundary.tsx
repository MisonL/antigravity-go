import React from 'react';
import { useAppDomain } from '../domains/AppDomainContext';

interface ErrorBoundaryState {
  error: Error | null;
}

class ErrorBoundaryRoot extends React.Component<React.PropsWithChildren<{ heading: string; resetLabel: string; summary: string; title: string }>, ErrorBoundaryState> {
  public constructor(props: React.PropsWithChildren<{ heading: string; resetLabel: string; summary: string; title: string }>) {
    super(props);
    this.state = { error: null };
  }

  public static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  public componentDidCatch(error: Error) {
    console.error('Frontend boundary caught an error', error);
  }

  public render() {
    if (!this.state.error) {
      return this.props.children;
    }

    return (
      <div className="error-boundary-shell">
        <div className="glass-panel error-boundary-card" role="alert">
          <div className="error-boundary-kicker">{this.props.title}</div>
          <h1>{this.props.heading}</h1>
          <p>{this.props.summary}</p>
          <pre className="error-boundary-stack">{`${this.state.error.name}: ${this.state.error.message}`}</pre>
          <button
            className="btn-primary"
            type="button"
            onClick={() => {
              this.setState({ error: null });
              window.location.reload();
            }}
          >
            {this.props.resetLabel}
          </button>
        </div>
      </div>
    );
  }
}

export function ErrorBoundary({ children }: React.PropsWithChildren) {
  const { t } = useAppDomain();

  return (
    <ErrorBoundaryRoot
      heading={t('error_boundary.heading')}
      resetLabel={t('error_boundary.retry')}
      summary={t('error_boundary.summary')}
      title={t('error_boundary.title')}
    >
      {children}
    </ErrorBoundaryRoot>
  );
}
