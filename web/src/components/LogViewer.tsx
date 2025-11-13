import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';
import { useAuthStore } from '../stores/authStore';

interface LogViewerProps {
  containerId: string;
  hostId: string;
  containerName: string;
  hostName?: string;
  onClose: () => void;
  follow?: boolean;
  tail?: string;
  timestamps?: boolean;
  showHeader?: boolean; // Whether to show the header with Close button
}

// Sanitize log data to prevent xterm parsing errors
const sanitizeLogData = (data: string): string => {
  if (!data) return '';

  // Build regex pattern to match control characters
  // Using Unicode escape sequences to avoid linter warnings
  const nullByte = String.fromCharCode(0x00);
  const tab = String.fromCharCode(0x09);
  const cr = String.fromCharCode(0x0D);
  const del = String.fromCharCode(0x7F);

  // Create regex that matches control chars except \n, \r, \t
  const controlChars = `[${nullByte}-${tab}\x0B\x0C${cr}\x0E-\x1F${del}]`;
  const controlCharRegex = new RegExp(controlChars, 'g');

  // Remove null bytes and problematic control characters
  let sanitized = data.replace(controlCharRegex, '');

  // Remove incomplete ANSI escape sequences
  const esc = String.fromCharCode(0x1B);
  const ansiRegex = new RegExp(`${esc}\\[[^\x40-\x7E]*$`, 'g');
  sanitized = sanitized.replace(ansiRegex, '');

  return sanitized;
};

const LogViewer: React.FC<LogViewerProps> = ({
  containerId,
  hostId,
  containerName,
  hostName,
  onClose,
  follow = true,
  tail = '100',
  timestamps = true,
  showHeader = true,
}) => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const terminalInstanceRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const searchAddonRef = useRef<SearchAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const { accessToken } = useAuthStore();

  const connectWebSocket = useCallback(() => {
    // Close existing connection if any
    if (wsRef.current) {
      wsRef.current.close();
    }

    // Check if we have a valid access token
    if (!accessToken) {
      console.error('No access token available for WebSocket connection');
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Always request logs with follow=false first to get historical logs, then enable follow if needed
    const followParam = follow ? 'true' : 'false';
    const wsUrl = `${protocol}//${window.location.host}/ws/logs/${hostId}/${containerId}?follow=${followParam}&tail=${tail}&timestamps=${timestamps}&token=${encodeURIComponent(accessToken)}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setIsConnected(true);
      const hostInfo = hostName ? ` on ${hostName}` : '';
      terminalInstanceRef.current?.writeln(`\x1b[32mConnected to logs for ${containerName}${hostInfo}\x1b[0m`);
      terminalInstanceRef.current?.writeln(`\x1b[36mStreaming logs (follow: ${follow}, tail: ${tail}, timestamps: ${timestamps})\x1b[0m`);
      terminalInstanceRef.current?.writeln('─'.repeat(80));
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        if (data.type === 'log_data') {
          let logData = data.payload.data;

          // Sanitize the log data to prevent xterm parsing errors
          logData = sanitizeLogData(logData);

          const timestamp = data.payload.timestamp ? `[${new Date(data.payload.timestamp).toISOString()}] ` : '';
          const stream = data.payload.stream === 'stderr' ? '\x1b[31m' : '\x1b[0m';

          // Write line by line to handle newlines properly
          const lines = logData.split('\n');
          lines.forEach((line: string, index: number) => {
            if (index < lines.length - 1) {
              // Full line with newline
              terminalInstanceRef.current?.writeln(`${stream}${timestamp}${line}\x1b[0m`);
            } else if (line) {
              // Partial line without newline
              terminalInstanceRef.current?.write(`${stream}${timestamp}${line}\x1b[0m`);
            }
          });
        } else if (data.type === 'log_error') {
          terminalInstanceRef.current?.writeln(`\x1b[31mError: ${data.payload.error}\x1b[0m`);
        }
      } catch (error) {
        console.error('Error parsing log data:', error, event.data);
        // If it's not JSON, treat as raw log data (sanitized)
        const sanitized = sanitizeLogData(event.data);
        terminalInstanceRef.current?.write(sanitized);
      }
    };

    ws.onclose = (event) => {
      setIsConnected(false);
      terminalInstanceRef.current?.writeln(`\x1b[33mLog stream disconnected (code: ${event.code})\x1b[0m`);
    };

    ws.onerror = (error) => {
      setIsConnected(false);
      console.error('WebSocket error:', error);
      terminalInstanceRef.current?.writeln('\x1b[31mWebSocket error occurred\x1b[0m');
    };
  }, [hostId, containerId, containerName, hostName, follow, tail, timestamps, accessToken]);

  useEffect(() => {
    if (!terminalRef.current) return;

    // Create terminal instance
    const terminal = new Terminal({
      theme: {
        background: '#1a1a1a',
        foreground: '#ffffff',
        cursor: '#ffffff',
      },
      fontSize: 14,
      fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
      cursorBlink: true,
      scrollback: 10000,
    });

    // Add addons
    const fitAddon = new FitAddon();
    const searchAddon = new SearchAddon();
    const webLinksAddon = new WebLinksAddon();

    terminal.loadAddon(fitAddon);
    terminal.loadAddon(searchAddon);
    terminal.loadAddon(webLinksAddon);

    // Open terminal
    terminal.open(terminalRef.current);
    fitAddon.fit();

    // Store references
    terminalInstanceRef.current = terminal;
    fitAddonRef.current = fitAddon;
    searchAddonRef.current = searchAddon;

    // Handle terminal resize
    const handleResize = () => {
      fitAddon.fit();
    };

    window.addEventListener('resize', handleResize);

    // Connect to WebSocket for log streaming
    connectWebSocket();

    return () => {
      window.removeEventListener('resize', handleResize);
      if (wsRef.current) {
        wsRef.current.close();
      }
      terminal.dispose();
    };
  }, [connectWebSocket]);

  const handleSearch = (term: string) => {
    if (!searchAddonRef.current) return;

    if (term) {
      searchAddonRef.current.findNext(term);
    }
  };

  const handleClear = () => {
    terminalInstanceRef.current?.clear();
  };

  const handleDownload = () => {
    const terminal = terminalInstanceRef.current;
    if (!terminal) return;

    let logContent = '';
    const buffer = terminal.buffer.active;
    for (let i = 0; i < buffer.length; i++) {
      const line = buffer.getLine(i);
      if (line) {
        logContent += line.translateToString() + '\n';
      }
    }

    const blob = new Blob([logContent], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${containerName}-logs-${new Date().toISOString().split('T')[0]}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex flex-col h-full bg-white dark:bg-gray-900">
      {/* Header */}
      {showHeader && (
        <div className="flex items-center justify-between p-4 bg-gray-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center space-x-4">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">
              Logs: {containerName}
              {hostName && <span className="text-gray-600 dark:text-gray-400 ml-2">({hostName})</span>}
            </h3>
            <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {isConnected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <button
              onClick={onClose}
              className="btn btn-sm btn-secondary"
            >
              Close
            </button>
          </div>
        </div>
      )}

      {/* Search Bar */}
      <div className="flex items-center p-2 bg-gray-100 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        {!showHeader && (
          <div className="flex items-center space-x-2 mr-3">
            <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
            <span className="text-xs text-gray-600 dark:text-gray-400">
              {isConnected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
        )}
        <input
          type="text"
          placeholder="Search logs..."
          value={searchTerm}
          onChange={(e) => {
            setSearchTerm(e.target.value);
            handleSearch(e.target.value);
          }}
          className="flex-1 px-3 py-1 text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded border border-gray-300 dark:border-gray-600 focus:outline-none focus:border-blue-500"
        />
        <button
          onClick={() => handleSearch(searchTerm)}
          className="ml-2 btn btn-sm btn-primary"
        >
          Search
        </button>
        <button
          onClick={() => {
            setSearchTerm('');
            handleSearch('');
          }}
          className="ml-1 btn btn-sm btn-secondary"
        >
          Clear
        </button>
      </div>

      {/* Terminal */}
      <div className="flex-1 p-2">
        <div ref={terminalRef} className="h-full w-full" />
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between p-2 bg-gray-100 dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700">
        <div className="flex items-center space-x-4">
          <button
            onClick={handleClear}
            className="btn btn-sm btn-secondary"
          >
            Clear
          </button>
          <button
            onClick={handleDownload}
            className="btn btn-sm btn-secondary"
          >
            Download
          </button>
        </div>
        <div className="text-xs text-gray-600 dark:text-gray-400">
          {follow ? 'Following logs' : 'Static logs'} • Tail: {tail} • Timestamps: {timestamps ? 'On' : 'Off'}
        </div>
      </div>
    </div>
  );
};

export default LogViewer;
