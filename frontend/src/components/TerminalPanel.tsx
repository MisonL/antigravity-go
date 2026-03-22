import React, { useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';

interface TerminalPanelProps {
  className?: string;
}

export const TerminalPanel: React.FC<TerminalPanelProps> = ({ className }) => {
  const token = (typeof window !== 'undefined'
    ? new URLSearchParams(window.location.search).get('token')?.trim() || ''
    : '');

  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!termRef.current) return;

    // Create terminal
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'JetBrains Mono', monospace",
      theme: {
        background: '#ffffff',
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
      }
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(termRef.current);
    fitAddon.fit();

    terminalRef.current = term;

    // Connect to WebSocket
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const wsURL = `${proto}://${window.location.host}/ws/term${token ? `?token=${encodeURIComponent(token)}` : ''}`;
    const ws = new WebSocket(wsURL);
    wsRef.current = ws;

    ws.onopen = () => {
      term.write('\x1b[32m已连接终端。\x1b[0m\r\n');
    };

    ws.onmessage = (event) => {
      // Binary data from PTY
      if (event.data instanceof Blob) {
        event.data.text().then((text) => term.write(text));
      } else {
        term.write(event.data);
      }
    };

    ws.onclose = () => {
      term.write('\r\n\x1b[31m连接已断开。\x1b[0m\r\n');
    };

    // Send input to PTY
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    // Handle resize
    const handleResize = () => {
      fitAddon.fit();
    };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      ws.close();
      term.dispose();
    };
  }, []);

    return (
      <div 
        ref={termRef} 
        className={className}
        style={{ 
          height: '100%', 
          width: '100%',
          padding: '8px',
          background: 'rgba(255,255,255,0.92)',
          borderRadius: '0 0 8px 8px',
        }} 
      />
    );
  };
