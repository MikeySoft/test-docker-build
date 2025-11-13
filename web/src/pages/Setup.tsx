import { useState } from "react";
import { useNavigate } from "react-router-dom";

export default function Setup() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!username.trim() || !password) {
      setError("Username and password required.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }
    setLoading(true);
    try {
      const res = await fetch("/api/v1/auth/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data?.message ?? "Setup failed");
      }
      navigate("/login", { state: { justSetup: true } });
    } catch (err: unknown) {
      let message: string | undefined = undefined;
      if (err instanceof Error) {
        message = err.message;
      } else if (typeof err === "object" && err && "message" in err) {
        message = (err as { message?: string }).message;
      }
      setError(message ?? "Setup failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
      <form onSubmit={onSubmit} className="w-full max-w-sm bg-white dark:bg-gray-800 p-6 rounded shadow">
        <h1 className="text-xl font-bold mb-4 text-gray-900 dark:text-gray-100">Administrator Setup</h1>
        <p className="mb-4 text-gray-700 dark:text-gray-300">Welcome! No admin users exist yet. Create your initial administrator account.</p>
        <div className="mb-3">
          <label htmlFor="setup-username" className="block text-sm mb-1 text-gray-700 dark:text-gray-300">Username</label>
          <input id="setup-username" value={username} onChange={(e)=>setUsername(e.target.value)} className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" autoFocus />
        </div>
        <div className="mb-3">
          <label htmlFor="setup-password" className="block text-sm mb-1 text-gray-700 dark:text-gray-300">Password</label>
          <input id="setup-password" type="password" value={password} onChange={(e)=>setPassword(e.target.value)} className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
        </div>
        <div className="mb-4">
          <label htmlFor="setup-confirm" className="block text-sm mb-1 text-gray-700 dark:text-gray-300">Confirm Password</label>
          <input id="setup-confirm" type="password" value={confirmPassword} onChange={(e)=>setConfirmPassword(e.target.value)} className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100" />
        </div>
        {error && <div className="text-red-600 text-sm mb-3">{error}</div>}
        <button type="submit" disabled={loading} className="btn btn-primary inline-flex items-center justify-center gap-2 w-full disabled:opacity-70">{loading ? "Creating..." : "Create Admin Account"}</button>
      </form>
    </div>
  );
}
