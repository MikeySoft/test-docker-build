import React from "react";
import { Copy, Trash2 } from "lucide-react";
import type { DockerImage } from "../types";

interface ImageListProps {
  images: DockerImage[];
  isLoading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  selectedIds?: string[];
  onToggleSelect?: (imageId: string, selected: boolean) => void;
  onToggleSelectAll?: (selectAll: boolean) => void;
  onCopy?: (image: DockerImage) => void;
  onDelete?: (image: DockerImage) => void;
  isBusy?: boolean;
  isDeleteDisabled?: (image: DockerImage) => boolean;
  deleteDisabledReason?: (image: DockerImage) => string | undefined;
}

const formatBytes = (bytes?: number): string => {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, exponent);
  return `${value.toFixed(value >= 10 || exponent === 0 ? 0 : 1)} ${units[exponent]}`;
};

const formatDate = (timestamp?: number): string => {
  if (!timestamp) return "Unknown";
  const date = new Date(timestamp * 1000);
  return date.toLocaleString();
};

const ImageList: React.FC<ImageListProps> = ({
  images,
  isLoading,
  error,
  onRetry,
  selectedIds = [],
  onToggleSelect,
  onToggleSelectAll,
  onCopy,
  onDelete,
  isBusy,
  isDeleteDisabled,
  deleteDisabledReason,
}) => {
  const selectedSet = React.useMemo(() => new Set(selectedIds), [selectedIds]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500" />
        <span className="ml-2 text-gray-400 font-inter">Loading images...</span>
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

  if (!images.length) {
    return (
      <div className="text-center py-12">
        <p className="text-lg font-semibold text-gray-900 dark:text-white font-space">No images found</p>
        <p className="text-gray-600 dark:text-gray-400 font-inter">This host does not have any Docker images yet.</p>
      </div>
    );
  }

  const allSelected = images.length > 0 && images.every((img) => selectedSet.has(img.id));
  const indeterminate = selectedSet.size > 0 && !allSelected;

  return (
    <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
      <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-800">
        <thead className="bg-gray-50 dark:bg-gray-900/40">
          <tr>
            <th className="px-3 py-3">
              <input
                type="checkbox"
                className="rounded border-gray-300 text-cyan-600 focus:ring-cyan-500"
                checked={allSelected}
                ref={(input) => {
                  if (input) input.indeterminate = indeterminate;
                }}
                onChange={(e) => onToggleSelectAll?.(e.target.checked)}
                disabled={isBusy}
                aria-label="Select all images"
              />
            </th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Repository</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Image ID</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Size</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Created</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Containers</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
          </tr>
        </thead>
        <tbody className="bg-white dark:bg-gray-950 divide-y divide-gray-200 dark:divide-gray-900">
          {images.map((image) => {
            const tags = image.tags && image.tags.length ? image.tags : ["<none>:<none>"];
            const isSelected = selectedSet.has(image.id);
            const deleteDisabled = isDeleteDisabled?.(image) ?? false;
            const disabledReason = deleteDisabledReason?.(image);

            return (
              <tr key={image.id}>
                <td className="px-3 py-4">
                  <input
                    type="checkbox"
                    className="rounded border-gray-300 text-cyan-600 focus:ring-cyan-500"
                    checked={isSelected}
                    onChange={(e) => onToggleSelect?.(image.id, e.target.checked)}
                    disabled={isBusy}
                    aria-label={`Select image ${image.tag ?? image.image ?? image.id}`}
                  />
                </td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100 font-inter">
                  <div className="flex flex-col gap-1">
                    {tags.map((tag) => (
                      <span
                        key={`${image.id}-${tag}`}
                        className="inline-flex items-center rounded bg-gray-100 dark:bg-gray-900 px-2 py-0.5 text-xs text-gray-700 dark:text-gray-300"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-4 text-sm text-gray-700 dark:text-gray-300 capitalize">
                  {image.status ?? (image.dangling ? "dangling" : "active")}
                </td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100 font-mono">
                  {image.short_id ?? image.id.slice(7, 19)}
                </td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div>{formatBytes(image.size)}</div>
                  {image.shared_size ? (
                    <div className="text-xs text-gray-500 dark:text-gray-400">Shared: {formatBytes(image.shared_size)}</div>
                  ) : null}
                </td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">{formatDate(image.created)}</td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  {image.containers !== undefined ? image.containers : "—"}
                </td>
                <td className="px-4 py-4 text-sm text-gray-900 dark:text-gray-100">
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      className="text-info-500 hover:text-info-400 transition-colors disabled:opacity-50"
                      onClick={() => onCopy?.(image)}
                      disabled={isBusy}
                      title="Copy registry URL"
                      aria-label="Copy registry URL"
                    >
                      <Copy className="h-4 w-4" />
                      <span className="sr-only">Copy registry URL</span>
                    </button>
                    <button
                      type="button"
                      className="text-danger-500 hover:text-danger-400 transition-colors disabled:opacity-50"
                      onClick={() => onDelete?.(image)}
                      disabled={isBusy || deleteDisabled}
                      title={disabledReason ?? "Delete image"}
                      aria-label="Delete image"
                    >
                      <Trash2 className="h-4 w-4" />
                      <span className="sr-only">Delete image</span>
                    </button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

export default ImageList;

