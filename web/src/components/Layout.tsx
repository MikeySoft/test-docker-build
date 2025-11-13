import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LogOut, User } from 'lucide-react';
import { useAuthStore } from '../stores/authStore';
import apiClient from '../api/client';
import { useAppStore } from '../stores/appStore';
import { useWebSocket } from '../hooks/useWebSocket';
import FlotillaLogo from './FlotillaLogo';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  const location = useLocation();
  const { isConnected } = useAppStore();
  const { reconnect } = useWebSocket();

  const path = location.pathname;
  const navigation = [
    { name: 'Dashboard', href: '/dashboard', current: path === '/dashboard' },
    { name: 'Hosts', href: '/hosts', current: path === '/hosts' || path.startsWith('/hosts/') },
    { name: 'Containers', href: '/containers', current: path === '/containers' || path.startsWith('/containers/') },
    { name: 'Stacks', href: '/stacks', current: path === '/stacks' || path.startsWith('/stacks/') },
    { name: 'Settings', href: '/settings', current: path.startsWith('/settings') },
  ];

  return (
    <div className="min-h-screen bg-white dark:bg-black">
      {/* Navigation */}
      <nav className="bg-white dark:bg-black border-b border-gray-200 dark:border-gray-900">
        <div className="max-w-7xl mx-auto px-6">
          <div className="flex justify-between h-16">
            <div className="flex">
              <div className="flex-shrink-0 flex items-center gap-3">
                <FlotillaLogo className="h-8 w-8" />
                <h1 className="text-xl font-bold text-gray-900 dark:text-white font-space">flotilla</h1>
              </div>
              <div className="hidden sm:ml-8 sm:flex sm:space-x-1">
                {navigation.map((item) => (
                  <Link
                    key={item.name}
                    to={item.href}
                    className={`${
                      item.current
                        ? 'border-cyan-500 text-gray-900 dark:text-white'
                        : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:border-gray-300 dark:hover:border-gray-700'
                    } inline-flex items-center px-3 py-2 border-b-2 text-sm font-medium font-inter transition-colors duration-200`}
                  >
                    {item.name}
                  </Link>
                ))}
              </div>
            </div>
            <div className="flex items-center gap-4">
              {/* Connection status */}
              <div className="flex items-center gap-2">
                <div
                  className={`w-2 h-2 rounded-full ${
                    isConnected ? 'bg-success-500' : 'bg-gray-400 dark:bg-gray-600'
                  }`}
                />
                {isConnected ? (
                  <span className="text-sm text-gray-600 dark:text-gray-400 font-inter">
                    Connected
                  </span>
                ) : (
                  <button
                    onClick={reconnect}
                    className="text-sm text-gray-600 dark:text-gray-400 font-inter hover:text-blue-600 dark:hover:text-blue-400 transition-colors cursor-pointer"
                    title="Click to reconnect"
                  >
                    Disconnected
                  </button>
                )}
              </div>
              <AuthMenu />
            </div>
          </div>
        </div>
      </nav>

      {/* Main content */}
      <main className="max-w-7xl mx-auto py-6 px-6">
        {children}
      </main>
    </div>
  );
};

export default Layout;

function AuthMenu() {
  const user = useAuthStore((s) => s.user);
  const clear = useAuthStore((s) => s.clear);
  const onLogout = async () => {
    try { await apiClient.logout(); } finally { clear(); }
  };
  if (!user) return null;
  return (
    <div className="flex items-center gap-3">
      {/* User info */}
      <div className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <User className="h-4 w-4" />
        <span className="font-medium">{user.username}</span>
        <span className="text-xs text-gray-500 dark:text-gray-400">({user.role})</span>
      </div>

      {/* Logout button */}
      <button
        onClick={onLogout}
        className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 border border-gray-200 dark:border-gray-700 rounded-md transition-colors duration-200 group"
        title="Logout"
      >
        <LogOut className="h-4 w-4 group-hover:scale-110 transition-transform duration-200" />
        <span className="hidden sm:inline">Logout</span>
      </button>
    </div>
  );
}
