import { useEffect, useRef, useCallback } from "react";
import { useAppStore } from "../stores/appStore";
import { useAuthStore } from "../stores/authStore";

export const useWebSocket = () => {
  const { setWebSocketConnection, setConnected } = useAppStore();
  const token = useAuthStore((s) => s.accessToken);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const suppressReconnectRef = useRef(false);
  const maxReconnectAttempts = 5;

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
  }, []);

  const closeConnection = useCallback(
    (suppressReconnect = false) => {
      if (suppressReconnect) {
        suppressReconnectRef.current = true;
      }

      if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) {
        try {
          wsRef.current.close();
        } catch (error) {
          console.warn("WebSocket cleanup error:", error);
        }
      } else {
        suppressReconnectRef.current = false;
      }

      wsRef.current = null;
      setWebSocketConnection(null);
      setConnected(false);
    },
    [setConnected, setWebSocketConnection]
  );

  const connect = useCallback(() => {
    // Clear any existing reconnection timeout
    clearReconnectTimer();

    // Close existing connection if any
    closeConnection(true);

    // Determine WebSocket URL based on environment
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const accessToken = useAuthStore.getState().accessToken;
    if (!accessToken) {
      console.debug("WebSocket connect skipped: missing access token");
      return;
    }
    const wsUrl = `${protocol}//${window.location.host}/ws/ui`;
    const protocols = ["flotilla-ui", `auth-${accessToken}`];

    // Create WebSocket connection
    const ws = new WebSocket(wsUrl, protocols);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WebSocket connected successfully");
      setWebSocketConnection(ws);
      setConnected(true);
      reconnectAttemptsRef.current = 0; // Reset attempts on successful connection
    };

    ws.onclose = (event) => {
      console.log("WebSocket closed:", event.code, event.reason);
      setWebSocketConnection(null);
      setConnected(false);

      if (suppressReconnectRef.current) {
        suppressReconnectRef.current = false;
        return;
      }

      // Attempt to reconnect with exponential backoff
      if (reconnectAttemptsRef.current < maxReconnectAttempts) {
        const delay = Math.min(
          1000 * Math.pow(2, reconnectAttemptsRef.current),
          30000
        );
        reconnectAttemptsRef.current++;

        console.log(
          `Attempting to reconnect in ${delay}ms (attempt ${reconnectAttemptsRef.current}/${maxReconnectAttempts})`
        );

        reconnectTimeoutRef.current = window.setTimeout(() => {
          connect();
        }, delay);
      } else {
        console.log("Max reconnection attempts reached");
      }
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      setConnected(false);
    };

    ws.onmessage = (event) => {
      try {
        JSON.parse(event.data);
        // Handle different message types here
      } catch (error) {
        console.warn("WebSocket message parse error:", error);
      }
    };
  }, [
    clearReconnectTimer,
    closeConnection,
    setWebSocketConnection,
    setConnected,
  ]);

  // Manual reconnect function
  const reconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0; // Reset attempts for manual reconnect
    connect();
  }, [connect]);

  useEffect(() => {
    if (!token) {
      clearReconnectTimer();
      closeConnection(true);
      return;
    }

    connect();

    return () => {
      clearReconnectTimer();
      closeConnection(true);
    };
  }, [token, connect, clearReconnectTimer, closeConnection]);

  return { reconnect };
};
