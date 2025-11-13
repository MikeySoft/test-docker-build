import { useState, useEffect } from "react";
import { Plus, Trash2, Ban } from "lucide-react";
import apiClient from "../../api/client";
import ConfirmModal from "../../components/ConfirmModal";
import { useToast } from "../../contexts/useToast";

interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  host_id?: string;
  created_at: string;
  last_used?: string;
  is_active: boolean;
}

export default function ApiKeysSettings() {
  const { showSuccess, showError } = useToast();
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newKey, setNewKey] = useState({
    name: "",
    host_id: ""
  });
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    action: 'revoke' | 'delete' | null;
    keyId: string | null;
    keyName: string | null;
  }>({
    isOpen: false,
    action: null,
    keyId: null,
    keyName: null,
  });

  useEffect(() => {
    loadApiKeys();
  }, []);

  const loadApiKeys = async () => {
    try {
      setLoading(true);
      const apiKeys = await apiClient.get<ApiKey[]>("/api-keys");
      setApiKeys(apiKeys);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const response = await apiClient.post<{api_key: string, prefix: string, name: string, host_id: string}>("/api-keys", newKey);
      setCreatedKey(response.api_key);
      setNewKey({ name: "", host_id: "" });
      setShowCreateForm(false);
      loadApiKeys();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    }
  };

  const handleRevokeKey = (keyId: string, keyName: string) => {
    setConfirmModal({
      isOpen: true,
      action: 'revoke',
      keyId,
      keyName,
    });
  };

  const handleDeleteKey = (keyId: string, keyName: string) => {
    setConfirmModal({
      isOpen: true,
      action: 'delete',
      keyId,
      keyName,
    });
  };

  const handleConfirmAction = async () => {
    if (!confirmModal.keyId || !confirmModal.action) return;

    try {
      if (confirmModal.action === 'revoke') {
        await apiClient.delete(`/api-keys/${confirmModal.keyId}`);
        showSuccess("API key revoked successfully");
      } else if (confirmModal.action === 'delete') {
        await apiClient.delete(`/api-keys/${confirmModal.keyId}/permanent`);
        showSuccess("API key deleted successfully");
      }
      loadApiKeys();
      setConfirmModal({ isOpen: false, action: null, keyId: null, keyName: null });
    } catch (err: unknown) {
      showError(err instanceof Error ? err.message : "Failed to perform action");
      setError(err instanceof Error ? err.message : "Unknown error");
    }
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      showSuccess("API key copied to clipboard!");
    } catch (err) {
      console.error("Failed to copy to clipboard:", err);
      showError("Failed to copy to clipboard");
    }
  };

  if (loading) {
    return <div className="text-gray-900 dark:text-gray-100">Loading API keys...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Agent API Keys</h2>
        <button
          onClick={() => setShowCreateForm(true)}
          className="btn btn-primary inline-flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          Create API Key
        </button>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      )}

      <div className="text-sm text-gray-600 dark:text-gray-300">
        For security, the full API key is only shown once at creation. Store it securely. The table shows only the prefix.
      </div>

      {createdKey && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">
          <div className="flex justify-between items-center">
            <div>
              <strong>API Key Created!</strong>
              <p className="text-sm mt-1">Copy this key now - you won't be able to see it again.</p>
            </div>
            <button
              onClick={() => setCreatedKey(null)}
              className="text-green-700 hover:text-green-900"
            >
              ×
            </button>
          </div>
          <div className="mt-2 p-2 bg-white rounded border">
            <code className="text-sm font-mono break-all">{createdKey}</code>
            <button
              onClick={() => copyToClipboard(createdKey)}
              className="ml-2 px-2 py-1 bg-green-600 text-white text-xs rounded hover:bg-green-700"
            >
              Copy
            </button>
          </div>
        </div>
      )}

      {showCreateForm && (
        <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded">
          <h3 className="text-lg font-medium mb-4">Create New API Key</h3>
          <form onSubmit={handleCreateKey} className="space-y-4">
            <div>
              <label htmlFor="api-key-name" className="block text-sm font-medium mb-1">Name</label>
              <input
                id="api-key-name"
                type="text"
                value={newKey.name}
                onChange={(e) => setNewKey({ ...newKey, name: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="e.g., Production Agent"
                required
              />
            </div>
            <div>
              <label htmlFor="api-key-host-id" className="block text-sm font-medium mb-1">Host ID (Optional)</label>
              <input
                id="api-key-host-id"
                type="text"
                value={newKey.host_id}
                onChange={(e) => setNewKey({ ...newKey, host_id: e.target.value })}
                className="w-full border rounded px-3 py-2"
                placeholder="Leave empty for general use"
              />
            </div>
            <div className="flex gap-2">
              <button
                type="submit"
                className="btn btn-primary inline-flex items-center gap-2"
              >
                <Plus className="h-4 w-4" />
                Create
              </button>
              <button
                type="button"
                onClick={() => setShowCreateForm(false)}
                className="btn btn-secondary"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gray-50 dark:bg-gray-700">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Name
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Prefix
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Host ID
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Last Used
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            {apiKeys.map((key) => (
              <tr key={key.id}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                  {key.name}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <code className="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs">
                    FLA_{key.prefix}_****
                  </code>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  {key.host_id ?? "-"}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <span className={`px-2 py-1 text-xs rounded ${
                    key.is_active
                      ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                      : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                  }`}>
                    {key.is_active ? 'Active' : 'Revoked'}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  {key.last_used ? new Date(key.last_used).toLocaleString() : 'Never'}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <div className="flex items-center gap-2">
                    {key.is_active ? (
                      <button
                        onClick={() => handleRevokeKey(key.id, key.name)}
                        className="text-warning-500 hover:text-warning-400 transition-colors"
                        title="Revoke API key"
                      >
                        <Ban className="h-4 w-4" />
                      </button>
                    ) : (
                      <button
                        onClick={() => handleDeleteKey(key.id, key.name)}
                        className="text-danger-500 hover:text-danger-400 transition-colors"
                        title="Delete API key"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {apiKeys.length === 0 && (
        <div className="text-center py-8 text-gray-500 dark:text-gray-400">
          No API keys found. Create one to get started.
        </div>
      )}

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        onClose={() => setConfirmModal({ isOpen: false, action: null, keyId: null, keyName: null })}
        onConfirm={handleConfirmAction}
        title={confirmModal.action === 'revoke' ? 'Revoke API Key' : 'Delete API Key'}
        message={
          confirmModal.action === 'revoke'
            ? `Are you sure you want to revoke the API key "${confirmModal.keyName}"? This will prevent it from being used for authentication.`
            : `Are you sure you want to permanently delete the API key "${confirmModal.keyName}"? This action cannot be undone.`
        }
        confirmText={confirmModal.action === 'revoke' ? 'Revoke' : 'Delete'}
        variant={confirmModal.action === 'delete' ? 'danger' : 'warning'}
      />
    </div>
  );
}


