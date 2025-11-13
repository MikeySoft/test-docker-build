import { useState, useEffect } from "react";
import { Plus, UserCheck, UserX, Trash2, Key } from "lucide-react";
import apiClient from "../../api/client";
import ConfirmModal from "../../components/ConfirmModal";
import { useToast } from "../../contexts/useToast";

interface User {
  id: string;
  username: string;
  email?: string;
  role: string;
  is_active: boolean;
  created_at: string;
  last_login_at?: string;
}

export default function UsersSettings() {
  const { showSuccess, showError } = useToast();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newUser, setNewUser] = useState({
    username: "",
    email: "",
    password: "",
    role: "user"
  });
  const [confirmModal, setConfirmModal] = useState<{
    isOpen: boolean;
    action: 'deactivate' | 'activate' | 'delete' | null;
    userId: string | null;
    userName: string | null;
  }>({
    isOpen: false,
    action: null,
    userId: null,
    userName: null,
  });

  useEffect(() => {
    loadUsers();
  }, []);

  const loadUsers = async () => {
    try {
      setLoading(true);
      const users = await apiClient.get<User[]>("/users");
      setUsers(users);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiClient.post("/users", newUser);
      setNewUser({ username: "", email: "", password: "", role: "user" });
      setShowCreateForm(false);
      loadUsers();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleToggleUser = (userId: string, isActive: boolean, userName: string) => {
    setConfirmModal({
      isOpen: true,
      action: isActive ? 'deactivate' : 'activate',
      userId,
      userName,
    });
  };

  const handleDeleteUser = (userId: string, userName: string) => {
    setConfirmModal({
      isOpen: true,
      action: 'delete',
      userId,
      userName,
    });
  };

  const handleConfirmAction = async () => {
    if (!confirmModal.userId || !confirmModal.action) return;

    try {
      if (confirmModal.action === 'activate' || confirmModal.action === 'deactivate') {
        await apiClient.put(`/users/${confirmModal.userId}`, {
          is_active: confirmModal.action === 'activate'
        });
        showSuccess(`User ${confirmModal.action === 'activate' ? 'activated' : 'deactivated'} successfully`);
      } else if (confirmModal.action === 'delete') {
        await apiClient.delete(`/users/${confirmModal.userId}/permanent`);
        showSuccess("User deleted successfully");
      }
      loadUsers();
      setConfirmModal({ isOpen: false, action: null, userId: null, userName: null });
    } catch (err: any) {
      showError(err.message ?? "Failed to perform action");
      setError(err.message);
    }
  };

  const handleResetPassword = async (userId: string, newPassword: string) => {
    try {
      await apiClient.post(`/users/${userId}/reset-password`, { password: newPassword });
      showSuccess("Password reset successfully");
    } catch (err: any) {
      showError(err.message ?? "Failed to reset password");
      setError(err.message);
    }
  };

  if (loading) {
    return <div className="text-gray-900 dark:text-gray-100">Loading users...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Users</h2>
        <button
          onClick={() => setShowCreateForm(true)}
          className="btn btn-primary inline-flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          Create User
        </button>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      )}

      {showCreateForm && (
        <div className="bg-gray-50 dark:bg-gray-800 p-4 rounded">
          <h3 className="text-lg font-medium mb-4">Create New User</h3>
          <form onSubmit={handleCreateUser} className="space-y-4">
            <div>
              <label htmlFor="newUsername" className="block text-sm font-medium mb-1">Username</label>
              <input
                id="newUsername"
                type="text"
                value={newUser.username}
                onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
                className="w-full border rounded px-3 py-2"
                required
              />
            </div>
            <div>
              <label htmlFor="newEmail" className="block text-sm font-medium mb-1">Email</label>
              <input
                id="newEmail"
                type="email"
                value={newUser.email}
                onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                className="w-full border rounded px-3 py-2"
              />
            </div>
            <div>
              <label htmlFor="newPassword" className="block text-sm font-medium mb-1">Password</label>
              <input
                id="newPassword"
                type="password"
                value={newUser.password}
                onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
                className="w-full border rounded px-3 py-2"
                required
              />
            </div>
            <div>
              <label htmlFor="newRole" className="block text-sm font-medium mb-1">Role</label>
              <select
                id="newRole"
                value={newUser.role}
                onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
                className="w-full border rounded px-3 py-2"
              >
                <option value="user">User</option>
                <option value="admin">Admin</option>
              </select>
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
                Username
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Email
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Role
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            {users.map((user) => (
              <tr key={user.id}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">
                  {user.username}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  {user.email ?? "-"}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <span className={`px-2 py-1 text-xs rounded ${
                    user.role === 'admin'
                      ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                      : 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                  }`}>
                    {user.role}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <span className={`px-2 py-1 text-xs rounded ${
                    user.is_active
                      ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                      : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                  }`}>
                    {user.is_active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleToggleUser(user.id, user.is_active, user.username)}
                      className={`transition-colors ${
                        user.is_active
                          ? 'text-warning-500 hover:text-warning-400'
                          : 'text-success-500 hover:text-success-400'
                      }`}
                      title={user.is_active ? 'Deactivate user' : 'Activate user'}
                    >
                      {user.is_active ? <UserX className="h-4 w-4" /> : <UserCheck className="h-4 w-4" />}
                    </button>
                    {!user.is_active && (
                      <button
                        onClick={() => handleDeleteUser(user.id, user.username)}
                        className="text-danger-500 hover:text-danger-400 transition-colors"
                        title="Delete user"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    )}
                    <button
                      onClick={() => {
                        const newPassword = prompt("Enter new password:");
                        if (newPassword) {
                          handleResetPassword(user.id, newPassword);
                        }
                      }}
                      className="text-info-500 hover:text-info-400 transition-colors"
                      title="Reset password"
                    >
                      <Key className="h-4 w-4" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmModal
        isOpen={confirmModal.isOpen}
        onClose={() => setConfirmModal({ isOpen: false, action: null, userId: null, userName: null })}
        onConfirm={handleConfirmAction}
        title={(() => {
          if (confirmModal.action === 'activate') return 'Activate User';
          if (confirmModal.action === 'deactivate') return 'Deactivate User';
          return 'Delete User';
        })()}
        message={(() => {
          if (confirmModal.action === 'activate') {
            return `Are you sure you want to activate the user "${confirmModal.userName}"?`;
          }
          if (confirmModal.action === 'deactivate') {
            return `Are you sure you want to deactivate the user "${confirmModal.userName}"? This will prevent them from logging in.`;
          }
          return `Are you sure you want to permanently delete the user "${confirmModal.userName}"? This action cannot be undone.`;
        })()}
        confirmText={(() => {
          if (confirmModal.action === 'activate') return 'Activate';
          if (confirmModal.action === 'deactivate') return 'Deactivate';
          return 'Delete';
        })()}
        variant={confirmModal.action === 'delete' ? 'danger' : 'warning'}
      />
    </div>
  );
}


