import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Container as ContainerIcon,
  Play,
  Square,
  RotateCw,
  FileText,
  ChevronRight,
  ChevronDown,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useToast } from '../contexts/useToast';
import StatusBadge from './StatusBadge';
import type { Container } from '../types';

interface StackContainersCollapsibleProps {
  hostId: string;
  stackName: string;
  hostName?: string;
  onViewLogs: (containerId: string, containerName: string) => void;
}

const StackContainersCollapsible: React.FC<StackContainersCollapsibleProps> = ({
  hostId,
  stackName,
  hostName,
  onViewLogs,
}) => {
  const { showSuccess, showError } = useToast();
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);

  const {
    data: containers = [],
    isLoading,
    error,
  } = useQuery({
    queryKey: ['stack-containers-collapsible', hostId, stackName],
    queryFn: () => apiClient.getStackContainers(hostId, stackName),
    enabled: isExpanded,
    refetchInterval: isExpanded ? 5000 : false,
  });

  const getContainerState = (container: Container): 'running' | 'stopped' | 'error' => {
    const state = container.state?.toLowerCase();
    if (state === 'running') return 'running';
    if (state === 'stopped' || state === 'exited') return 'stopped';
    return 'error';
  };

  const handleContainerAction = async (
    containerId: string,
    action: 'start' | 'stop' | 'restart'
  ) => {
    setActionLoading(containerId);
    try {
      await apiClient.stackContainerAction(hostId, stackName, containerId, action);
      showSuccess(`Container ${action}ed successfully`);
    } catch (error: any) {
      showError(`Failed to ${action} container: ${error.message}`);
    } finally {
      setActionLoading(null);
    }
  };

  // Get container counts from props or query
  const containerCount = containers.length || 0;
  const runningCount = containers.filter(c => {
    const state = getContainerState(c);
    return state === 'running';
  }).length || 0;

  return (
    <div className="mt-2">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 dark:hover:text-gray-200 font-inter transition-colors"
      >
        {isExpanded ? (
          <ChevronDown className="h-4 w-4" />
        ) : (
          <ChevronRight className="h-4 w-4" />
        )}
        Containers: {containerCount} ({runningCount} running)
      </button>

      {isExpanded && (
        <div className="mt-2 ml-5 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-800">
          {isLoading ? (
            <div className="flex items-center justify-center py-4">
              <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary-600"></div>
              <span className="ml-2 text-sm text-gray-500 font-inter">Loading containers...</span>
            </div>
          ) : error ? (
            <div className="py-4 px-2">
              <p className="text-sm text-red-600 dark:text-red-400 font-inter">
                Failed to load containers
              </p>
            </div>
          ) : containers.length === 0 ? (
            <div className="py-4 px-2">
              <p className="text-sm text-gray-500 font-inter">No containers found</p>
            </div>
          ) : (
            <div className="divide-y divide-gray-200 dark:divide-gray-800">
              {containers.map((container) => {
                const state = getContainerState(container);
                return (
                  <div
                    key={container.id}
                    className="px-3 py-2 hover:bg-gray-100 dark:hover:bg-gray-900 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 flex-1 min-w-0">
                        <ContainerIcon className="h-4 w-4 text-gray-500 flex-shrink-0" />
                        <span className="text-sm font-medium text-gray-900 dark:text-white truncate font-space">
                          {container.name}
                        </span>
                        <StatusBadge status={state} />
                        {hostName && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-900 text-gray-600 dark:text-gray-400 border border-gray-300 dark:border-gray-800 font-inter">
                            {hostName}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-2 ml-2">
                        {state === 'stopped' ? (
                          <button
                            onClick={() => handleContainerAction(container.id, 'start')}
                            disabled={actionLoading === container.id}
                            className="btn btn-success text-sm px-2 py-1"
                            title="Start container"
                          >
                            <Play className="h-4 w-4" />
                          </button>
                        ) : (
                          <>
                            <button
                              onClick={() => handleContainerAction(container.id, 'stop')}
                              disabled={actionLoading === container.id}
                              className="btn btn-warning text-sm px-2 py-1"
                              title="Stop container"
                            >
                              <Square className="h-4 w-4" />
                            </button>
                            <button
                              onClick={() => handleContainerAction(container.id, 'restart')}
                              disabled={actionLoading === container.id}
                              className="btn btn-info text-sm px-2 py-1"
                              title="Restart container"
                            >
                              <RotateCw className="h-4 w-4" />
                            </button>
                          </>
                        )}
                        <button
                          onClick={() => onViewLogs(container.id, container.name)}
                          className="btn btn-secondary text-sm px-2 py-1"
                          title="View logs"
                        >
                          <FileText className="h-4 w-4" />
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default StackContainersCollapsible;

