import { create } from "zustand";

type User = { id: string; username: string; role: string } | null;

interface AuthState {
  accessToken: string | null;
  csrfToken: string | null;
  user: User;
  setAuth: (token: string, csrf: string, user?: User) => void;
  setAccessToken: (token: string | null) => void;
  setCsrfToken: (csrf: string | null) => void;
  clear: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  accessToken: null,
  csrfToken: null,
  user: null,
  setAuth: (token, csrf, user) =>
    set({ accessToken: token, csrfToken: csrf, user: user ?? null }),
  setAccessToken: (token) => set({ accessToken: token }),
  setCsrfToken: (csrf) => set({ csrfToken: csrf }),
  clear: () => set({ accessToken: null, csrfToken: null, user: null }),
}));
