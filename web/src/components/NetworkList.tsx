import React from "react";
import { Eye, Trash2 } from "lucide-react";
import type { DockerNetwork } from "../types";
import StatusBadge from "./StatusBadge";

interface NetworkListProps {
  networks: DockerNetwork[];
  isLoading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  onInspect?: (network: DockerNetwork) => void;
  onRemove?: (network: DockerNetwork) => void;
  isBusy?: boolean;
}

const NetworkList: React.FC<NetworkListProps> = ({
  networks,
  isLoading,
  error,
  onRetry,
  onInspect,
  onRemove,
  isBusy = false,
}) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500" />
        <span className="ml-2 text-gray-400 font-inter">Loading networks...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-8">
        <p className="text-danger-500 mb-2 font-inter">{error}</p>
        {onRetry && (
          <button onClick={onRetry} className="btn btn-secondary">
            Try Again
          </button>
        )}
      </div>
    );
  }

  if (!networks.length) {
    return (
      <div className="text-center py-12">
        <p className="text-lg font-semibold text-gray-900 dark:text-white font-space">No networks found</p>
        <p className="text-gray-600 dark:text-gray-400 font-inter">This host does not have any custom Docker networks yet.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {networks.map((network) => {
        const attachments = network.containers_detail ?? [];
        const containerCount = attachments.length || network.containers || 0;
        const stacks = network.stacks ?? [];
        const metadataPending = network.topology_metadata_pending ?? false;
        const refreshedAt = network.topology_refreshed_at ? new Date(network.topology_refreshed_at) : null;
        const isStale = network.topology_is_stale ?? false;

        return (
        <div key={network.id} className="card">
          <div className="flex flex-wrap items-center justify-between gap-3 mb-2">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">{network.name}</h3>
              <p className="text-sm text-gray-500 font-inter break-all">{network.id}</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2">
                <StatusBadge
                  status={
                    network.internal
                      ? "paused"
                      : containerCount > 0
                        ? "running"
                        : "stopped"
                  }
                />
                <span className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 font-inter">
                  {network.internal
                    ? "Internal"
                    : containerCount > 0
                      ? `${containerCount} attached`
                      : "Idle"}
                </span>
              </div>
              <div className="flex items-center gap-2">
                {onInspect && (
                  <button
                    type="button"
                    className="text-info-500 hover:text-info-400 transition-colors disabled:opacity-50"
                    onClick={() => onInspect(network)}
                    disabled={isBusy}
                    title="Inspect network details"
                    aria-label={`Inspect network ${network.name}`}
                  >
                    <Eye className="h-4 w-4" />
                    <span className="sr-only">Inspect network</span>
                  </button>
                )}
                {onRemove && (
                  <button
                    type="button"
                    className="text-danger-500 hover:text-danger-400 transition-colors disabled:opacity-50"
                    onClick={() => onRemove(network)}
                    disabled={isBusy}
                    title="Remove network"
                    aria-label={`Remove network ${network.name}`}
                  >
                    <Trash2 className="h-4 w-4" />
                    <span className="sr-only">Remove network</span>
                  </button>
                )}
              </div>
            </div>
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400 font-inter">
            {metadataPending ? (
              <span className="inline-flex items-center rounded bg-amber-100 dark:bg-amber-900/40 px-2 py-0.5 text-amber-800 dark:text-amber-200">
                Metadata pending
              </span>
            ) : refreshedAt ? (
              <span>Topology refreshed {refreshedAt.toLocaleString()}</span>
            ) : (
              <span>Topology metadata unavailable</span>
            )}
            {isStale && (
              <span className="inline-flex items-center rounded bg-red-100 dark:bg-red-900/40 px-2 py-0.5 text-red-800 dark:text-red-200">
                Stale
              </span>
            )}
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
            <div>
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">Driver</div>
              <div className="text-sm">{network.driver}</div>
            </div>
            <div>
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">Scope</div>
              <div className="text-sm capitalize">{network.scope ?? "local"}</div>
            </div>
            <div>
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">IPv6</div>
              <div className="text-sm">{network.enable_ipv6 ? "Enabled" : "Disabled"}</div>
            </div>
          </div>
          <div className="mt-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
            <div className="text-xs uppercase text-gray-500 dark:text-gray-400 mb-1">Labels</div>
            {network.labels && Object.keys(network.labels).length ? (
              <div className="flex flex-wrap gap-2">
                {Object.entries(network.labels).map(([key, value]) => (
                  <span key={key} className="inline-flex items-center rounded bg-gray-100 dark:bg-gray-900 px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300">
                    {key}={value}
                  </span>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">None</p>
            )}
          </div>
          <div className="mt-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
            <div className="text-xs uppercase text-gray-500 dark:text-gray-400 mb-1">Stacks</div>
            {stacks.length ? (
              <div className="flex flex-wrap gap-2">
                {stacks.map((stack) => (
                  <span key={stack} className="inline-flex items-center rounded bg-cyan-100 dark:bg-cyan-900/40 px-2 py-0.5 text-xs text-cyan-800 dark:text-cyan-200">
                    {stack}
                  </span>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">No associated stacks</p>
            )}
          </div>
          <div className="mt-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
            <div className="text-xs uppercase text-gray-500 dark:text-gray-400 mb-1">Containers</div>
            {attachments.length ? (
              <div className="flex flex-col gap-2">
                {attachments.map((attachment) => (
                  <div key={attachment.id} className="flex flex-wrap items-center gap-3">
                    <span className="text-sm text-gray-900 dark:text-gray-100 font-medium">{attachment.name ?? attachment.id}</span>
                    {attachment.stack && (
                      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded bg-cyan-100 dark:bg-cyan-900/40 text-cyan-800 dark:text-cyan-200">
                        {attachment.stack}
                      </span>
                    )}
                    {attachment.service && (
                      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-900 text-gray-700 dark:text-gray-300">
                        {attachment.service}
                      </span>
                    )}
                    {(attachment.ipv4 || attachment.ipv6) && (
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        {attachment.ipv4 || attachment.ipv6}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">No containers attached</p>
            )}
          </div>
        </div>
        );
      })}
    </div>
  );
};

export default NetworkList;

