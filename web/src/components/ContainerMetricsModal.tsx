import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../api/client';
import Modal from './Modal';
import { MetricsChart } from './MetricsChart';
import type { MetricDataPoint } from '../types';

interface ContainerMetricsModalProps {
  isOpen: boolean;
  onClose: () => void;
  hostId: string;
  containerId: string;
  containerName?: string;
  hostName?: string;
}

export const ContainerMetricsModal: React.FC<ContainerMetricsModalProps> = ({ isOpen, onClose, hostId, containerId, containerName, hostName }) => {
  const [timeRange, setTimeRange] = useState<"1h" | "6h" | "24h" | "7d">("1h");
  const [interval, setInterval] = useState<string>("1m");

  const { start, end } = getTimeRange(timeRange);

  const { data, isLoading, isFetching, error } = useQuery({
    queryKey: ['container-metrics-detail', hostId, containerId, timeRange, interval],
    queryFn: async () => {
      const res = await apiClient.getContainerMetrics(hostId, containerId, {
        start: start.toISOString(),
        end: end.toISOString(),
        interval,
      });
      return res.metrics;
    },
    enabled: isOpen && !!hostId && !!containerId,
    refetchInterval: 30000,
  });

  const cpuData: MetricDataPoint[] = data?.map((m) => ({ timestamp: m.timestamp ?? new Date().toISOString(), value: m.cpu_percent })) ?? [];
  const memData: MetricDataPoint[] = data?.map((m) => ({ timestamp: m.timestamp ?? new Date().toISOString(), value: m.memory_usage })) ?? [];
  const memPctData: MetricDataPoint[] = data?.map((m) => ({ timestamp: m.timestamp ?? new Date().toISOString(), value: m.memory_limit > 0 ? (m.memory_usage / m.memory_limit) * 100 : 0 })) ?? [];
  const diskData: MetricDataPoint[] = data?.map((m) => ({ timestamp: m.timestamp ?? new Date().toISOString(), value: (m.disk_read_bytes ?? 0) + (m.disk_write_bytes ?? 0) })) ?? [];

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`Metrics • ${containerName ?? containerId}`} size="xl">
      {error ? (
        <div className="p-4 border border-red-200 rounded-lg bg-red-50 text-sm text-red-600">Failed to load container metrics</div>
      ) : (isLoading || isFetching) ? (
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        </div>
      ) : (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            {hostName && (
              <div className="text-sm text-gray-600">Host: {hostName}</div>
            )}
            <div className="flex items-center gap-2">
              <select
                value={timeRange}
                onChange={(e) => {
                  const val = e.target.value as "1h" | "6h" | "24h" | "7d";
                  setTimeRange(val);
                  setInterval(getIntervalForRange(val));
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

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <MetricsChart data={cpuData} title="CPU Usage" unit="%" color="#3b82f6" height={250} />
            </div>
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <MetricsChart data={memData} title="Memory Usage" unit="bytes" color="#10b981" height={250} />
            </div>
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <MetricsChart data={memPctData} title="Memory Usage (%)" unit="%" color="#8b5cf6" height={250} />
            </div>
            <div className="p-4 border border-gray-200 rounded-lg bg-white">
              <MetricsChart data={diskData} title="Disk I/O (Total)" unit="bytes" color="#f59e0b" height={250} />
            </div>
          </div>
        </div>
      )}
    </Modal>
  );
};

function getTimeRange(range: "1h" | "6h" | "24h" | "7d") {
  const end = new Date();
  const start = new Date();
  switch (range) {
    case "1h": start.setHours(start.getHours() - 1); break;
    case "6h": start.setHours(start.getHours() - 6); break;
    case "24h": start.setHours(start.getHours() - 24); break;
    case "7d": start.setDate(start.getDate() - 7); break;
  }
  return { start, end };
}

function getIntervalForRange(range: "1h" | "6h" | "24h" | "7d"): string {
  switch (range) {
    case "1h": return "1m";
    case "6h": return "5m";
    case "24h": return "15m";
    case "7d": return "1h";
  }
}

export default ContainerMetricsModal;


