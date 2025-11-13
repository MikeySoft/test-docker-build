import React from "react";
import { Copy, Info, Trash2 } from "lucide-react";
import type { DockerVolume } from "../types";

interface VolumeListProps {
  volumes: DockerVolume[];
  isLoading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  onInspect?: (volume: DockerVolume) => void;
  onCopyPath?: (volume: DockerVolume) => void;
  onRemove?: (volume: DockerVolume) => void;
  isBusy?: boolean;
}

const formatBytes = (bytes?: number): string => {
  if (bytes === undefined || bytes === null) return "Unknown";
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, exponent);
  return `${value.toFixed(value >= 10 || exponent === 0 ? 0 : 1)} ${units[exponent]}`;
};

const VolumeList: React.FC<VolumeListProps> = ({
  volumes,
  isLoading,
  error,
  onRetry,
  onInspect,
  onCopyPath,
  onRemove,
  isBusy = false,
}) => {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500" />
        <span className="ml-2 text-gray-400 font-inter">Loading volumes...</span>
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

  if (!volumes.length) {
    return (
      <div className="text-center py-12">
        <p className="text-lg font-semibold text-gray-900 dark:text-white font-space">No volumes found</p>
        <p className="text-gray-600 dark:text-gray-400 font-inter">This host has no Docker volumes yet.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {volumes.map((volume) => {
        const consumers = volume.containers_detail ?? [];
        const stacks = volume.stacks ?? [];
        const refCount = volume.containers ?? volume.ref_count ?? consumers.length;
        const metadataPending = volume.topology_metadata_pending ?? false;
        const refreshedAt = volume.topology_refreshed_at ? new Date(volume.topology_refreshed_at) : null;
        const isStale = volume.topology_is_stale ?? false;

        return (
        <div key={volume.name} className="card">
          <div className="flex flex-wrap items-center justify-between gap-3 mb-2">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">{volume.name}</h3>
              <p className="text-sm text-gray-500 font-inter">{volume.driver}</p>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-sm text-gray-600 dark:text-gray-300 font-inter">
                <span className="font-medium">Refs:</span> {refCount}
              </span>
              <div className="flex items-center gap-2">
                {onInspect && (
                  <button
                    type="button"
                    className="text-info-500 hover:text-info-400 transition-colors disabled:opacity-50"
                    onClick={() => onInspect(volume)}
                    disabled={isBusy}
                    title="Inspect volume details"
                    aria-label={`Inspect volume ${volume.name}`}
                  >
                    <Info className="h-4 w-4" />
                    <span className="sr-only">Inspect volume</span>
                  </button>
                )}
                {onCopyPath && (
                  <button
                    type="button"
                    className="text-cyan-500 hover:text-cyan-400 transition-colors disabled:opacity-50"
                    onClick={() => onCopyPath(volume)}
                    disabled={isBusy}
                    title="Copy mountpoint path"
                    aria-label={`Copy mountpoint for volume ${volume.name}`}
                  >
                    <Copy className="h-4 w-4" />
                    <span className="sr-only">Copy mountpoint</span>
                  </button>
                )}
                {onRemove && (
                  <button
                    type="button"
                    className="text-danger-500 hover:text-danger-400 transition-colors disabled:opacity-50"
                    onClick={() => onRemove(volume)}
                    disabled={isBusy}
                    title="Remove volume"
                    aria-label={`Remove volume ${volume.name}`}
                  >
                    <Trash2 className="h-4 w-4" />
                    <span className="sr-only">Remove volume</span>
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
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">Mountpoint</div>
              <div className="text-sm break-all">{volume.mountpoint}</div>
            </div>
            <div>
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">Scope</div>
              <div className="text-sm capitalize">{volume.scope ?? "local"}</div>
            </div>
            <div>
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400">Size</div>
              <div className="text-sm">{formatBytes(volume.size_bytes)}</div>
            </div>
          </div>
          <div className="mt-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
            <div className="text-xs uppercase text-gray-500 dark:text-gray-400 mb-1">Labels</div>
            {volume.labels && Object.keys(volume.labels).length ? (
              <div className="flex flex-wrap gap-2">
                {Object.entries(volume.labels).map(([key, value]) => (
                  <span key={key} className="inline-flex items-center rounded bg-gray-100 dark:bg-gray-900 px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300">
                    {key}={value}
                  </span>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">None</p>
            )}
          </div>
          {volume.options && Object.keys(volume.options).length ? (
            <div className="mt-4 text-sm text-gray-700 dark:text-gray-300 font-inter">
              <div className="text-xs uppercase text-gray-500 dark:text-gray-400 mb-1">Options</div>
              <div className="flex flex-wrap gap-2">
                {Object.entries(volume.options).map(([key, value]) => (
                  <span key={key} className="inline-flex items-center rounded bg-cyan-100 dark:bg-cyan-900/40 px-2 py-0.5 text-xs text-cyan-800 dark:text-cyan-200">
                    {key}={value}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
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
            {consumers.length ? (
              <div className="flex flex-col gap-2">
                {consumers.map((consumer) => (
                  <div key={`${consumer.id}-${consumer.destination ?? ""}`} className="flex flex-wrap items-center gap-3">
                    <span className="text-sm text-gray-900 dark:text-gray-100 font-medium">
                      {consumer.name ?? consumer.id}
                    </span>
                    {consumer.stack && (
                      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded bg-cyan-100 dark:bg-cyan-900/40 text-cyan-800 dark:text-cyan-200">
                        {consumer.stack}
                      </span>
                    )}
                    {consumer.service && (
                      <span className="inline-flex items-center px-2 py-0.5 text-xs rounded bg-gray-100 dark:bg-gray-900 text-gray-700 dark:text-gray-300">
                        {consumer.service}
                      </span>
                    )}
                    {consumer.destination && (
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        ↳ {consumer.destination}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">No containers using this volume</p>
            )}
          </div>
        </div>
        );
      })}
    </div>
  );
};

export default VolumeList;

