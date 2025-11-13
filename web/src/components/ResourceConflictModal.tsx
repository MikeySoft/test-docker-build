import React from "react";
import { AlertTriangle, RefreshCw, ShieldAlert } from "lucide-react";
import Modal from "./Modal";
import type {
  ResourceRemovalBlocker,
  ResourceRemovalConflict,
  ResourceRemovalError,
  ResourceRemovalType,
} from "../types";

type BlockerAction = {
  key: string;
  label: string;
  intent?: "primary" | "secondary" | "danger";
  perform: () => Promise<void> | void;
};

interface ResourceConflictModalProps {
  isOpen: boolean;
  onClose: () => void;
  resourceType: ResourceRemovalType;
  resourceIds: string[];
  resourceNames?: string[];
  conflicts: ResourceRemovalConflict[];
  errors?: ResourceRemovalError[];
  isProcessing?: boolean;
  processingActionKey?: string;
  onRetry?: () => void;
  onForce?: () => void;
  blockerActions?: (blocker: ResourceRemovalBlocker) => BlockerAction[];
}

const resourceLabels: Record<ResourceRemovalType, { singular: string; plural: string }> = {
  image: { singular: "image", plural: "images" },
  volume: { singular: "volume", plural: "volumes" },
  network: { singular: "network", plural: "networks" },
};

const ResourceConflictModal: React.FC<ResourceConflictModalProps> = ({
  isOpen,
  onClose,
  resourceType,
  resourceIds,
  resourceNames,
  conflicts,
  errors,
  isProcessing = false,
  processingActionKey,
  onRetry,
  onForce,
  blockerActions,
}) => {
  const [actionLoadingKey, setActionLoadingKey] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!isOpen) {
      setActionLoadingKey(null);
    }
  }, [isOpen]);

  const labels = resourceLabels[resourceType];
  const canForce = React.useMemo(
    () => conflicts.some((conflict) => conflict.force_supported),
    [conflicts]
  );

  const isAnyActionBusy = isProcessing || actionLoadingKey !== null;
  const retryLoading = isProcessing && (!processingActionKey || processingActionKey === "retry");
  const forceLoading = isProcessing && (!processingActionKey || processingActionKey === "force");

  const handleBlockerAction = async (action: BlockerAction) => {
    try {
      const result = action.perform();
      if (result && typeof (result as Promise<unknown>).then === "function") {
        setActionLoadingKey(action.key);
        await result;
      }
    } finally {
      setActionLoadingKey(null);
    }
  };

  const renderBlockerDetails = (blocker: ResourceRemovalBlocker) => {
    const detailEntries = Object.entries(blocker.details ?? {});
    if (detailEntries.length === 0) {
      return null;
    }
    return (
      <ul className="text-xs text-gray-600 dark:text-gray-400 leading-relaxed mt-1">
        {detailEntries.map(([key, value]) => (
          <li key={`${blocker.kind}-${key}`}>
            <span className="font-medium">{key}:</span> {value}
          </li>
        ))}
      </ul>
    );
  };

  const renderBlockers = (conflict: ResourceRemovalConflict) => {
    if (!conflict.blockers || conflict.blockers.length === 0) {
      return (
        <p className="text-sm text-gray-600 dark:text-gray-300">
          No additional reference data was provided by the agent.
        </p>
      );
    }

    return conflict.blockers.map((blocker) => {
      const actions = blockerActions?.(blocker) ?? [];
      const actionDisabled = isAnyActionBusy;

      return (
        <div
          key={`${blocker.kind}-${blocker.id ?? blocker.name ?? Math.random()}`}
          className="rounded-md border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900/40 p-3"
        >
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-gray-900 dark:text-white">
                {blocker.name ?? blocker.id ?? blocker.kind}
              </div>
              <div className="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {blocker.kind.replace(/_/g, " ")}
              </div>
              {blocker.stack && (
                <div className="mt-1 inline-flex items-center rounded bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200 px-2 py-0.5 text-xs">
                  Stack: {blocker.stack}
                </div>
              )}
              {renderBlockerDetails(blocker)}
            </div>
            {actions.length > 0 && (
              <div className="flex flex-col gap-2">
                {actions.map((action) => {
                  const loading = actionLoadingKey === action.key || processingActionKey === action.key;
                  const intentClass =
                    action.intent === "danger"
                      ? "btn btn-danger"
                      : action.intent === "secondary"
                        ? "btn btn-secondary"
                        : "btn btn-primary";
                  return (
                    <button
                      key={action.key}
                      type="button"
                      className={`${intentClass} text-xs`}
                      disabled={actionDisabled}
                      onClick={() => handleBlockerAction(action)}
                    >
                      {loading ? (
                        <span className="flex items-center gap-2">
                          <RefreshCw className="h-3 w-3 animate-spin" />
                          Working…
                        </span>
                      ) : (
                        action.label
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      );
    });
  };

  const renderConflicts = () => {
    if (conflicts.length === 0) {
      return (
        <div className="rounded-md border border-yellow-300 bg-yellow-50 dark:border-yellow-700 dark:bg-yellow-900/30 p-4">
          <div className="flex items-start gap-3">
            <ShieldAlert className="h-5 w-5 text-yellow-600 dark:text-yellow-400 mt-0.5" />
            <div>
              <h4 className="font-semibold text-yellow-800 dark:text-yellow-200">
                No detailed conflicts provided
              </h4>
              <p className="text-sm text-yellow-700 dark:text-yellow-300">
                Docker reported a conflict but the agent did not return additional reference details.
                Try retrying the removal or forcing the operation if you are confident it is safe.
              </p>
            </div>
          </div>
        </div>
      );
    }

    return conflicts.map((conflict) => (
      <div
        key={`${conflict.resource_id ?? conflict.resource_name ?? conflict.reason}`}
        className="rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 p-4 space-y-3"
      >
        <div className="flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-amber-500 mt-0.5" />
          <div>
            <h4 className="text-base font-semibold text-gray-900 dark:text-white">
              {conflict.reason}
            </h4>
            {conflict.original_error && (
              <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
                Docker response: {conflict.original_error}
              </p>
            )}
          </div>
        </div>
        <div className="space-y-3">{renderBlockers(conflict)}</div>
      </div>
    ));
  };

  const renderErrors = () => {
    if (!errors || errors.length === 0) {
      return null;
    }
    return (
      <div className="rounded-md border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/30 p-4">
        <div className="flex items-start gap-3">
          <ShieldAlert className="h-5 w-5 text-red-500 mt-0.5" />
          <div>
            <h4 className="font-semibold text-red-700 dark:text-red-300">
              Additional errors encountered
            </h4>
            <ul className="mt-2 list-disc pl-5 text-sm text-red-700 dark:text-red-300 space-y-1">
              {errors.map((error, idx) => (
                <li key={`${error.message}-${idx}`}>
                  {error.message}
                  {error.resource_name && ` (${error.resource_name})`}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    );
  };

  const primaryLabel = `Resolve ${labels.singular} removal`;
  const targetSummary =
    resourceNames && resourceNames.length > 0
      ? resourceNames.join(", ")
      : resourceIds.join(", ");

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={primaryLabel} size="xl">
      <div className="p-6 space-y-6">
        <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/30 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="h-5 w-5 text-amber-500 mt-0.5" />
            <div>
              <h3 className="text-base font-semibold text-amber-800 dark:text-amber-200">
                {labels.singular.charAt(0).toUpperCase() + labels.singular.slice(1)} removal blocked
              </h3>
              <p className="text-sm text-amber-700 dark:text-amber-100 mt-1">
                Docker reported that the {labels.singular} could not be removed because it is still referenced by other resources.
                Review the blockers below and resolve them, or force the removal if you are certain it is safe.
              </p>
              {targetSummary && (
                <p className="mt-2 text-sm font-medium text-amber-800 dark:text-amber-200">
                  Target: {targetSummary}
                </p>
              )}
            </div>
          </div>
        </div>

        <div className="space-y-4">{renderConflicts()}</div>

        {renderErrors()}

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-sm text-gray-600 dark:text-gray-400">
            {canForce
              ? "Force removal detaches the resource even if references still exist. Containers depending on it may fail afterwards."
              : "Docker does not support forcing removal for this resource type. Resolve the blockers above before retrying."}
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onClose}
              disabled={isAnyActionBusy}
            >
              Close
            </button>
            {onRetry && (
              <button
                type="button"
                className="btn btn-primary"
                onClick={onRetry}
                disabled={isAnyActionBusy}
              >
                {retryLoading ? (
                  <span className="flex items-center gap-2">
                    <RefreshCw className="h-4 w-4 animate-spin" />
                    Retrying…
                  </span>
                ) : (
                  "Retry"
                )}
              </button>
            )}
            {onForce && canForce && (
              <button
                type="button"
                className="btn btn-danger"
                onClick={onForce}
                disabled={isAnyActionBusy}
              >
                {forceLoading ? (
                  <span className="flex items-center gap-2">
                    <RefreshCw className="h-4 w-4 animate-spin" />
                    Forcing…
                  </span>
                ) : (
                  "Force remove"
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default ResourceConflictModal;

