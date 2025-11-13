import { useState, useEffect } from "react";
import { Save } from "lucide-react";
import ConfirmModal from "../../components/ConfirmModal";
import { useToast } from "../../contexts/useToast";

const SETTINGS_STORAGE_KEY = "flotilla_settings";

interface GeneralSettings {
  serverName: string;
  timezone: string;
  theme: string;
  notifications: boolean;
  autoRefresh: boolean;
  refreshInterval: number;
}

export default function GeneralSettings() {
  const { showSuccess, showError } = useToast();
  const [settings, setSettings] = useState<GeneralSettings>({
    serverName: "Flotilla Server",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    theme: "system",
    notifications: true,
    autoRefresh: true,
    refreshInterval: 30
  });

  const [showSaveModal, setShowSaveModal] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  // Load settings from localStorage on mount
  useEffect(() => {
    const saved = localStorage.getItem(SETTINGS_STORAGE_KEY);
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        setSettings((prev) => ({ ...prev, ...parsed }));
      } catch (err) {
        console.error("Failed to load settings:", err);
      }
    }
  }, []);

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    setShowSaveModal(true);
  };

  const handleConfirmSave = async () => {
    setIsSaving(true);
    try {
      // Save to localStorage (can be migrated to API later)
      localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings));

      // Apply theme if changed
      if (settings.theme === "dark") {
        document.documentElement.classList.add("dark");
      } else if (settings.theme === "light") {
        document.documentElement.classList.remove("dark");
      } else {
        // System theme - respect browser preference
        const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
        if (prefersDark) {
          document.documentElement.classList.add("dark");
        } else {
          document.documentElement.classList.remove("dark");
        }
      }

      setShowSaveModal(false);
      showSuccess("Settings saved successfully!");
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : "Unknown error occurred";
      showError(`Failed to save settings: ${errorMessage}`);
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">General Settings</h2>

      <form onSubmit={handleSave} className="space-y-6">
        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Server Configuration</h3>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label htmlFor="serverName" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Server Name
              </label>
              <input
                id="serverName"
                type="text"
                value={settings.serverName}
                onChange={(e) => setSettings({ ...settings, serverName: e.target.value })}
                className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
              />
            </div>

            <div>
              <label htmlFor="timezone" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Timezone
              </label>
              <select
                id="timezone"
                value={settings.timezone}
                onChange={(e) => setSettings({ ...settings, timezone: e.target.value })}
                className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
              >
                <option value="UTC">UTC</option>
                <option value="America/New_York">Eastern Time</option>
                <option value="America/Chicago">Central Time</option>
                <option value="America/Denver">Mountain Time</option>
                <option value="America/Los_Angeles">Pacific Time</option>
                <option value="Europe/London">London</option>
                <option value="Europe/Paris">Paris</option>
                <option value="Asia/Tokyo">Tokyo</option>
              </select>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">User Interface</h3>

          <div className="space-y-4">
            <div>
              <label htmlFor="theme" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Theme
              </label>
              <select
                id="theme"
                value={settings.theme}
                onChange={(e) => setSettings({ ...settings, theme: e.target.value })}
                className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
              >
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </div>

            <div className="flex items-center justify-between">
              <div>
                <label htmlFor="notifications" className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Enable Notifications
                </label>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Show browser notifications for important events
                </p>
              </div>
              <input
                id="notifications"
                type="checkbox"
                checked={settings.notifications}
                onChange={(e) => setSettings({ ...settings, notifications: e.target.checked })}
                className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
              />
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">Data Refresh</h3>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <label htmlFor="autoRefresh" className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Auto Refresh
                </label>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Automatically refresh data in the background
                </p>
              </div>
              <input
                id="autoRefresh"
                type="checkbox"
                checked={settings.autoRefresh}
                onChange={(e) => setSettings({ ...settings, autoRefresh: e.target.checked })}
                className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
              />
            </div>

            {settings.autoRefresh && (
              <div>
                <label htmlFor="refreshInterval" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Refresh Interval (seconds)
                </label>
                <select
                  id="refreshInterval"
                  value={settings.refreshInterval}
                  onChange={(e) => setSettings({ ...settings, refreshInterval: parseInt(e.target.value) })}
                  className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
                >
                  <option value={10}>10 seconds</option>
                  <option value={30}>30 seconds</option>
                  <option value={60}>1 minute</option>
                  <option value={300}>5 minutes</option>
                </select>
              </div>
            )}
          </div>
        </div>

        <div className="flex justify-end">
          <button
            type="submit"
            className="btn btn-primary inline-flex items-center gap-2"
          >
            <Save className="h-4 w-4" />
            Save Settings
          </button>
        </div>
      </form>

      <ConfirmModal
        isOpen={showSaveModal}
        onClose={() => setShowSaveModal(false)}
        onConfirm={handleConfirmSave}
        title="Save Settings"
        message="Are you sure you want to save these settings?"
        confirmText="Save"
        variant="info"
        isLoading={isSaving}
      />
    </div>
  );
}


