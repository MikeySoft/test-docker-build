import React from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Server, HardDrive } from 'lucide-react';
import { apiClient } from '../api/client';
import QueryFilterInput from '../components/QueryFilterInput';
import StatusBadge from '../components/StatusBadge';
import HostMiniMetrics from '../components/HostMiniMetrics';
import HostDockerVersion from '../components/HostDockerVersion';

const HostList: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const q = searchParams.get('q') || '';
  const [filterDraft, setFilterDraft] = React.useState(q);

  const {
    data: hosts = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['hosts', q],
    queryFn: () => apiClient.getHosts(q || undefined),
    refetchInterval: 30000, // Refetch every 30 seconds
    staleTime: 0,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div>
        <span className="ml-2 text-gray-600 dark:text-gray-400 font-inter">Loading hosts...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="text-danger-500 mb-4">
          <Server className="h-12 w-12 mx-auto mb-2" />
          <h3 className="text-lg font-semibold font-space">Failed to load hosts</h3>
          <p className="text-sm text-gray-600 dark:text-gray-400 mt-1 font-inter">{error.message}</p>
        </div>
        <button
          onClick={() => refetch()}
          className="btn btn-primary"
        >
          Try Again
        </button>
      </div>
    );
  }

  if (hosts.length === 0) {
    return (
      <div className="text-center py-12">
        <Server className="h-12 w-12 mx-auto text-gray-700 mb-4" />
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2 font-space">No hosts found</h3>
        <p className="text-gray-600 dark:text-gray-400 font-inter">No Docker hosts are currently connected to flotilla.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-space">Docker Hosts</h1>
          <p className="text-gray-600 dark:text-gray-400 mt-1 font-inter">
            Manage your Docker containers and stacks across multiple hosts
          </p>
        </div>
      </div>

      {/* Filter block above list */}
      <div className="mb-4 w-full">
        <div className="flex items-center gap-2 w-full">
          <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
          <div className="flex-1">
            <QueryFilterInput
              value={filterDraft}
              onChange={setFilterDraft}
              onSubmit={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraft) { p.set('q', filterDraft); } else { p.delete('q'); } return p; })}
              placeholder="name:prod status=online"
              statuses={["online","offline","error"]}
            />
          </div>
          <button
            onClick={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraft) { p.set('q', filterDraft); } else { p.delete('q'); } return p; })}
            className="btn btn-secondary"
          >
            Filter
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {hosts.map((host) => (
          <Link
            key={host.id}
            to={`/hosts/${host.id}`}
            className="card-interactive"
          >
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-2">
                  <Server className="h-5 w-5 text-gray-500" />
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate font-space">
                    {host.name}
                  </h3>
                </div>
                {host.description && (
                  <p className="text-sm text-gray-600 dark:text-gray-400 mb-3 font-inter">{host.description}</p>
                )}
                <div className="flex items-center gap-4 text-sm text-gray-600 dark:text-gray-500 font-inter">
                  <div className="flex items-center gap-1">
                    <HardDrive className="h-4 w-4" />
                    <span>Agent</span>
                  </div>
                  {host.agent_version && (
                    <span>v{host.agent_version}</span>
                  )}
                  <HostDockerVersion hostId={host.id} />
                </div>
              </div>
              <StatusBadge status={host.status} />
            </div>

            <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-900">
              <div className="flex items-center justify-between text-sm text-gray-600 dark:text-gray-500 font-inter">
                <span>Last seen</span>
                <span>
                  {host.last_seen
                    ? new Date(host.last_seen).toLocaleString()
                    : 'Never'
                  }
                </span>
              </div>
              {/* Mini metrics */}
              <HostMiniMetrics hostId={host.id} />
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
};

export default HostList;
