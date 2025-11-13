import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X, Play, Square, RotateCw, Container as ContainerIcon, BarChart3 } from 'lucide-react';
import { apiClient } from '../api/client';
import { useToast } from '../contexts/useToast';
import StatusBadge from './StatusBadge';
import LogViewer from './LogViewer';
import ContainerMiniMetrics from './ContainerMiniMetrics';
import { MetricsChart } from './MetricsChart';
import type { Stack, Container, MetricDataPoint } from '../types';

interface StackDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  stack: Stack;
  hostId: string;
}

const StackDetailModal: React.FC<StackDetailModalProps> = ({
  isOpen,
  onClose,
  stack,
  hostId,
}) => {
  const { showSuccess, showError } = useToast();
  const [activeTab, setActiveTab] = useState<'containers' | 'logs' | 'metrics'>('containers');
  const [selectedLogContainer, setSelectedLogContainer] = useState<string | null>(null);
  const [isActionLoading, setIsActionLoading] = useState<string | null>(null);

  // Fetch stack containers
  const {
    data: containers = [],
    isLoading: containersLoading,
    refetch: refetchContainers,
  } = useQuery({
    queryKey: ['stack-containers', hostId, stack.name],
    queryFn: () => apiClient.getStackContainers(hostId, stack.name),
    enabled: isOpen,
    refetchInterval: activeTab === 'containers' ? 5000 : false,
  });

  useEffect(() => {
    if (isOpen && containers.length > 0 && selectedLogContainer === null) {
      setSelectedLogContainer(containers[0].id);
    }
  }, [isOpen, containers, selectedLogContainer]);

  const handleContainerAction = async (
    containerId: string,
    action: 'start' | 'stop' | 'restart'
  ) => {
    setIsActionLoading(containerId);
    try {
      await apiClient.stackContainerAction(hostId, stack.name, containerId, action);
      showSuccess(`Container ${action}{ed} successfully`);
      refetchContainers();
    } catch (error: any) {
      showError(`Failed to ${action} container: ${error.message}`);
    } finally {
      setIsActionLoading(null);
    }
  };

  const getContainerState = (container: Container): 'running' | 'stopped' | 'error' => {
    const state = container.state?.toLowerCase();
    if (state === 'running') return 'running';
    if (state === 'stopped' || state === 'exited') return 'stopped';
    return 'error';
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white dark:bg-gray-900 rounded-xl shadow-xl w-full max-w-4xl max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-800">
          <div>
            <h2 className="text-xl font-bold text-gray-900 dark:text-white font-space">
              {stack.name}
            </h2>
            <p className="text-sm text-gray-600 dark:text-gray-400 font-inter">
              {stack.host_name} • {containers.length} containers
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-gray-200 dark:border-gray-800">
          <button
            onClick={() => setActiveTab('containers')}
            className={`px-6 py-3 font-medium transition-colors ${
              activeTab === 'containers'
                ? 'text-primary-600 dark:text-primary-400 border-b-2 border-primary-600 dark:border-primary-400'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
            }`}
          >
            Containers
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`px-6 py-3 font-medium transition-colors ${
              activeTab === 'logs'
                ? 'text-primary-600 dark:text-primary-400 border-b-2 border-primary-600 dark:border-primary-400'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
            }`}
          >
            Logs
          </button>
          <button
            onClick={() => setActiveTab('metrics')}
            className={`px-6 py-3 font-medium transition-colors ${
              activeTab === 'metrics'
                ? 'text-primary-600 dark:text-primary-400 border-b-2 border-primary-600 dark:border-primary-400'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
            }`}
          >
            <BarChart3 className="h-4 w-4 inline mr-2" />
            Metrics
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {activeTab === 'containers' && (
            <div className="space-y-3">
              {containersLoading ? (
                <div className="flex items-center justify-center py-8">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
                </div>
              ) : containers.length === 0 ? (
                <div className="text-center py-8">
                  <ContainerIcon className="h-12 w-12 mx-auto text-gray-400 mb-2" />
                  <p className="text-gray-600 dark:text-gray-400">No containers found</p>
                </div>
              ) : (
                containers.map((container) => {
                  const state = getContainerState(container);
                  return (
                    <div
                      key={container.id}
                      className="border border-gray-200 dark:border-gray-800 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-3">
                            <ContainerIcon className="h-5 w-5 text-gray-500 flex-shrink-0" />
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <button
                                  onClick={() => {
                                    setSelectedLogContainer(container.id);
                                    setActiveTab('logs');
                                  }}
                                  className="text-sm font-medium text-gray-900 dark:text-white truncate font-space hover:text-primary-600 dark:hover:text-primary-400 transition-colors text-left"
                                  title="View logs"
                                >
                                  {container.name}
                                </button>
                                <StatusBadge status={state} />
                              </div>
                              <p className="text-xs text-gray-600 dark:text-gray-400 mt-1 font-inter">
                                {container.image}
                              </p>
                            </div>
                          </div>
                        </div>
                      <div className="flex items-center gap-3 ml-4">
                        {/* Mini metrics positioned next to action buttons */}
                        <ContainerMiniMetrics hostId={hostId} containerId={container.id} isRunning={state === 'running'} containerName={container.name} hostName={stack.host_name} />
                          {state === 'stopped' ? (
                            <button
                              onClick={() => handleContainerAction(container.id, 'start')}
                              disabled={isActionLoading === container.id}
                              className="btn btn-success text-sm px-3 py-1.5"
                              title="Start container"
                            >
                              <Play className="h-4 w-4" />
                            </button>
                          ) : (
                            <>
                              <button
                                onClick={() => handleContainerAction(container.id, 'stop')}
                                disabled={isActionLoading === container.id}
                                className="btn btn-warning text-sm px-3 py-1.5"
                                title="Stop container"
                              >
                                <Square className="h-4 w-4" />
                              </button>
                              <button
                                onClick={() => handleContainerAction(container.id, 'restart')}
                                disabled={isActionLoading === container.id}
                                className="btn btn-info text-sm px-3 py-1.5"
                                title="Restart container"
                              >
                                <RotateCw className="h-4 w-4" />
                              </button>
                            </>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}

          {activeTab === 'logs' && (
            <div>
              {containers.length === 0 ? (
                <div className="text-center py-8">
                  <p className="text-gray-600 dark:text-gray-400">No containers available</p>
                </div>
              ) : (
                <>
                  {/* Container selector */}
                  <div className="mb-4 flex gap-2 overflow-x-auto pb-2">
                    {containers.map((container) => (
                      <button
                        key={container.id}
                        onClick={() => setSelectedLogContainer(container.id)}
                        className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap ${
                          selectedLogContainer === container.id
                            ? 'text-primary-600 dark:text-primary-400 border-b-2 border-primary-600 dark:border-primary-400 bg-primary-600/10 dark:bg-primary-400/10'
                            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 bg-gray-100 dark:bg-gray-800'
                        }`}
                      >
                        {container.name}
                      </button>
                    ))}
                  </div>

                  {/* Log viewer */}
                  {selectedLogContainer && (() => {
                    const selectedContainer = containers.find(c => c.id === selectedLogContainer);
                    return selectedContainer ? (
                      <div className="border border-gray-200 dark:border-gray-800 rounded-lg overflow-hidden">
                        <LogViewer
                          hostId={hostId}
                          containerId={selectedLogContainer}
                          containerName={selectedContainer.name}
                          hostName={stack.host_name}
                          onClose={() => {}} // No-op since this is embedded in a modal
                          follow={true}
                          showHeader={false}
                        />
                      </div>
                    ) : null;
                  })()}
                </>
              )}
            </div>
          )}

          {activeTab === 'metrics' && (
            <StackMetricsPanel hostId={hostId} stackName={stack.name} containers={containers} />
          )}
        </div>
      </div>
    </div>
  );
};

// StackMetricsPanel component for aggregated container metrics
interface StackMetricsPanelProps {
  hostId: string;
  stackName: string;
  containers: Container[];
}

type TimeRange = "1h" | "6h" | "24h" | "7d";

const StackMetricsPanel: React.FC<StackMetricsPanelProps> = ({ hostId, stackName, containers }) => {
  const [timeRange, setTimeRange] = useState<TimeRange>("1h");
  const [interval, setInterval] = useState<string>("1m");

  // Calculate start and end times based on time range
  const { start, end } = getTimeRange(timeRange);

  // Fetch metrics for all containers in the stack
  const containerMetricsQueries = useQuery({
    queryKey: ['stack-metrics', hostId, stackName, timeRange, interval],
    queryFn: async () => {
      const metricsPromises = containers.map(container =>
        apiClient.getContainerMetrics(hostId, container.id, {
          start: start.toISOString(),
          end: end.toISOString(),
          interval,
        }).then(response => ({
          containerId: container.id,
          containerName: container.name,
          metrics: response.metrics
        }))
      );

      const results = await Promise.all(metricsPromises);
      return results.filter(result => result.metrics.length > 0);
    },
    enabled: containers.length > 0,
    refetchInterval: 30000, // Refetch every 30 seconds
  });

  const { data: containerMetrics = [], isLoading, error } = containerMetricsQueries;

  // Aggregate metrics across all containers
  const aggregatedMetrics = React.useMemo(() => {
    if (containerMetrics.length === 0) return [];

    // Group by timestamp and aggregate
    const timestampMap = new Map<string, {
      timestamp: string;
      cpu_percent: number;
      memory_usage: number;
      memory_limit: number;
      disk_read_bytes: number;
      disk_write_bytes: number;
      container_count: number;
    }>();

    containerMetrics.forEach(({ metrics }) => {
      metrics.forEach(metric => {
        const key = metric.timestamp ?? new Date().toISOString();
        if (!timestampMap.has(key)) {
          timestampMap.set(key, {
            timestamp: key,
            cpu_percent: 0,
            memory_usage: 0,
            memory_limit: 0,
            disk_read_bytes: 0,
            disk_write_bytes: 0,
            container_count: 0,
          });
        }

        const aggregated = timestampMap.get(key)!;
        aggregated.cpu_percent += metric.cpu_percent;
        aggregated.memory_usage += metric.memory_usage;
        aggregated.memory_limit += metric.memory_limit;
        aggregated.disk_read_bytes += metric.disk_read_bytes;
        aggregated.disk_write_bytes += metric.disk_write_bytes;
        aggregated.container_count += 1;
      });
    });

    // Convert to array and sort by timestamp
    return Array.from(timestampMap.values()).sort((a, b) =>
      new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    );
  }, [containerMetrics]);

  // Transform metrics for charts
  const cpuData: MetricDataPoint[] = aggregatedMetrics.map((m) => ({
    timestamp: m.timestamp,
    value: m.cpu_percent / m.container_count, // Average CPU across containers
  }));

  const memoryData: MetricDataPoint[] = aggregatedMetrics.map((m) => ({
    timestamp: m.timestamp,
    value: m.memory_usage,
  }));

  const memoryPercentData: MetricDataPoint[] = aggregatedMetrics.map((m) => ({
    timestamp: m.timestamp,
    value: m.memory_limit > 0 ? (m.memory_usage / m.memory_limit) * 100 : 0,
  }));

  const diskData: MetricDataPoint[] = aggregatedMetrics.map((m) => ({
    timestamp: m.timestamp,
    value: m.disk_read_bytes + m.disk_write_bytes,
  }));

  if (error) {
    return (
      <div className="p-4 border border-red-200 rounded-lg bg-red-50">
        <p className="text-red-600 text-sm">
          Failed to load stack metrics: {error instanceof Error ? error.message : "Unknown error"}
        </p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
      </div>
    );
  }

  if (aggregatedMetrics.length === 0) {
    return (
      <div className="p-8 border border-gray-200 rounded-lg bg-gray-50 text-center">
        <BarChart3 className="h-12 w-12 mx-auto text-gray-400 mb-4" />
        <h3 className="text-lg font-semibold text-gray-900 mb-2">No Metrics Data Available</h3>
        <p className="text-gray-600 text-sm mb-4">
          Metrics will appear here once containers start generating data.
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
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with time range selector */}
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-gray-900">Stack Metrics</h3>
        <div className="flex items-center gap-2">
          <select
            value={timeRange}
            onChange={(e) => {
              setTimeRange(e.target.value as "1h" | "6h" | "24h" | "7d");
              setInterval(getIntervalForRange(e.target.value as "1h" | "6h" | "24h" | "7d"));
            }}
            className="px-3 py-1 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
          >
            <option value="1h">Last Hour</option>
            <option value="6h">Last 6 Hours</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* CPU Usage (Average) */}
        <div className="p-4 border border-gray-200 rounded-lg bg-white">
          <MetricsChart
            data={cpuData}
            title="Average CPU Usage"
            unit="%"
            color="#3b82f6"
            height={250}
          />
        </div>

        {/* Memory Usage (Total) */}
        <div className="p-4 border border-gray-200 rounded-lg bg-white">
          <MetricsChart
            data={memoryData}
            title="Total Memory Usage"
            unit="bytes"
            color="#10b981"
            height={250}
          />
        </div>

        {/* Memory Percent (Total) */}
        <div className="p-4 border border-gray-200 rounded-lg bg-white">
          <MetricsChart
            data={memoryPercentData}
            title="Memory Usage (%)"
            unit="%"
            color="#8b5cf6"
            height={250}
          />
        </div>

        {/* Disk I/O (Total) */}
        <div className="p-4 border border-gray-200 rounded-lg bg-white">
          <MetricsChart
            data={diskData}
            title="Total Disk I/O"
            unit="bytes"
            color="#f59e0b"
            height={250}
          />
        </div>
      </div>
    </div>
  );
};

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

export default StackDetailModal;

