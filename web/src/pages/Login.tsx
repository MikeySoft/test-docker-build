import { useState, useEffect } from "react";
import apiClient from "../api/client";
import { useAuthStore } from "../stores/authStore";
import { useNavigate, useLocation } from "react-router-dom";
import { useToast } from "../contexts/useToast";

export default function Login() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [checkingSetup, setCheckingSetup] = useState(true);
  const setAuth = useAuthStore((s) => s.setAuth);
  const navigate = useNavigate();
  const location = useLocation();
  const toast = useToast();

  useEffect(() => {
    let mounted = true;
    fetch("/api/v1/auth/setup", { credentials: "same-origin" })
      .then(async (res) => {
        if (!mounted) return;
        const data = await res.json();
        if (data?.setup === true) {
          navigate("/setup", { replace: true });
        } else {
          setCheckingSetup(false);
        }
      })
      .catch(() => setCheckingSetup(false));
    return () => {
      mounted = false;
    };
  }, [navigate]);

  useEffect(() => {
    if (location.state && location.state.justSetup) {
      toast.showSuccess("Administrator account created! You can now log in.");
      window.history.replaceState({}, document.title); // clear location state
    }
  }, [location.state, toast]);

  if (checkingSetup) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
        <div className="text-gray-700 dark:text-gray-200 text-lg">Checking system state…</div>
      </div>
    );
  }

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const res = await apiClient.login(username, password);
      setAuth(res.access_token, res.csrf_token, res.user);
      navigate("/");
    } catch (err: any) {
      setError(err?.message ?? "Login failed");
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
      <form onSubmit={onSubmit} className="w-full max-w-sm bg-white dark:bg-gray-800 p-6 rounded shadow">
        <h1 className="text-xl font-semibold mb-4 text-gray-900 dark:text-gray-100">Sign in</h1>
        <div className="mb-3">
          <label htmlFor="username" className="block text-sm mb-1 text-gray-700 dark:text-gray-300">Username</label>
          <input id="username" value={username} onChange={(e)=>setUsername(e.target.value)} className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
        </div>
        <div className="mb-4">
          <label htmlFor="password" className="block text-sm mb-1 text-gray-700 dark:text-gray-300">Password</label>
          <input id="password" type="password" value={password} onChange={(e)=>setPassword(e.target.value)} className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
        </div>
        {error && <div className="text-red-600 text-sm mb-3">{error}</div>}
        <button type="submit" className="btn btn-primary inline-flex items-center justify-center gap-2 w-full">Sign in</button>
      </form>
    </div>
  );
}


