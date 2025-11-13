import React, { useState } from 'react';
import { X, Settings } from 'lucide-react';
import LogViewer from './LogViewer';

interface LogModalProps {
  isOpen: boolean;
  onClose: () => void;
  containerId: string;
  hostId: string;
  containerName: string;
  hostName?: string;
}

const LogModal: React.FC<LogModalProps> = ({
  isOpen,
  onClose,
  containerId,
  hostId,
  containerName,
  hostName,
}) => {
  const [showSettings, setShowSettings] = useState(false);
  const [follow, setFollow] = useState(true);
  const [tail, setTail] = useState('100');
  const [timestamps, setTimestamps] = useState(true);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden">
      {/* Backdrop */}
      <button
        className="absolute inset-0 bg-black bg-opacity-50"
        onClick={onClose}
        onKeyDown={(e) => e.key === 'Escape' && onClose()}
        type="button"
        aria-label="Close modal"
      />

      {/* Modal */}
      <div className="relative flex flex-col h-full max-w-7xl mx-auto bg-white">
        {/* Header */}
        <div className="flex items-center justify-between p-4 bg-gray-100 border-b">
          <div className="flex items-center space-x-4">
            <h2 className="text-xl font-semibold text-gray-900">
              Container Logs
            </h2>
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="p-1 text-gray-500 hover:text-gray-700"
              title="Settings"
            >
              <Settings className="h-5 w-5" />
            </button>
          </div>
          <button
            onClick={onClose}
            className="p-1 text-gray-500 hover:text-gray-700"
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        {/* Settings Panel */}
        {showSettings && (
          <div className="p-4 bg-gray-50 border-b">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label htmlFor="follow-logs" className="block text-sm font-medium text-gray-700 mb-1">
                  Follow Logs
                </label>
                <select
                  id="follow-logs"
                  value={follow ? 'true' : 'false'}
                  onChange={(e) => setFollow(e.target.value === 'true')}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="true">Yes (Real-time)</option>
                  <option value="false">No (Static)</option>
                </select>
              </div>
              <div>
                <label htmlFor="tail-lines" className="block text-sm font-medium text-gray-700 mb-1">
                  Tail Lines
                </label>
                <select
                  id="tail-lines"
                  value={tail}
                  onChange={(e) => setTail(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="10">Last 10 lines</option>
                  <option value="50">Last 50 lines</option>
                  <option value="100">Last 100 lines</option>
                  <option value="500">Last 500 lines</option>
                  <option value="1000">Last 1000 lines</option>
                  <option value="all">All lines</option>
                </select>
              </div>
              <div>
                <label htmlFor="timestamps" className="block text-sm font-medium text-gray-700 mb-1">
                  Timestamps
                </label>
                <select
                  id="timestamps"
                  value={timestamps ? 'true' : 'false'}
                  onChange={(e) => setTimestamps(e.target.value === 'true')}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="true">Show</option>
                  <option value="false">Hide</option>
                </select>
              </div>
            </div>
          </div>
        )}

        {/* Log Viewer */}
        <div className="flex-1 min-h-0">
          <LogViewer
            containerId={containerId}
            hostId={hostId}
            containerName={containerName}
            hostName={hostName}
            onClose={onClose}
            follow={follow}
            tail={tail}
            timestamps={timestamps}
            showHeader={false}
          />
        </div>
      </div>
    </div>
  );
};

export default LogModal;
