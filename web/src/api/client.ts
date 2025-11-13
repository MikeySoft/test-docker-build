import axios from "axios";
import { useAuthStore } from "../stores/authStore";
import type { AxiosInstance, AxiosResponse } from "axios";
import type {
  Host,
  Container,
  Stack,
  ApiError,
  DeployStackPayload,
  UpdateStackPayload,
  CreateContainerPayload,
  CreateContainerResponse,
  ContainerMetricsResponse,
  HostMetricsResponse,
  MetricsQueryParams,
  DockerImage,
  DockerNetwork,
  DockerVolume,
  RemoveImagesResponse,
  PruneImagesResponse,
  AppLogsResponse,
  TopologyRefreshResponse,
  ResourceRemovalResult,
  DashboardSummary,
  DashboardTask,
  DashboardTaskListResponse,
  DashboardTaskQueryParams,
  CreateDashboardTaskPayload,
  UpdateDashboardTaskPayload,
  DashboardTaskStatus,
} from "../types";

class ApiClient {
  private readonly client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: "/api/v1",
      timeout: 30000,
      headers: {
        "Content-Type": "application/json",
      },
    });
    // Always send cookies for same-origin (refresh cookie)
    this.client.defaults.withCredentials = true;

    // Request interceptor to attach Authorization and CSRF
    this.client.interceptors.request.use(
      (config) => {
        try {
          const { accessToken, csrfToken } = useAuthStore.getState();
          if (accessToken) {
            config.headers = config.headers || {};
            (config.headers as any)["Authorization"] = `Bearer ${accessToken}`;
          }
          // If we don't have a CSRF token in state (e.g., after reload), try cookie
          let token = csrfToken;
          if (!token && typeof document !== "undefined") {
            const regex = /(?:^|; )flotilla_csrf=([^;]+)/;
            const match = regex.exec(document.cookie);
            if (match) token = decodeURIComponent(match[1]);
          }
          if (token) {
            (config.headers as any)["X-CSRF-Token"] = token;
          }
        } catch (err) {
          // Swallow errors (store not initialized, document unavailable) and leave headers unset.
          if (import.meta.env.DEV) {
            console.debug("Auth header injection skipped:", err);
          }
        }
        return config;
      },
      (error) => {
        return Promise.reject(new Error(error.message ?? "Request failed"));
      }
    );

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response: AxiosResponse) => response,
      (error) => {
        // Attempt one refresh on 401 then retry original request
        if (error?.response?.status === 401 && !error.config.__isRetryRequest) {
          const original = error.config;
          original.__isRetryRequest = true;
          return this.refresh()
            .then(() => {
              const { accessToken } = useAuthStore.getState();
              if (accessToken) {
                original.headers = original.headers ?? {};
                original.headers["Authorization"] = `Bearer ${accessToken}`;
              }
              return this.client.request(original);
            })
            .catch((e) => {
              useAuthStore.getState().clear();
              return Promise.reject(
                e instanceof Error ? e : new Error(String(e))
              );
            });
        }
        const apiError: ApiError = {
          message:
            error?.response?.data?.message ??
            error?.message ??
            "An unexpected error occurred",
          code: error?.response?.data?.code ?? error?.code,
          details: error?.response?.data?.details,
        };
        console.error("API Response Error:", apiError);
        return Promise.reject(new Error(apiError.message));
      }
    );
  }

  async login(
    username: string,
    password: string
  ): Promise<{ access_token: string; csrf_token: string; user: any }> {
    const res = await this.client.post("/auth/login", { username, password });
    const access = res.data.access_token as string;
    const csrf = (res.headers["x-csrf-token"] as string) || "";
    // persist in store
    const setAuth = useAuthStore.getState().setAuth;
    setAuth(access, csrf, res.data.user);
    return { access_token: access, csrf_token: csrf, user: res.data.user };
  }

  async refresh(): Promise<void> {
    const res = await this.client.post("/auth/refresh");
    const access = res.data.access_token as string;
    const csrf = (res.headers["x-csrf-token"] as string) || "";
    useAuthStore.getState().setAuth(access, csrf);
  }

  async logout(): Promise<void> {
    await this.client.post("/auth/logout");
    useAuthStore.getState().clear();
  }

  // Host endpoints
  async getHosts(q?: string): Promise<Host[]> {
    const response = await this.client.get<Host[]>("/hosts", { params: q ? { q } : undefined });
    return response.data;
  }

  async getHost(hostId: string): Promise<Host> {
    const response = await this.client.get<Host>(`/hosts/${hostId}`);
    return response.data;
  }

  async deleteHost(hostId: string): Promise<void> {
    await this.client.delete(`/hosts/${hostId}`);
  }

  async getHostInfo(hostId: string): Promise<{
    docker_version: string;
    ncpu: number;
    mem_total: number;
    disk_total?: number;
    disk_free?: number;
  }> {
    const response = await this.client.get(`/hosts/${hostId}/info`);
    return response.data;
  }

  // Container endpoints
  async getContainers(hostId: string, q?: string): Promise<Container[]> {
    const response = await this.client.get<Container[]>(
      `/hosts/${hostId}/containers`,
      { params: q ? { q } : undefined }
    );
    return response.data;
  }

  async getAllContainers(q?: string): Promise<Container[]> {
    const response = await this.client.get<Container[]>(
      `/containers`,
      { params: { t: Date.now(), ...(q ? { q } : {}) } }
    );
    return response.data;
  }

  async getImages(hostId: string, q?: string): Promise<DockerImage[]> {
    const response = await this.client.get<DockerImage[]>(
      `/hosts/${hostId}/images`,
      { params: q ? { q } : undefined }
    );
    return response.data;
  }

  async getNetworks(hostId: string, q?: string): Promise<DockerNetwork[]> {
    const response = await this.client.get<DockerNetwork[]>(
      `/hosts/${hostId}/networks`,
      { params: q ? { q } : undefined }
    );
    return response.data;
  }

  async inspectNetwork(hostId: string, networkId: string): Promise<any> {
    const response = await this.client.get(
      `/hosts/${hostId}/networks/${encodeURIComponent(networkId)}`
    );
    return response.data;
  }

  async removeNetwork(hostId: string, networkId: string, force = false): Promise<ResourceRemovalResult> {
    const response = await this.client.delete<ResourceRemovalResult>(
      `/hosts/${hostId}/networks/${encodeURIComponent(networkId)}`,
      {
        params: force ? { force: 1 } : undefined,
      }
    );
    return response.data;
  }

  async getVolumes(hostId: string, q?: string): Promise<DockerVolume[]> {
    const response = await this.client.get<DockerVolume[]>(
      `/hosts/${hostId}/volumes`,
      { params: q ? { q } : undefined }
    );
    return response.data;
  }

  async inspectVolume(hostId: string, volumeName: string): Promise<any> {
    const response = await this.client.get(
      `/hosts/${hostId}/volumes/${encodeURIComponent(volumeName)}`
    );
    return response.data;
  }

  async removeVolume(hostId: string, volumeName: string, force = false): Promise<ResourceRemovalResult> {
    const response = await this.client.delete<ResourceRemovalResult>(
      `/hosts/${hostId}/volumes/${encodeURIComponent(volumeName)}`,
      {
        params: force ? { force: 1 } : undefined,
      }
    );
    return response.data;
  }

  async refreshNetworks(hostId: string, ids?: string[]): Promise<TopologyRefreshResponse> {
    const payload = ids && ids.length ? { ids } : {};
    const response = await this.client.post<TopologyRefreshResponse>(
      `/hosts/${hostId}/networks/refresh`,
      payload
    );
    return response.data;
  }

  async refreshVolumes(hostId: string, names?: string[]): Promise<TopologyRefreshResponse> {
    const payload = names && names.length ? { names } : {};
    const response = await this.client.post<TopologyRefreshResponse>(
      `/hosts/${hostId}/volumes/refresh`,
      payload
    );
    return response.data;
  }

  async removeImages(hostId: string, images: string[], force?: boolean): Promise<RemoveImagesResponse> {
    const response = await this.client.post<RemoveImagesResponse>(
      `/hosts/${hostId}/images/remove`,
      { images, ...(force ? { force } : {}) }
    );
    return response.data;
  }

  async pruneDanglingImages(hostId: string): Promise<PruneImagesResponse> {
    const response = await this.client.post<PruneImagesResponse>(
      `/hosts/${hostId}/images/prune`
    );
    return response.data;
  }

  async getAppLogs(after?: string, limit = 200): Promise<AppLogsResponse> {
    const response = await this.client.get<AppLogsResponse>(
      `/logs`,
      {
        params: {
          ...(after ? { after } : {}),
          ...(limit ? { limit } : {}),
        },
      }
    );
    return response.data;
  }

  getAppLogsWebSocketURL(token: string): string {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/ws/logs?token=${encodeURIComponent(token)}`;
  }

  async getContainer(hostId: string, containerId: string): Promise<Container> {
    const response = await this.client.get<Container>(
      `/hosts/${hostId}/containers/${containerId}`
    );
    return response.data;
  }

  async getAllStacks(q?: string): Promise<Stack[]> {
    const response = await this.client.get<Stack[]>(`/stacks`, { params: { t: Date.now(), ...(q ? { q } : {}) } });
    return response.data;
  }

  async startContainer(hostId: string, containerId: string, containerName?: string): Promise<void> {
    await this.client.post(`/hosts/${hostId}/containers/${containerId}/start`, null, {
      params: containerName ? { name: containerName } : undefined,
    });
  }

  async stopContainer(hostId: string, containerId: string, containerName?: string): Promise<void> {
    await this.client.post(
      `/hosts/${hostId}/containers/${containerId}/stop`,
      {},
      {
        timeout: 120000, // 2 minutes for stop operations
        params: containerName ? { name: containerName } : undefined,
      }
    );
  }

  async restartContainer(hostId: string, containerId: string, containerName?: string): Promise<void> {
    await this.client.post(
      `/hosts/${hostId}/containers/${containerId}/restart`,
      {},
      {
        timeout: 120000, // 2 minutes for restart operations
        params: containerName ? { name: containerName } : undefined,
      }
    );
  }

  async removeContainer(hostId: string, containerId: string, containerName?: string): Promise<void> {
    await this.client.post(
      `/hosts/${hostId}/containers/${containerId}/remove`,
      {},
      {
        params: containerName ? { name: containerName } : undefined,
      }
    );
  }

  async createContainer(
    hostId: string,
    payload: CreateContainerPayload
  ): Promise<CreateContainerResponse> {
    const response = await this.client.post<CreateContainerResponse>(
      `/hosts/${hostId}/containers`,
      payload,
      {
        timeout: 60000, // 1 minute for container creation
      }
    );
    return response.data;
  }

  // Stack endpoints
  async getStacks(hostId: string, revealSecrets?: boolean, q?: string): Promise<Stack[]> {
    const response = await this.client.get<Stack[]>(`/hosts/${hostId}/stacks`, {
      params: { ...(revealSecrets ? { reveal_secrets: 1 } : {}), ...(q ? { q } : {}) },
    });
    return response.data;
  }

  async deployStack(
    hostId: string,
    payload: DeployStackPayload
  ): Promise<Stack> {
    const response = await this.client.post<Stack>(
      `/hosts/${hostId}/stacks`,
      payload
    );
    return response.data;
  }

  async updateStack(
    hostId: string,
    stackName: string,
    payload: UpdateStackPayload
  ): Promise<void> {
    await this.client.post(
      `/hosts/${hostId}/stacks/${stackName}/update`,
      payload
    );
  }

  async removeStack(hostId: string, stackName: string): Promise<void> {
    await this.client.post(`/hosts/${hostId}/stacks/${stackName}/remove`);
  }

  async startStack(hostId: string, stackName: string): Promise<void> {
    await this.client.post(`/hosts/${hostId}/stacks/${stackName}/start`);
  }

  async stopStack(hostId: string, stackName: string): Promise<void> {
    await this.client.post(`/hosts/${hostId}/stacks/${stackName}/stop`);
  }

  async restartStack(hostId: string, stackName: string): Promise<void> {
    await this.client.post(`/hosts/${hostId}/stacks/${stackName}/restart`);
  }

  async importStack(hostId: string, payload: any): Promise<any> {
    const response = await this.client.post(
      `/hosts/${hostId}/stacks/import`,
      payload
    );
    return response.data;
  }

  async getStackContainers(
    hostId: string,
    stackName: string
  ): Promise<Container[]> {
    const response = await this.client.get<{ containers: Container[] }>(
      `/hosts/${hostId}/stacks/${stackName}/containers`
    );
    return response.data.containers;
  }

  async stackContainerAction(
    hostId: string,
    stackName: string,
    containerId: string,
    action: "start" | "stop" | "restart"
  ): Promise<void> {
    await this.client.post(
      `/hosts/${hostId}/stacks/${stackName}/containers/${containerId}/${action}`
    );
  }

  // Dashboard endpoints
  async getDashboardSummary(): Promise<DashboardSummary> {
    const response = await this.client.get<DashboardSummary>("/dashboard/summary");
    return response.data;
  }

  async listDashboardTasks(
    params?: DashboardTaskQueryParams
  ): Promise<DashboardTaskListResponse> {
    const response = await this.client.get<DashboardTaskListResponse>(
      "/dashboard/tasks",
      {
        params,
      }
    );
    return response.data;
  }

  async createDashboardTask(
    payload: CreateDashboardTaskPayload
  ): Promise<DashboardTask> {
    const response = await this.client.post<DashboardTask>(
      "/dashboard/tasks",
      payload
    );
    return response.data;
  }

  async updateDashboardTask(
    taskId: string,
    payload: UpdateDashboardTaskPayload
  ): Promise<DashboardTask> {
    const response = await this.client.patch<DashboardTask>(
      `/dashboard/tasks/${taskId}`,
      payload
    );
    return response.data;
  }

  async updateDashboardTaskStatus(
    taskId: string,
    status: DashboardTaskStatus
  ): Promise<DashboardTask> {
    const response = await this.client.post<DashboardTask>(
      `/dashboard/tasks/${taskId}/status`,
      { status }
    );
    return response.data;
  }

  // Log endpoints
  async getContainerLogs(
    hostId: string,
    containerId: string,
    tail?: number
  ): Promise<string> {
    const response = await this.client.get(
      `/hosts/${hostId}/containers/${containerId}/logs`,
      {
        params: { tail },
      }
    );
    return response.data;
  }

  // Metrics endpoints
  async getHostMetrics(
    hostId: string,
    params?: MetricsQueryParams
  ): Promise<HostMetricsResponse> {
    const response = await this.client.get<HostMetricsResponse>(
      `/hosts/${hostId}/metrics`,
      {
        params,
      }
    );
    return response.data;
  }

  async getContainerMetrics(
    hostId: string,
    containerId: string,
    params?: MetricsQueryParams
  ): Promise<ContainerMetricsResponse> {
    const response = await this.client.get<ContainerMetricsResponse>(
      `/hosts/${hostId}/containers/${containerId}/metrics`,
      {
        params,
      }
    );
    return response.data;
  }

  // Generic HTTP methods for settings pages
  async get<T = any>(url: string): Promise<T> {
    const response = await this.client.get<T>(url);
    return response.data;
  }

  async post<T = any>(url: string, data?: any): Promise<T> {
    const response = await this.client.post<T>(url, data);
    return response.data;
  }

  async put<T = any>(url: string, data?: any): Promise<T> {
    const response = await this.client.put<T>(url, data);
    return response.data;
  }

  async delete<T = any>(url: string): Promise<T> {
    const response = await this.client.delete<T>(url);
    return response.data;
  }
}

export const apiClient = new ApiClient();
export default apiClient;
