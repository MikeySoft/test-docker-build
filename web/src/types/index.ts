export interface Host {
  id: string;
  name: string;
  description?: string;
  agent_version?: string;
  docker_version?: string;
  last_seen?: string;
  status: "online" | "offline" | "error";
  created_at: string;
  updated_at: string;
}

export interface Container {
  id: string;
  name: string;
  service_name?: string;
  image: string;
  state?: string;
  status: "running" | "stopped" | "exited" | "paused" | "restarting";
  created: string;
  ports?: Port[];
  labels?: Record<string, string>;
  host_id?: string;
  host_name?: string;
}

export interface DockerImage {
  id: string;
  short_id?: string;
  image?: string;
  tag?: string;
  repository?: string;
  tags?: string[];
  tags_str?: string;
  digests?: string[];
  digests_str?: string;
  size: number;
  shared_size?: number;
  virtual_size?: number;
  created: number;
  labels?: Record<string, string>;
  containers?: number;
  dangling?: boolean;
  dangling_str?: string;
  status?: string;
  host_name?: string;
}

export interface DockerNetwork {
  id: string;
  name: string;
  driver: string;
  scope?: string;
  internal?: boolean;
  attachable?: boolean;
  ingress?: boolean;
  enable_ipv6?: boolean;
  created?: string;
  labels?: Record<string, string>;
  options?: Record<string, string>;
  containers?: number;
  config_only?: boolean;
  config_from?: {
    Network?: string;
  } | null;
  connected?: boolean;
  host_name?: string;
  containers_detail?: NetworkAttachment[];
  stacks?: string[];
  topology_snapshot?: Record<string, any>;
  topology_refreshed_at?: string;
  topology_is_stale?: boolean;
  topology_metadata_pending?: boolean;
}

export interface DockerVolume {
  name: string;
  driver: string;
  mountpoint: string;
  created?: string;
  labels?: Record<string, string>;
  scope?: string;
  status?: Record<string, any> | null;
  options?: Record<string, string>;
  size_bytes?: number;
  ref_count?: number;
  host_name?: string;
  containers?: number;
  containers_detail?: VolumeConsumer[];
  stacks?: string[];
  topology_snapshot?: Record<string, any>;
  topology_refreshed_at?: string;
  topology_is_stale?: boolean;
  topology_metadata_pending?: boolean;
}

export type ResourceRemovalType = "image" | "volume" | "network";

export interface ResourceRemovalBlocker {
  kind: string;
  id?: string;
  name?: string;
  stack?: string;
  details?: Record<string, string>;
}

export interface ResourceRemovalConflict {
  resource_type: ResourceRemovalType;
  resource_id?: string;
  resource_name?: string;
  reason: string;
  blockers?: ResourceRemovalBlocker[];
  force_supported: boolean;
  original_error?: string;
}

export interface ResourceRemovalError {
  resource_type: ResourceRemovalType;
  resource_id?: string;
  resource_name?: string;
  message: string;
}

export interface ResourceRemovalResult {
  removed: string[];
  conflicts?: ResourceRemovalConflict[];
  errors?: ResourceRemovalError[];
}

export type RemoveImagesResponse = ResourceRemovalResult;

export interface PruneImagesResponse {
  removed: string[];
  space_reclaimed?: number;
}

export interface AppLogEntry {
  id: string;
  timestamp: string;
  level: string;
  source: string;
  message: string;
  fields?: Record<string, any>;
}

export interface AppLogsResponse {
  logs: AppLogEntry[];
  next_cursor?: string;
}

export type DashboardTaskStatus = "open" | "acknowledged" | "resolved" | "dismissed";
export type DashboardTaskSeverity = "info" | "warning" | "critical";
export type DashboardTaskSource = "system" | "manual";

export interface DashboardSummary {
  hosts_total: number;
  hosts_online: number;
  hosts_offline: number;
  hosts_error: number;
  containers_total: number;
  stacks_total: number;
  updated_at: string;
}

export interface DashboardTask {
  id: string;
  title: string;
  description?: string;
  status: DashboardTaskStatus;
  severity: DashboardTaskSeverity;
  source: DashboardTaskSource;
  category?: string;
  task_type?: string;
  fingerprint?: string;
  host_id?: string;
  stack_id?: string;
  container_id?: string;
  metadata?: Record<string, any>;
  due_at?: string | null;
  snoozed_until?: string | null;
  acknowledged_at?: string | null;
  resolved_at?: string | null;
  created_by?: string | null;
  acknowledged_by?: string | null;
  resolved_by?: string | null;
  created_at: string;
  updated_at: string;
}

export interface DashboardTaskListResponse {
  tasks: DashboardTask[];
  total: number;
}

export interface DashboardTaskQueryParams {
  status?: string;
  severity?: string;
  source?: string;
  limit?: number;
  offset?: number;
}

export interface CreateDashboardTaskPayload {
  title: string;
  description?: string;
  severity?: DashboardTaskSeverity;
  category?: string;
  task_type?: string;
  metadata?: Record<string, any>;
  host_id?: string;
  stack_id?: string;
  container_id?: string;
  due_at?: string | null;
  snoozed_until?: string | null;
}

export interface UpdateDashboardTaskPayload {
  title?: string;
  description?: string | null;
  severity?: DashboardTaskSeverity;
  category?: string | null;
  task_type?: string | null;
  metadata?: Record<string, any> | null;
  due_at?: string | null;
  snoozed_until?: string | null;
}

export interface NetworkAttachment {
  id: string;
  name?: string;
  stack?: string;
  service?: string;
  ipv4?: string;
  ipv6?: string;
  mac?: string;
}

export interface VolumeConsumer {
  id: string;
  name?: string;
  stack?: string;
  service?: string;
  destination?: string;
  mode?: string;
  rw?: boolean;
}

export interface TopologySnapshot {
  snapshot: Record<string, any>;
  refreshed_at: string;
  is_stale: boolean;
  host_id: string;
  resource_type: "network" | "volume";
}

export interface TopologyRefreshResponse {
  status: string;
  host_id: string;
  topology: Record<string, TopologySnapshot>;
  refreshed: string;
  requested?: string[];
}

export interface Port {
  private_port: number;
  public_port?: number;
  type: "tcp" | "udp";
}

export interface Stack {
  id: string;
  host_id: string;
  host_name?: string;
  name: string;
  compose_content: string;
  env_vars?: Record<string, string>;
  status: "running" | "stopped" | "error";
  imported?: boolean;
  env_vars_sensitive?: boolean;
  managed_by_flotilla?: boolean;
  containers?: number;
  running?: number;
  created_at: string;
  updated_at: string;
}

export interface Service {
  id: string;
  name: string;
  image: string;
  status: "running" | "stopped" | "exited";
  ports?: Port[];
  environment?: Record<string, string>;
}

export interface ApiError {
  message: string;
  code?: string;
  details?: any;
}

export interface CreateContainerPayload {
  name: string;
  image: string;
  command?: string;
  env?: string[];
  ports?: Record<string, number>;
  volumes?: string[];
  labels?: Record<string, string>;
  restart?: "no" | "on-failure" | "always" | "unless-stopped";
  auto_start?: boolean;
}

export interface CreateContainerResponse {
  message: string;
  container_id: string;
  name: string;
  auto_started: boolean;
}

export interface WebSocketMessage {
  type: "command" | "response" | "event" | "heartbeat";
  id?: string;
  timestamp: string;
  payload: any;
}

export interface DeployStackPayload {
  name: string;
  compose_content: string;
  env_vars?: Record<string, string>;
  status?: "running" | "stopped" | "error";
}

export interface UpdateStackPayload {
  compose_content: string;
  env_vars?: Record<string, string>;
}

export interface ImportStackPayload {
  name: string;
  compose: string;
  env_vars?: Record<string, string>;
}

export interface MetricDataPoint {
  timestamp: string;
  value: number;
}

export interface ContainerMetric {
  timestamp?: string;
  container_id: string;
  container_name: string;
  stack_name?: string;
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
  disk_read_bytes: number;
  disk_write_bytes: number;
  network_rx_bytes?: number;
  network_tx_bytes?: number;
}

export interface HostMetric {
  timestamp?: string;
  cpu_percent: number;
  memory_usage: number;
  memory_total: number;
  disk_usage: number;
  disk_total: number;
}

export interface MetricsQueryParams {
  start?: string;
  end?: string;
  interval?: string;
}

export interface ContainerMetricsResponse {
  host_id: string;
  container_id: string;
  metrics: ContainerMetric[];
}

export interface HostMetricsResponse {
  host_id: string;
  metrics: HostMetric[];
}
