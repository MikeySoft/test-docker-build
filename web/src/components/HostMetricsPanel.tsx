import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BarChart3 } from "lucide-react";
import apiClient from "../api/client";
import { MetricsChart } from "./MetricsChart";
import type { MetricDataPoint } from "../types";

interface HostMetricsPanelProps {
  hostId: string;
}

export function HostMetricsPanel({ hostId }: HostMetricsPanelProps) {
  const [timeRange, setTimeRange] = useState<"1h" | "6h" | "24h" | "7d">("1h");
  const [interval, setInterval] = useState<string>("1m");

  // Calculate start and end times based on time range
  const { start, end } = getTimeRange(timeRange);

  // Fetch metrics
  const { data, isLoading, error, isFetching } = useQuery({
    queryKey: ["host-metrics", hostId, timeRange, interval],
    queryFn: async () => {
      const response = await apiClient.getHostMetrics(hostId, {
        start: start.toISOString(),
        end: end.toISOString(),
        interval,
      });
      return response.metrics;
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });

  // Fetch static/slow-changing host info for cores and totals
  const { data: hostInfo } = useQuery({
    queryKey: ["host-info", hostId],
    queryFn: () => apiClient.getHostInfo(hostId),
    refetchInterval: 5 * 60 * 1000, // 5 minutes
  });

  // Transform metrics for charts
  const cpuData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: m.cpu_percent,
  })) || [];

  const memoryData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: m.memory_usage,
  })) || [];

  const memoryPercentData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: (m.memory_usage / m.memory_total) * 100,
  })) || [];

  const diskData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: m.disk_usage,
  })) || [];

  const diskPercentData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: (m.disk_usage / m.disk_total) * 100,
  })) || [];

  // Latest snapshot for summary cards
  const latest = data && data.length > 0 ? data[data.length - 1] : undefined;

  if (error) {
    return (
      <div className="p-4 border border-red-200 rounded-lg bg-red-50">
        <p className="text-red-600 text-sm">
          Failed to load metrics: {error instanceof Error ? error.message : "Unknown error"}
        </p>
        <p className="text-gray-600 text-xs mt-2">
          Note: Host metrics collection is optional. Enable it by setting METRICS_COLLECT_HOST_STATS=true on the agent.
        </p>
      </div>
    );
  }

  // Show empty state if not loading and no data
  if (!isLoading && !isFetching && data && data.length === 0) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Host Metrics</h2>
        </div>
        <div className="p-8 border border-gray-200 rounded-lg bg-gray-50 text-center">
          <BarChart3 className="h-12 w-12 mx-auto text-gray-400 mb-4" />
          <h3 className="text-lg font-semibold text-gray-900 mb-2">No Metrics Data Available</h3>
          <p className="text-gray-600 text-sm mb-4">
            Metrics will appear here once the agent starts collecting data.
          </p>
          <div className="text-left max-w-md mx-auto text-xs text-gray-500">
            <p className="mb-2">To enable metrics collection:</p>
            <ul className="list-disc list-inside space-y-1">
              <li>Ensure InfluxDB is running: <code className="bg-gray-200 px-1 rounded">docker-compose up -d influxdb</code></li>
              <li>Set <code className="bg-gray-200 px-1 rounded">INFLUXDB_ENABLED=true</code> on the server</li>
              <li>Set <code className="bg-gray-200 px-1 rounded">METRICS_ENABLED=true</code> on the agent</li>
              <li>Wait 30-60 seconds for data to be collected</li>
            </ul>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with time range selector */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Host Metrics</h2>
        <div className="flex items-center gap-2">
          <select
            value={timeRange}
            onChange={(e) => {
              setTimeRange(e.target.value as "1h" | "6h" | "24h" | "7d");
              setInterval(getIntervalForRange(e.target.value as "1h" | "6h" | "24h" | "7d"));
            }}
            className="px-3 py-1 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="1h">Last Hour</option>
            <option value="6h">Last 6 Hours</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
          </select>
        </div>
      </div>

      {(isLoading || isFetching) ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Summary cards */}
          <div className="md:col-span-2 grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <div className="text-sm text-gray-600">CPU Cores</div>
              <div className="mt-1 text-2xl font-semibold">{hostInfo?.ncpu ?? "—"}</div>
            </div>
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <div className="text-sm text-gray-600">Memory</div>
              <div className="mt-1 text-2xl font-semibold">{hostInfo ? formatBytes(hostInfo.mem_total) : latest ? formatBytes(latest.memory_total) : "—"}</div>
            </div>
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <div className="text-sm text-gray-600">Disk</div>
              <div className="mt-1 text-2xl font-semibold">{hostInfo?.disk_total ? formatBytes(hostInfo.disk_total) : latest ? formatBytes(latest.disk_total) : "—"}</div>
            </div>
          </div>
          {/* CPU Usage */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={cpuData}
              title="CPU Usage"
              unit="%"
              color="#3b82f6"
              height={250}
            />
          </div>

          {/* Memory Usage */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={memoryData}
              title="Memory Usage"
              unit="bytes"
              color="#10b981"
              height={250}
            />
          </div>

          {/* Memory Percent */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={memoryPercentData}
              title="Memory Usage (%)"
              unit="%"
              color="#8b5cf6"
              height={250}
            />
          </div>

          {/* Disk Usage */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={diskData}
              title="Disk Usage"
              unit="bytes"
              color="#f59e0b"
              height={250}
            />
          </div>

          {/* Disk Percent */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={diskPercentData}
              title="Disk Usage (%)"
              unit="%"
              color="#ef4444"
              height={250}
            />
          </div>
        </div>
      )}
    </div>
  );
}

function getTimeRange(range: "1h" | "6h" | "24h" | "7d") {
  const end = new Date();
  const start = new Date();

  switch (range) {
    case "1h":
      start.setHours(start.getHours() - 1);
      break;
    case "6h":
      start.setHours(start.getHours() - 6);
      break;
    case "24h":
      start.setHours(start.getHours() - 24);
      break;
    case "7d":
      start.setDate(start.getDate() - 7);
      break;
  }

  return { start, end };
}

function getIntervalForRange(range: "1h" | "6h" | "24h" | "7d"): string {
  switch (range) {
    case "1h":
      return "1m";
    case "6h":
      return "5m";
    case "24h":
      return "15m";
    case "7d":
      return "1h";
  }
}

export default HostMetricsPanel;

function formatBytes(bytes?: number) {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i++;
  }
  return `${val.toFixed(1)} ${units[i]}`;
}

