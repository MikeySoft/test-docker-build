import React from 'react';
import { useQuery } from '@tanstack/react-query';
import apiClient from '../api/client';
import ContainerMetricsModal from './ContainerMetricsModal';
import type { MetricDataPoint } from '../types';
import {
  LineChart,
  Line,
  ResponsiveContainer,
} from 'recharts';

interface ContainerMiniMetricsProps {
  hostId: string;
  containerId: string;
  isRunning: boolean;
  containerName?: string;
  hostName?: string;
}

const MiniSparkline: React.FC<{ data: MetricDataPoint[]; color: string; height?: number }> = ({ data, color, height = 24 }) => {
  if (!data || data.length === 0) {
    return <div style={{ width: 80, height }} />;
  }
  return (
    <div style={{ width: 80, height }}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data.map(d => ({ x: new Date(d.timestamp).getTime(), y: d.value }))}>
          <Line type="monotone" dataKey="y" stroke={color} dot={false} strokeWidth={2} isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

const ContainerMiniMetrics: React.FC<ContainerMiniMetricsProps> = ({ hostId, containerId, isRunning, containerName, hostName }) => {
  const [open, setOpen] = React.useState(false);
  const { data } = useQuery({
    queryKey: ['container-mini-metrics', hostId, containerId],
    queryFn: async () => {
      const end = new Date();
      const start = new Date(end.getTime() - 10 * 60 * 1000); // last 10 min
      const res = await apiClient.getContainerMetrics(hostId, containerId, {
        start: start.toISOString(),
        end: end.toISOString(),
        interval: '30s',
      });
      const metrics = Array.isArray(res.metrics) ? res.metrics : [];
      const cpu: MetricDataPoint[] = metrics.map((m: any) => ({
        timestamp: m.timestamp || end.toISOString(),
        value: typeof m.cpu_percent === 'number' ? m.cpu_percent : 0,
      }));
      const memPct: MetricDataPoint[] = metrics.map((m: any) => ({
        timestamp: m.timestamp || end.toISOString(),
        value:
          typeof m.memory_usage === 'number' && typeof m.memory_limit === 'number' && m.memory_limit > 0
            ? (m.memory_usage / m.memory_limit) * 100
            : 0,
      }));
      return { cpu, memPct };
    },
    enabled: !!hostId && !!containerId && isRunning,
    refetchInterval: 10000,
  });

  if (!isRunning || !data) return null;

  const onKey = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      setOpen(true);
    }
  };

  return (
    <>
      <button type="button" className="flex items-center gap-3 cursor-pointer" onClick={() => setOpen(true)} onKeyDown={onKey} title="View detailed metrics">
        <div className="flex items-center gap-1 text-xs text-gray-500">
          <span>CPU</span>
          <MiniSparkline data={data.cpu} color="#3b82f6" />
        </div>
        <div className="flex items-center gap-1 text-xs text-gray-500">
          <span>Mem</span>
          <MiniSparkline data={data.memPct} color="#10b981" />
        </div>
      </button>
      {/* Detail modal as sibling to avoid nested button interactions */}
      <ContainerMetricsModal
        isOpen={open}
        onClose={() => setOpen(false)}
        hostId={hostId}
        containerId={containerId}
        containerName={containerName}
        hostName={hostName}
      />
    </>
  );
};

export default ContainerMiniMetrics;


