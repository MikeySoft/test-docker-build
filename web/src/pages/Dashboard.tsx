import React, { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Info, Loader2, Plus, XCircle } from "lucide-react";
import apiClient from "../api/client";
import Modal from "../components/Modal";
import { useToast } from "../contexts/useToast";
import type {
  DashboardSummary,
  DashboardTask,
  DashboardTaskListResponse,
  DashboardTaskSeverity,
  DashboardTaskStatus,
  DashboardTaskSource,
  DashboardTaskQueryParams,
  CreateDashboardTaskPayload,
} from "../types";

type TaskFilters = Pick<DashboardTaskQueryParams, "status" | "severity" | "source">;

const severityStyles: Record<DashboardTaskSeverity, string> = {
  info: "bg-sky-100 text-sky-800",
  warning: "bg-amber-100 text-amber-800",
  critical: "bg-rose-100 text-rose-800",
};

const statusStyles: Record<DashboardTaskStatus, string> = {
  open: "bg-gray-100 text-gray-800",
  acknowledged: "bg-blue-100 text-blue-800",
  resolved: "bg-emerald-100 text-emerald-800",
  dismissed: "bg-slate-100 text-slate-600",
};

const sourceStyles: Record<DashboardTaskSource, string> = {
  system: "bg-purple-100 text-purple-800",
  manual: "bg-teal-100 text-teal-800",
};

const severityIcon: Record<DashboardTaskSeverity, React.ReactElement> = {
  info: <Info className="h-4 w-4" />,
  warning: <AlertTriangle className="h-4 w-4" />,
  critical: <XCircle className="h-4 w-4" />,
};

const statusOptions: { label: string; value?: DashboardTaskStatus }[] = [
  { label: "All" },
  { label: "Open", value: "open" },
  { label: "Acknowledged", value: "acknowledged" },
  { label: "Resolved", value: "resolved" },
  { label: "Dismissed", value: "dismissed" },
];

const severityOptions: { label: string; value?: DashboardTaskSeverity }[] = [
  { label: "All" },
  { label: "Info", value: "info" },
  { label: "Warning", value: "warning" },
  { label: "Critical", value: "critical" },
];

const sourceOptions: { label: string; value?: DashboardTaskSource }[] = [
  { label: "All" },
  { label: "System", value: "system" },
  { label: "Manual", value: "manual" },
];

interface NewTaskFormState {
  title: string;
  description: string;
  severity: DashboardTaskSeverity;
  category: string;
  taskType: string;
  hostId: string;
  stackId: string;
  dueAt: string;
  snoozedUntil: string;
}

const initialFormState: NewTaskFormState = {
  title: "",
  description: "",
  severity: "info",
  category: "",
  taskType: "",
  hostId: "",
  stackId: "",
  dueAt: "",
  snoozedUntil: "",
};

const Dashboard: React.FC = () => {
  const queryClient = useQueryClient();
  const { showSuccess, showError } = useToast();
  const [filters, setFilters] = useState<TaskFilters>({ status: "open" });
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [formState, setFormState] = useState<NewTaskFormState>(initialFormState);

  const summaryQuery = useQuery<DashboardSummary, Error>({
    queryKey: ["dashboard", "summary"],
    queryFn: (): Promise<DashboardSummary> => apiClient.getDashboardSummary(),
    refetchInterval: 30000,
  });

  const tasksQuery = useQuery<DashboardTaskListResponse, Error>({
    queryKey: ["dashboard", "tasks", filters],
    queryFn: () =>
      apiClient.listDashboardTasks({
        status: filters.status,
        severity: filters.severity,
        source: filters.source,
        limit: 100,
      }),
    refetchInterval: 30000,
  });

  const hostsQuery = useQuery({
    queryKey: ["dashboard", "hosts"],
    queryFn: () => apiClient.getHosts(),
    staleTime: 5 * 60 * 1000,
  });

  const createTaskMutation = useMutation<DashboardTask, Error, CreateDashboardTaskPayload>({
    mutationFn: (payload: CreateDashboardTaskPayload) => apiClient.createDashboardTask(payload),
    onSuccess: () => {
      showSuccess("Task created");
      setIsCreateModalOpen(false);
      setFormState(initialFormState);
      queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks"] });
    },
    onError: (error: unknown) => {
      showError(error instanceof Error ? error.message : "Failed to create task");
    },
  });

  const updateStatusMutation = useMutation<DashboardTask, Error, { id: string; status: DashboardTaskStatus }>({
    mutationFn: ({ id, status }: { id: string; status: DashboardTaskStatus }) =>
      apiClient.updateDashboardTaskStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks"] });
      showSuccess("Task updated");
    },
    onError: (error: unknown) => {
      showError(error instanceof Error ? error.message : "Failed to update task");
    },
  });

  const summaryCards = useMemo(() => {
    const summary = summaryQuery.data;
    if (!summary) {
      return [];
    }
    return [
      {
        title: "Hosts Online",
        value: summary.hosts_online,
        subvalue: summary.hosts_total,
        highlight: "text-emerald-600",
      },
      {
        title: "Hosts Offline",
        value: summary.hosts_offline,
        highlight: "text-rose-600",
      },
      {
        title: "Containers",
        value: summary.containers_total,
      },
      {
        title: "Stacks",
        value: summary.stacks_total,
      },
    ];
  }, [summaryQuery.data]);

  const handleFilterClick = (type: keyof TaskFilters, value?: string) => {
    setFilters((prev) => ({
      ...prev,
      [type]: value,
    }));
  };

  const handleCreateTask = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const payload: CreateDashboardTaskPayload = {
      title: formState.title.trim(),
      description: formState.description.trim() || undefined,
      severity: formState.severity,
      category: formState.category.trim() || undefined,
      task_type: formState.taskType.trim() || undefined,
      host_id: formState.hostId || undefined,
      stack_id: formState.stackId || undefined,
      due_at: formState.dueAt ? new Date(formState.dueAt).toISOString() : undefined,
      snoozed_until: formState.snoozedUntil ? new Date(formState.snoozedUntil).toISOString() : undefined,
    };
    createTaskMutation.mutate(payload);
  };

  const tasks: DashboardTask[] = tasksQuery.data?.tasks ?? [];

  const renderSeverityBadge = (task: DashboardTask) => (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${severityStyles[task.severity]}`}>
      {severityIcon[task.severity]}
      {task.severity.toUpperCase()}
    </span>
  );

  const renderStatusBadge = (task: DashboardTask) => (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${statusStyles[task.status]}`}>
      {task.status.toUpperCase()}
    </span>
  );

  const renderSourceBadge = (task: DashboardTask) => (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${sourceStyles[task.source]}`}>
      {task.source === "system" ? "System" : "Manual"}
    </span>
  );

  const busy = summaryQuery.isLoading || tasksQuery.isLoading;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-space">Fleet Dashboard</h1>
          <p className="text-sm text-gray-600 dark:text-gray-400 font-inter">
            Unified view of host health, stack status, and actionable tasks.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            className="btn btn-secondary flex items-center gap-2"
            onClick={() => tasksQuery.refetch()}
            type="button"
          >
            <Loader2 className={`h-4 w-4 ${tasksQuery.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </button>
          <button
            className="btn btn-primary flex items-center gap-2"
            onClick={() => setIsCreateModalOpen(true)}
            type="button"
          >
            <Plus className="h-4 w-4" />
            New Task
          </button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {summaryCards.map((card) => (
          <div
            key={card.title}
            className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
          >
            <p className="text-sm text-gray-500 dark:text-gray-400 font-inter">{card.title}</p>
            <p className={`mt-2 text-3xl font-semibold font-space ${card.highlight ?? "text-gray-900 dark:text-white"}`}>
              {card.value}
              {card.subvalue !== undefined && (
                <span className="ml-1 text-sm font-medium text-gray-500 dark:text-gray-400">
                  / {card.subvalue}
                </span>
              )}
            </p>
          </div>
        ))}
      </div>

      <div className="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 px-6 py-4 dark:border-gray-800">
          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Action Items</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 font-inter">
              System and manual tasks that require attention across your fleet.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <FilterSelect
              label="Status"
              options={statusOptions}
              value={filters.status}
              onChange={(value) => handleFilterClick("status", value)}
            />
            <FilterSelect
              label="Severity"
              options={severityOptions}
              value={filters.severity}
              onChange={(value) => handleFilterClick("severity", value)}
            />
            <FilterSelect
              label="Source"
              options={sourceOptions}
              value={filters.source}
              onChange={(value) => handleFilterClick("source", value)}
            />
          </div>
        </div>

        <div className="overflow-x-auto">
          {busy ? (
            <div className="flex items-center justify-center py-12 text-gray-600 dark:text-gray-400">
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              Loading dashboard data...
            </div>
          ) : tasks.length === 0 ? (
            <div className="py-12 text-center">
              <CheckCircle2 className="mx-auto h-10 w-10 text-emerald-500" />
              <p className="mt-3 text-sm text-gray-600 dark:text-gray-400 font-inter">
                No tasks match your filters.
              </p>
            </div>
          ) : (
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-800">
              <thead className="bg-gray-50 dark:bg-gray-800/40">
                <tr>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Task
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Severity
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Status
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Source
                  </th>
                  <th scope="col" className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Updated
                  </th>
                  <th scope="col" className="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
                {tasks.map((task) => (
                  <tr key={task.id} className="bg-white dark:bg-gray-900">
                    <td className="px-6 py-4 align-top">
                      <div className="font-medium text-gray-900 dark:text-white">{task.title}</div>
                      {task.description && (
                        <div className="mt-1 text-sm text-gray-600 dark:text-gray-400">{task.description}</div>
                      )}
                      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                        {task.category && <span className="font-medium uppercase tracking-wide">{task.category}</span>}
                        {task.task_type && <span>{task.task_type}</span>}
                        {task.host_id && <span>Host: {task.host_id}</span>}
                        {task.stack_id && <span>Stack: {task.stack_id}</span>}
                      </div>
                    </td>
                    <td className="px-6 py-4 align-top">{renderSeverityBadge(task)}</td>
                    <td className="px-6 py-4 align-top">{renderStatusBadge(task)}</td>
                    <td className="px-6 py-4 align-top">{renderSourceBadge(task)}</td>
                    <td className="px-6 py-4 align-top text-sm text-gray-500 dark:text-gray-400">
                      {new Date(task.updated_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-4 align-top">
                      <div className="flex justify-end gap-2">
                        {task.status !== "acknowledged" && (
                          <button
                            type="button"
                            className="btn btn-secondary btn-sm"
                            onClick={() => updateStatusMutation.mutate({ id: task.id, status: "acknowledged" })}
                          >
                            Acknowledge
                          </button>
                        )}
                        {task.status !== "resolved" && (
                          <button
                            type="button"
                            className="btn btn-success btn-sm"
                            onClick={() => updateStatusMutation.mutate({ id: task.id, status: "resolved" })}
                          >
                            Resolve
                          </button>
                        )}
                        {task.status !== "dismissed" && (
                          <button
                            type="button"
                            className="btn btn-danger btn-sm"
                            onClick={() => updateStatusMutation.mutate({ id: task.id, status: "dismissed" })}
                          >
                            Dismiss
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {!busy && (
          <div className="flex items-center justify-between px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
            <span>Total tasks: {tasksQuery.data?.total ?? 0}</span>
            <span>Showing {tasks.length} items</span>
          </div>
        )}
      </div>

      <Modal
        isOpen={isCreateModalOpen}
        onClose={() => {
          setIsCreateModalOpen(false);
          setFormState(initialFormState);
        }}
        title="Create Manual Task"
      >
        <form onSubmit={handleCreateTask} className="space-y-4 px-6 py-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Title
            </label>
            <input
              type="text"
              required
              value={formState.title}
              onChange={(e) => setFormState((prev) => ({ ...prev, title: e.target.value }))}
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Description
            </label>
            <textarea
              value={formState.description}
              onChange={(e) => setFormState((prev) => ({ ...prev, description: e.target.value }))}
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              rows={3}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Severity
              </label>
              <select
                value={formState.severity}
                onChange={(e) =>
                  setFormState((prev) => ({ ...prev, severity: e.target.value as DashboardTaskSeverity }))
                }
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              >
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Category
              </label>
              <input
                type="text"
                value={formState.category}
                onChange={(e) => setFormState((prev) => ({ ...prev, category: e.target.value }))}
                placeholder="e.g. host, stack"
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Host
              </label>
              <select
                value={formState.hostId}
                onChange={(e) => setFormState((prev) => ({ ...prev, hostId: e.target.value }))}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              >
                <option value="">Unassigned</option>
                {hostsQuery.data?.map((host) => (
                  <option key={host.id} value={host.id}>
                    {host.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Stack
              </label>
              <input
                type="text"
                value={formState.stackId}
                onChange={(e) => setFormState((prev) => ({ ...prev, stackId: e.target.value }))}
                placeholder="Optional stack identifier"
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Task Type
            </label>
            <input
              type="text"
              value={formState.taskType}
              onChange={(e) => setFormState((prev) => ({ ...prev, taskType: e.target.value }))}
              placeholder="Short identifier for filtering"
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Due At
              </label>
              <input
                type="datetime-local"
                value={formState.dueAt}
                onChange={(e) => setFormState((prev) => ({ ...prev, dueAt: e.target.value }))}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Snooze Until
              </label>
              <input
                type="datetime-local"
                value={formState.snoozedUntil}
                onChange={(e) => setFormState((prev) => ({ ...prev, snoozedUntil: e.target.value }))}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
              />
            </div>
          </div>

          <div className="flex items-center justify-end gap-2 pt-2">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => {
                setIsCreateModalOpen(false);
                setFormState(initialFormState);
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={createTaskMutation.isPending}
            >
              {createTaskMutation.isPending ? (
                <span className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Creating...
                </span>
              ) : (
                "Create Task"
              )}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
};

interface FilterSelectProps<T extends string | undefined> {
  label: string;
  options: { label: string; value?: T }[];
  value?: T;
  onChange: (value?: T) => void;
}

function FilterSelect<T extends string | undefined>({ label, options, value, onChange }: FilterSelectProps<T>) {
  return (
    <div className="flex items-center gap-2 text-sm font-inter">
      <span className="text-gray-600 dark:text-gray-300">{label}:</span>
      <div className="flex rounded-md border border-gray-200 bg-gray-50 p-0.5 dark:border-gray-700 dark:bg-gray-800">
        {options.map((option) => {
          const selected = option.value === value || (!option.value && !value);
          return (
            <button
              key={option.label}
              type="button"
              onClick={() => onChange(option.value)}
              className={`rounded-md px-3 py-1 text-xs font-medium transition ${
                selected
                  ? "bg-cyan-600 text-white shadow-sm"
                  : "text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700"
              }`}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export default Dashboard;

