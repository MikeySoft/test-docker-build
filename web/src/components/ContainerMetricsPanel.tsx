import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import apiClient from "../api/client";
import { MetricsChart } from "./MetricsChart";
import type { MetricDataPoint } from "../types";

interface ContainerMetricsPanelProps {
  hostId: string;
  containerId: string;
  containerName: string;
}

export function ContainerMetricsPanel({
  hostId,
  containerId,
  containerName,
}: ContainerMetricsPanelProps) {
  const [timeRange, setTimeRange] = useState<"1h" | "6h" | "24h" | "7d">("1h");
  const [interval, setInterval] = useState<string>("1m");

  // Calculate start and end times based on time range
  const { start, end } = getTimeRange(timeRange);

  // Fetch metrics
  const { data, isLoading, error } = useQuery({
    queryKey: ["container-metrics", hostId, containerId, start, end, interval],
    queryFn: async () => {
      const response = await apiClient.getContainerMetrics(hostId, containerId, {
        start: start.toISOString(),
        end: end.toISOString(),
        interval,
      });
      return response.metrics;
    },
    refetchInterval: 30000, // Refetch every 30 seconds
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

  const diskReadData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: m.disk_read_bytes,
  })) || [];

  const diskWriteData: MetricDataPoint[] = data?.map((m, index) => ({
    timestamp: new Date(Date.now() - (data.length - index) * 30000).toISOString(),
    value: m.disk_write_bytes,
  })) || [];

  if (error) {
    return (
      <div className="p-4 border border-red-200 rounded-lg bg-red-50">
        <p className="text-red-600 text-sm">
          Failed to load metrics: {error instanceof Error ? error.message : "Unknown error"}
        </p>
      </div>
    );
  }

  // Show empty state if not loading and no data
  if (!isLoading && data && data.length === 0) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">
            Metrics for {containerName}
          </h2>
        </div>
        <div className="p-8 border border-gray-200 rounded-lg bg-gray-50 text-center">
          <p className="text-gray-600 text-sm">
            No metrics data available for this container yet.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with time range selector */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">
          Metrics for {containerName}
        </h2>
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

      {isLoading ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
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

          {/* Disk Read */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={diskReadData}
              title="Disk Read"
              unit="bytes"
              color="#f59e0b"
              height={250}
            />
          </div>

          {/* Disk Write */}
          <div className="p-4 border border-gray-200 rounded-lg bg-white">
            <MetricsChart
              data={diskWriteData}
              title="Disk Write"
              unit="bytes"
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

export default ContainerMetricsPanel;

