import React from 'react';
import {
  Container as ContainerIcon,
  Play,
  Square,
  RotateCcw,
  Trash2,
  FileText,
} from 'lucide-react';
// apiClient not needed here; left in case of future list actions
import type { Container } from '../types';
import StatusBadge from './StatusBadge';
import ContainerMiniMetrics from './ContainerMiniMetrics';

interface ContainerListProps {
  containers: Container[];
  onAction: (action: 'start' | 'stop' | 'restart', containerId: string, hostId?: string) => void;
  onRemove: (containerId: string, hostId?: string) => void;
  onViewLogs?: (containerId: string, hostId?: string, containerName?: string, hostName?: string) => void;
  hostId?: string;
  showHost?: boolean;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
}

// Component for real-time metrics badges
// removed inline metrics badge in favor of ContainerMiniMetrics

const ContainerList: React.FC<ContainerListProps> = ({
  containers,
  onAction,
  onRemove,
  onViewLogs,
  hostId,
  showHost = false,
  isLoading = false,
  error = null,
  onRetry,
}) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500"></div>
        <span className="ml-2 text-gray-600 dark:text-gray-400 font-inter">Loading containers...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-8">
        <p className="text-danger-500 mb-2 font-inter">Failed to load containers</p>
        {onRetry && (
          <button onClick={onRetry} className="btn btn-secondary">
            Try Again
          </button>
        )}
      </div>
    );
  }

  if (containers.length === 0) {
    return (
      <div className="text-center py-8">
        <ContainerIcon className="h-12 w-12 mx-auto text-gray-700 mb-4" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2 font-space">No containers</h3>
        <p className="text-gray-600 dark:text-gray-400 font-inter">
          {showHost ? 'No containers are running on any host.' : 'No containers are running on this host.'}
        </p>
      </div>
    );
  }

  return (
    <div className="bg-white dark:bg-gray-950 border border-gray-200 dark:border-gray-900 rounded-xl overflow-hidden">
      <ul className="divide-y divide-gray-200 dark:divide-gray-900">
        {containers.map((container) => (
          <li key={container.id} className="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
            <div className="flex items-center justify-between">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-3">
                  <ContainerIcon className="h-5 w-5 text-gray-500 flex-shrink-0" />
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium text-gray-900 dark:text-white truncate font-space">
                        {container.name}
                      </p>
                      <StatusBadge status={container.status} />
                      {showHost && container.host_name && (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-900 text-gray-600 dark:text-gray-400 border border-gray-300 dark:border-gray-800 font-inter">
                          {container.host_name}
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-gray-600 dark:text-gray-500 font-inter">{container.image}</p>
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-4">
                { (container.host_id ?? hostId) && (
                  <ContainerMiniMetrics
                    hostId={container.host_id ?? hostId!}
                    containerId={container.id}
                    isRunning={container.status === 'running'}
                    containerName={container.name}
                  />
                )}
                <div className="flex items-center gap-2">
                  {onViewLogs && (
                    <button
                      onClick={() => onViewLogs(container.id, container.host_id, container.name, container.host_name)}
                      className="text-cyan-500 hover:text-cyan-400 transition-colors"
                      title="View logs"
                    >
                      <FileText className="h-4 w-4" />
                    </button>
                  )}
                  {container.status === 'stopped' && (
                    <button
                      onClick={() => onAction('start', container.id, container.host_id)}
                      className="text-success-500 hover:text-success-400 transition-colors"
                      title="Start container"
                    >
                      <Play className="h-4 w-4" />
                    </button>
                  )}
                  {container.status === 'running' && (
                    <button
                      onClick={() => onAction('stop', container.id, container.host_id)}
                      className="text-warning-500 hover:text-warning-400 transition-colors"
                      title="Stop container"
                    >
                      <Square className="h-4 w-4" />
                    </button>
                  )}
                  <button
                    onClick={() => onAction('restart', container.id, container.host_id)}
                    className="text-info-500 hover:text-info-400 transition-colors"
                    title="Restart container"
                  >
                    <RotateCcw className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => onRemove(container.id, container.host_id)}
                    className="text-danger-500 hover:text-danger-400 transition-colors"
                    title="Remove container"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
            <div className="mt-2 text-sm text-gray-600 dark:text-gray-500 font-inter">
              Created: {new Date(container.created).toLocaleString()}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
};

export default ContainerList;
