import { Link, Routes, Route, Navigate, useLocation } from "react-router-dom";
import Users from "./Users";
import ApiKeys from "./ApiKeys";
import General from "./General";
import Logs from "./Logs";
import { useAuthStore } from "../../stores/authStore";

export default function Settings() {
  const location = useLocation();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role?.toLowerCase() === "admin";
  const tabs = [
    ...(isAdmin
      ? [
          { name: "Users", href: "/settings/users" },
          { name: "Agent API Keys", href: "/settings/api-keys" },
        ]
      : []),
    { name: "General", href: "/settings/general" },
    { name: "Logs", href: "/settings/logs" },
  ];
  return (
    <div>
      <div className="mb-4 flex gap-4 border-b">
        {tabs.map((t) => (
          <Link key={t.name} to={t.href} className={`pb-2 ${location.pathname===t.href? 'border-b-2 border-blue-600 text-blue-600':'text-gray-600 dark:text-gray-400'}`}>{t.name}</Link>
        ))}
      </div>
      <Routes>
        <Route path="" element={<Navigate to={isAdmin ? "users" : "general"} replace />} />
        {isAdmin && <Route path="users" element={<Users />} />}
        {isAdmin && <Route path="api-keys" element={<ApiKeys />} />}
        {!isAdmin && <Route path="users" element={<Navigate to="../general" replace />} />}
        {!isAdmin && <Route path="api-keys" element={<Navigate to="../general" replace />} />}
        <Route path="general" element={<General />} />
        <Route path="logs" element={<Logs />} />
      </Routes>
    </div>
  );
}


