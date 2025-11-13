import { create } from "zustand";
import type { Host, Container, Stack } from "../types";

interface AppState {
  // Hosts
  hosts: Host[];
  selectedHost: Host | null;

  // Containers
  containers: Container[];

  // Stacks
  stacks: Stack[];

  // UI State
  isLoading: boolean;
  error: string | null;

  // WebSocket
  wsConnection: WebSocket | null;
  isConnected: boolean;

  // Actions
  setHosts: (hosts: Host[]) => void;
  setSelectedHost: (host: Host | null) => void;
  updateHostStatus: (hostId: string, status: Host["status"]) => void;
  removeHost: (hostId: string) => void;

  setContainers: (containers: Container[]) => void;
  addContainer: (container: Container) => void;
  updateContainer: (containerId: string, updates: Partial<Container>) => void;
  removeContainer: (containerId: string) => void;

  setStacks: (stacks: Stack[]) => void;
  addStack: (stack: Stack) => void;
  updateStack: (stackId: string, updates: Partial<Stack>) => void;
  removeStack: (stackId: string) => void;

  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;

  setWebSocketConnection: (ws: WebSocket | null) => void;
  setConnected: (connected: boolean) => void;
}

export const useAppStore = create<AppState>((set) => ({
  // Initial state
  hosts: [],
  selectedHost: null,
  containers: [],
  stacks: [],
  isLoading: false,
  error: null,
  wsConnection: null,
  isConnected: false,

  // Host actions
  setHosts: (hosts) => set({ hosts }),

  setSelectedHost: (host) => set({ selectedHost: host }),

  updateHostStatus: (hostId, status) =>
    set((state) => ({
      hosts: state.hosts.map((host) =>
        host.id === hostId ? { ...host, status } : host
      ),
      selectedHost:
        state.selectedHost?.id === hostId
          ? { ...state.selectedHost, status }
          : state.selectedHost,
    })),

  removeHost: (hostId) =>
    set((state) => ({
      hosts: state.hosts.filter((host) => host.id !== hostId),
      selectedHost:
        state.selectedHost?.id === hostId ? null : state.selectedHost,
    })),

  // Container actions
  setContainers: (containers) => set({ containers }),

  addContainer: (container) =>
    set((state) => ({
      containers: [...state.containers, container],
    })),

  updateContainer: (containerId, updates) =>
    set((state) => ({
      containers: state.containers.map((container) =>
        container.id === containerId ? { ...container, ...updates } : container
      ),
    })),

  removeContainer: (containerId) =>
    set((state) => ({
      containers: state.containers.filter(
        (container) => container.id !== containerId
      ),
    })),

  // Stack actions
  setStacks: (stacks) => set({ stacks }),

  addStack: (stack) =>
    set((state) => ({
      stacks: [...state.stacks, stack],
    })),

  updateStack: (stackId, updates) =>
    set((state) => ({
      stacks: state.stacks.map((stack) =>
        stack.id === stackId ? { ...stack, ...updates } : stack
      ),
    })),

  removeStack: (stackId) =>
    set((state) => ({
      stacks: state.stacks.filter((stack) => stack.id !== stackId),
    })),

  // UI actions
  setLoading: (isLoading) => set({ isLoading }),

  setError: (error) => set({ error }),

  // WebSocket actions
  setWebSocketConnection: (wsConnection) => set({ wsConnection }),

  setConnected: (isConnected) => set({ isConnected }),
}));
