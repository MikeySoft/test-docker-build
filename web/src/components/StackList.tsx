import React from 'react';
import {
  Layers,
  Play,
  Square,
  RotateCw,
  Trash2,
  Edit,
  Plus,
  ZoomIn,
} from 'lucide-react';
import type { Stack } from '../types';
import StatusBadge from './StatusBadge';
import StackContainersCollapsible from './StackContainersCollapsible';

interface StackListProps {
  stacks: Stack[];
  onAction: (action: 'start' | 'stop' | 'restart', stackName: string, hostId: string) => void;
  onRemove: (stackName: string, hostId: string) => void;
  onEdit?: (stack: Stack) => void;
  onImport?: (stackName: string, hostId: string) => void;
  onViewDetails?: (stack: Stack, hostId: string) => void;
  onViewLogs?: (containerId: string, containerName: string, hostId: string) => void;
  showHost?: boolean;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
}

const StackList: React.FC<StackListProps> = ({
  stacks,
  onAction,
  onRemove,
  onEdit,
  onImport,
  onViewDetails,
  onViewLogs,
  showHost = false,
  isLoading = false,
  error = null,
  onRetry,
}) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500"></div>
        <span className="ml-2 text-gray-600 dark:text-gray-400 font-inter">Loading stacks...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-8">
        <p className="text-danger-500 mb-2 font-inter">Failed to load stacks</p>
        {onRetry && (
          <button onClick={onRetry} className="btn btn-secondary">
            Try Again
          </button>
        )}
      </div>
    );
  }

  if (stacks.length === 0) {
    return (
      <div className="text-center py-8">
        <Layers className="h-12 w-12 mx-auto text-gray-700 mb-4" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2 font-space">No stacks</h3>
        <p className="text-gray-600 dark:text-gray-400 font-inter">
          {showHost ? 'No Docker Compose stacks are deployed on any host.' : 'No Docker Compose stacks are deployed on this host.'}
        </p>
      </div>
    );
  }

  return (
    <div className="bg-white dark:bg-gray-950 border border-gray-200 dark:border-gray-900 rounded-xl overflow-hidden">
      <ul className="divide-y divide-gray-200 dark:divide-gray-900">
        {stacks.map((stack) => (
          <li key={`${stack.host_id}-${stack.name}`} className="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
            <div className="flex items-center justify-between">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-3">
                  <Layers className="h-5 w-5 text-gray-500 flex-shrink-0" />
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      {onViewDetails ? (
                        <button
                          onClick={() => onViewDetails(stack, stack.host_id)}
                          className="text-sm font-medium text-gray-900 dark:text-white truncate font-space hover:text-primary-600 dark:hover:text-primary-400 transition-colors text-left"
                        >
                          {stack.name}
                        </button>
                      ) : (
                        <p className="text-sm font-medium text-gray-900 dark:text-white truncate font-space">
                          {stack.name}
                        </p>
                      )}
                      {showHost && stack.host_name && (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-900 text-gray-600 dark:text-gray-400 border border-gray-300 dark:border-gray-800 font-inter">
                          {stack.host_name}
                        </span>
                      )}
                      {stack.compose_content ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200">
                          Flotilla Managed
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200">
                          External
                        </span>
                      )}
                    </div>
                    <div className="mt-1">
                      {onViewLogs ? (
                        <StackContainersCollapsible
                          hostId={stack.host_id}
                          stackName={stack.name}
                          hostName={stack.host_name}
                          onViewLogs={(containerId, containerName) =>
                            onViewLogs(containerId, containerName, stack.host_id)
                          }
                        />
                      ) : (
                        <p className="text-sm text-gray-600 dark:text-gray-500 font-inter">
                          Containers: {stack.containers ?? 0} ({stack.running ?? 0} running)
                        </p>
                      )}
                      {!stack.compose_content && (
                        <p className="text-xs text-yellow-600 dark:text-yellow-400 font-inter mt-1">
                          Import required to manage
                        </p>
                      )}
                    </div>
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-4">
                <StatusBadge status={stack.status} />
                <div className="flex items-center gap-2">
                  {stack.compose_content && onEdit && (
                    <button
                      onClick={() => onEdit(stack)}
                      className="text-secondary-500 hover:text-secondary-400 transition-colors"
                      title="Edit stack"
                    >
                      <Edit className="h-4 w-4" />
                    </button>
                  )}
                  {!stack.compose_content && onImport && (
                    <button
                      onClick={() => onImport(stack.name, stack.host_id)}
                      className="text-primary-500 hover:text-primary-400 transition-colors"
                      title="Import stack"
                    >
                      <Plus className="h-4 w-4" />
                    </button>
                  )}
                  {stack.compose_content && stack.status === 'stopped' && (
                    <button
                      onClick={() => onAction('start', stack.name, stack.host_id)}
                      className="text-success-500 hover:text-success-400 transition-colors"
                      title="Start stack"
                    >
                      <Play className="h-4 w-4" />
                    </button>
                  )}
                  {stack.compose_content && stack.status === 'running' && (
                    <>
                      <button
                        onClick={() => onAction('stop', stack.name, stack.host_id)}
                        className="text-warning-500 hover:text-warning-400 transition-colors"
                        title="Stop stack"
                      >
                        <Square className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => onAction('restart', stack.name, stack.host_id)}
                        className="text-info-500 hover:text-info-400 transition-colors"
                        title="Restart stack"
                      >
                        <RotateCw className="h-4 w-4" />
                      </button>
                    </>
                  )}
                  {onViewDetails && (
                    <button
                      onClick={() => onViewDetails(stack, stack.host_id)}
                      className="text-info-500 hover:text-info-400 transition-colors"
                      title="View stack details"
                    >
                      <ZoomIn className="h-4 w-4" />
                    </button>
                  )}
                  <button
                    onClick={() => onRemove(stack.name, stack.host_id)}
                    className="text-danger-500 hover:text-danger-400 transition-colors"
                    title="Remove stack"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
            <div className="mt-2 text-sm text-gray-600 dark:text-gray-500 font-inter">
              Created: {stack.created_at ? new Date(stack.created_at).toLocaleString() : 'Unknown'}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
};

export default StackList;

