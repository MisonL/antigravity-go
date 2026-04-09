import React, { memo, useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';
import { useAppDomain } from '../domains/AppDomainContext';

interface TerminalPanelProps {
  className?: string;
}

function enforceSquareTerminal(host: HTMLDivElement) {
  host.style.borderRadius = '0';
  const selectors = ['.xterm', '.xterm-viewport', '.xterm-screen'];
  selectors.forEach((selector) => {
    host.querySelectorAll<HTMLElement>(selector).forEach((element) => {
      element.style.borderRadius = '0';
    });
  });
  host.querySelectorAll<HTMLCanvasElement>('canvas').forEach((canvas) => {
    canvas.style.borderRadius = '0';
  });
}

export const TerminalPanel: React.FC<TerminalPanelProps> = memo(function TerminalPanel({ className }) {
  const { t } = useAppDomain();

  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!termRef.current) {
      return;
    }

    let cancelled = false;
    let term: Terminal | null = null;
    let ws: WebSocket | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let openDelay = 0;
    let detachWindowResize: (() => void) | null = null;

    const mountTerminal = async () => {
      try {
        const statusResp = await fetch('/api/status');
        if (!statusResp.ok) {
          throw new Error(`status ${statusResp.status}`);
        }
      } catch {
        if (!cancelled && termRef.current) {
          termRef.current.textContent = t('terminal.unavailable');
          termRef.current.classList.add('terminal-shell-error');
        }
        return;
      }

      if (cancelled || !termRef.current) {
        return;
      }

      term = new Terminal({
      allowTransparency: true,
      convertEol: true,
      cursorBlink: true,
      fontFamily: "'JetBrains Mono', monospace",
      fontSize: 14,
      lineHeight: 1.3,
      scrollback: 2000,
      theme: {
        background: '#00000000',
        foreground: '#101828',
        cursor: '#2563eb',
        black: '#101828',
        red: '#dc2626',
        green: '#16a34a',
        yellow: '#d97706',
        blue: '#2563eb',
        magenta: '#7c3aed',
        cyan: '#0ea5e9',
        white: '#e5e7eb',
        brightBlack: '#475467',
        brightRed: '#b91c1c',
        brightGreen: '#15803d',
        brightYellow: '#b45309',
        brightBlue: '#1d4ed8',
        brightMagenta: '#6d28d9',
        brightCyan: '#0284c7',
        brightWhite: '#f3f4f6',
      },
    });

      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      try {
        term.open(termRef.current);
        enforceSquareTerminal(termRef.current);
      } catch {
        termRef.current.textContent = t('terminal.unavailable');
        termRef.current.classList.add('terminal-shell-error');
        return;
      }

      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
      const wsURL = `${proto}://${window.location.host}/ws/term`;
      ws = new WebSocket(wsURL);
      wsRef.current = ws;

      ws.onopen = () => {
        term?.write(`\x1b[32m${t('terminal.connected')}\x1b[0m\r\n`);
      };

      ws.onmessage = (event) => {
        if (!term) {
          return;
        }
        if (event.data instanceof Blob) {
          event.data.text().then((text) => term?.write(text));
        } else {
          term.write(event.data);
        }
      };

      ws.onclose = () => {
        term?.write(`\r\n\x1b[31m${t('terminal.disconnected')}\x1b[0m\r\n`);
      };

      term.onData((data) => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      });

      const fitTerminal = () => {
        const host = termRef.current;
        if (!host || host.clientWidth === 0 || host.clientHeight === 0) {
          return;
        }
        window.requestAnimationFrame(() => {
          try {
            fitAddon.fit();
          } catch {
            // Ignore transient layout races while the terminal host is resizing.
          }
        });
      };
      openDelay = window.setTimeout(fitTerminal, 0);

      resizeObserver = new ResizeObserver(() => {
        fitTerminal();
      });
      resizeObserver.observe(termRef.current);
      if (termRef.current.parentElement) {
        resizeObserver.observe(termRef.current.parentElement);
      }
      window.addEventListener('resize', fitTerminal);
      detachWindowResize = () => {
        window.removeEventListener('resize', fitTerminal);
      };
    };

    void mountTerminal();

    return () => {
      cancelled = true;
      window.clearTimeout(openDelay);
      resizeObserver?.disconnect();
      detachWindowResize?.();
      ws?.close();
      term?.dispose();
    };
  }, [t]);

  return (
    <div
      ref={termRef}
      className={`terminal-shell ${className ?? ''}`.trim()}
      aria-label={t('app.panel.terminal')}
      role="region"
    />
  );
});
