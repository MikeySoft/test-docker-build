import React from "react";
import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../api/client";
import type { AppLogEntry } from "../../types";
import { useAuthStore } from "../../stores/authStore";

type LevelFilter = "all" | "info" | "warn" | "error";

const MAX_LOGS = 500;

const levelColors: Record<string, string> = {
  error: "text-red-500 dark:text-red-400",
  warn: "text-yellow-600 dark:text-yellow-400",
  warning: "text-yellow-600 dark:text-yellow-400",
  info: "text-cyan-600 dark:text-cyan-400",
  debug: "text-gray-500 dark:text-gray-400",
};

function normalizeLevel(level: string | undefined): string {
  const value = level?.toLowerCase() ?? "info";
  if (value === "warning") return "warn";
  if (value === "err") return "error";
  return value;
}

const formatTimestamp = (iso: string) => {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
};

const truncate = (value: string, max = 120) => {
  if (value.length <= max) return value;
  return `${value.slice(0, max)}…`;
};

const mergeLogs = (existing: AppLogEntry[], incoming: AppLogEntry[]) => {
  if (!incoming.length) return existing;
  const seen = new Set(existing.map((entry) => entry.id));
  const merged = [...existing];
  for (const entry of incoming) {
    if (!seen.has(entry.id)) {
      merged.push(entry);
      seen.add(entry.id);
    }
  }
  if (merged.length > MAX_LOGS) {
    return merged.slice(merged.length - MAX_LOGS);
  }
  return merged;
};

const Logs: React.FC = () => {
  const { accessToken } = useAuthStore();
  const [logs, setLogs] = React.useState<AppLogEntry[]>([]);
  const [isConnected, setIsConnected] = React.useState(false);
  const [follow, setFollow] = React.useState(true);
  const [levelFilter, setLevelFilter] = React.useState<LevelFilter>("all");
  const [search, setSearch] = React.useState("");
  const [lastCursor, setLastCursor] = React.useState<string | undefined>(undefined);
  const listRef = React.useRef<HTMLDivElement>(null);
  const wsRef = React.useRef<WebSocket | null>(null);

  const { isFetching, refetch } = useQuery({
    queryKey: ["app-logs", "initial"],
    queryFn: async () => {
      const response = await apiClient.getAppLogs(undefined, 200);
      setLogs(response.logs ?? []);
      setLastCursor(response.next_cursor ?? response.logs?.at(-1)?.id);
      return response;
    },
  });

  React.useEffect(() => {
    if (!accessToken) {
      return;
    }

    const wsUrl = apiClient.getAppLogsWebSocketURL(accessToken);
    const socket = new WebSocket(wsUrl);
    wsRef.current = socket;

    socket.onopen = () => setIsConnected(true);
    socket.onclose = () => setIsConnected(false);
    socket.onerror = () => setIsConnected(false);
    socket.onmessage = (event) => {
      try {
        const entry = JSON.parse(event.data) as AppLogEntry;
        setLogs((prev) => {
          const next = mergeLogs(prev, [entry]);
          setLastCursor(entry.id);
          return next;
        });
      } catch (error) {
        console.error("Failed to parse app log entry", error, event.data);
      }
    };

    return () => {
      socket.close();
    };
  }, [accessToken]);

  React.useEffect(() => {
    if (!follow) return;
    const container = listRef.current;
    if (!container) return;
    const handle = window.requestAnimationFrame(() => {
      container.scrollTo({ top: container.scrollHeight, behavior: "smooth" });
    });
    return () => window.cancelAnimationFrame(handle);
  }, [logs, follow]);

  const handleRefresh = async () => {
    const latestId = logs.at(-1)?.id ?? lastCursor;
    try {
      const response = await apiClient.getAppLogs(latestId, 200);
      setLogs((prev) => mergeLogs(prev, response.logs ?? []));
      if (response.logs?.length) {
        setLastCursor(response.logs.at(-1)?.id);
      }
    } catch (error) {
      console.error("Failed to refresh logs", error);
    }
  };

  const filteredLogs = React.useMemo(() => {
    const searchTerm = search.trim().toLowerCase();
    return logs.filter((entry) => {
      const level = normalizeLevel(entry.level);
      if (levelFilter !== "all" && level !== levelFilter) {
        return false;
      }
      if (searchTerm) {
        const candidate = `${entry.message} ${entry.source ?? ""} ${JSON.stringify(entry.fields ?? {})}`.toLowerCase();
        if (!candidate.includes(searchTerm)) {
          return false;
        }
      }
      return true;
    });
  }, [logs, levelFilter, search]);

  const latestTimestamp = logs.at(-1)?.timestamp;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-sm text-gray-600 dark:text-gray-400">Stream</span>
          <span className={`inline-flex items-center gap-1 text-sm font-medium ${isConnected ? "text-green-600 dark:text-green-400" : "text-red-500 dark:text-red-400"}`}>
            <span className={`h-2 w-2 rounded-full ${isConnected ? "bg-green-500" : "bg-red-500"}`} />
            {isConnected ? "Connected" : "Disconnected"}
          </span>
        </div>
        <button
          className="btn btn-secondary"
          onClick={() => {
            setFollow((prev) => !prev);
          }}
        >
          {follow ? "Pause Autoscroll" : "Resume Autoscroll"}
        </button>
        <button className="btn btn-secondary" onClick={() => { void refetch(); }}>
          {isFetching ? "Refreshing…" : "Reload"}
        </button>
        <button className="btn btn-secondary" onClick={() => { void handleRefresh(); }}>
          Fetch Newer
        </button>
        <div className="flex items-center gap-2">
          <label htmlFor="level-filter" className="text-sm text-gray-600 dark:text-gray-400">
            Level
          </label>
          <select
            id="level-filter"
            className="form-select text-sm"
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value as LevelFilter)}
          >
            <option value="all">All</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </select>
        </div>
        <input
          type="text"
          className="form-input text-sm flex-1 min-w-[200px]"
          placeholder="Search logs (message, source, fields)"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {latestTimestamp && (
          <span className="text-xs text-gray-500 dark:text-gray-400">
            Latest at {formatTimestamp(latestTimestamp)}
          </span>
        )}
      </div>

      <div
        ref={listRef}
        className="border border-gray-200 dark:border-gray-800 rounded-md bg-white dark:bg-gray-950 h-[520px] overflow-y-auto font-mono text-sm"
      >
        {filteredLogs.length === 0 ? (
          <div className="flex items-center justify-center h-full text-gray-500 dark:text-gray-400">
            No logs match the selected filters.
          </div>
        ) : (
          <ul className="divide-y divide-gray-200 dark:divide-gray-800">
            {filteredLogs.map((entry) => {
              const level = normalizeLevel(entry.level);
              const levelClass = levelColors[level] ?? "text-gray-600 dark:text-gray-300";
              return (
                <li key={entry.id} className="px-4 py-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-3">
                      <span className={`${levelClass} uppercase text-xs font-semibold tracking-wide`}>
                        {level}
                      </span>
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        {formatTimestamp(entry.timestamp)}
                      </span>
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        {entry.source}
                      </span>
                    </div>
                    {entry.fields && Object.keys(entry.fields).length > 0 && (
                      <details className="text-xs text-gray-500 dark:text-gray-400">
                        <summary className="cursor-pointer text-xs text-gray-500 dark:text-gray-400">
                          Fields
                        </summary>
                        <pre className="text-xs whitespace-pre-wrap break-all mt-1 text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-gray-900 p-2 rounded">
                          {JSON.stringify(entry.fields, null, 2)}
                        </pre>
                      </details>
                    )}
                  </div>
                  <p className="mt-1 text-sm text-gray-900 dark:text-gray-100">{truncate(entry.message, 400)}</p>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
};

export default Logs;

