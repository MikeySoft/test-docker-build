import React, { useState, useEffect, useRef } from 'react';
import { X, Plus, Minus, Server } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../api/client';
import type { CreateContainerPayload, CreateContainerResponse, Host } from '../types';

interface NewContainerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (hostId: string, payload: CreateContainerPayload) => Promise<CreateContainerResponse>;
  isLoading?: boolean;
  preselectedHostId?: string | null;
  onContainerCreated?: (hostId: string, containerName: string) => void;
}

const NewContainerModal: React.FC<NewContainerModalProps> = ({
  isOpen,
  onClose,
  onCreate,
  isLoading = false,
  preselectedHostId = null,
  onContainerCreated,
}) => {
  const [selectedHostId, setSelectedHostId] = useState<string | null>(null);
  const [followLogs, setFollowLogs] = useState(true);

  const [formData, setFormData] = useState<CreateContainerPayload>({
    name: '',
    image: '',
    command: '',
    env: [],
    ports: {},
    volumes: [],
    labels: {},
    restart: 'no',
    auto_start: true,
  });

  const [envInput, setEnvInput] = useState('');
  const [portInput, setPortInput] = useState({ container: '', host: '' });
  const [volumeInput, setVolumeInput] = useState('');
  const [labelInput, setLabelInput] = useState({ key: '', value: '' });

  // Fetch hosts for selection
  const {
    data: hosts = [],
    isLoading: hostsLoading,
  } = useQuery({
    queryKey: ['hosts'],
    queryFn: () => apiClient.getHosts(),
    enabled: isOpen, // Only fetch when modal is open
  });

  // Initialize host selection only once when modal opens
  const hasInitializedRef = useRef(false);

  useEffect(() => {
    if (isOpen && !hasInitializedRef.current) {
      setSelectedHostId(preselectedHostId);
      hasInitializedRef.current = true;
    } else if (!isOpen) {
      hasInitializedRef.current = false;
    }
  }, [isOpen, preselectedHostId]);

  const handleClose = () => {
    // Reset form when closing
    setFormData({
      name: '',
      image: '',
      command: '',
      env: [],
      ports: {},
      volumes: [],
      labels: {},
      restart: 'no',
      auto_start: true,
    });
    setEnvInput('');
    setPortInput({ container: '', host: '' });
    setVolumeInput('');
    setLabelInput({ key: '', value: '' });
    setSelectedHostId(null);
    setFollowLogs(true); // Reset to default value
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedHostId) {
      alert('Please select a host');
      return;
    }
    const response = await onCreate(selectedHostId, formData);

    // If follow logs is enabled, notify parent to open logs modal
    if (followLogs && onContainerCreated && response) {
      // Use container_id from response if available, otherwise fall back to name
      const containerId = response.container_id || formData.name;
      onContainerCreated(selectedHostId, containerId);
    }
  };

  const handleReset = () => {
    setFormData({
      name: '',
      image: '',
      command: '',
      env: [],
      ports: {},
      volumes: [],
      labels: {},
      restart: 'no',
      auto_start: true,
    });
    setEnvInput('');
    setPortInput({ container: '', host: '' });
    setVolumeInput('');
    setLabelInput({ key: '', value: '' });
    setSelectedHostId(preselectedHostId);
  };

  const addEnvVar = () => {
    if (envInput.trim()) {
      setFormData(prev => ({
        ...prev,
        env: [...(prev.env || []), envInput.trim()]
      }));
      setEnvInput('');
    }
  };

  const removeEnvVar = (index: number) => {
    setFormData(prev => ({
      ...prev,
      env: (prev.env || []).filter((_, i) => i !== index)
    }));
  };

  const addPort = () => {
    if (portInput.container && portInput.host) {
      setFormData(prev => ({
        ...prev,
        ports: {
          ...prev.ports,
          [portInput.container]: parseInt(portInput.host)
        }
      }));
      setPortInput({ container: '', host: '' });
    }
  };

  const removePort = (containerPort: string) => {
    setFormData(prev => {
      const newPorts = { ...prev.ports };
      delete newPorts[containerPort];
      return { ...prev, ports: newPorts };
    });
  };

  const addVolume = () => {
    if (volumeInput.trim()) {
      setFormData(prev => ({
        ...prev,
        volumes: [...(prev.volumes || []), volumeInput.trim()]
      }));
      setVolumeInput('');
    }
  };

  const removeVolume = (index: number) => {
    setFormData(prev => ({
      ...prev,
      volumes: (prev.volumes || []).filter((_, i) => i !== index)
    }));
  };

  const addLabel = () => {
    if (labelInput.key && labelInput.value) {
      setFormData(prev => ({
        ...prev,
        labels: {
          ...prev.labels,
          [labelInput.key]: labelInput.value
        }
      }));
      setLabelInput({ key: '', value: '' });
    }
  };

  const removeLabel = (key: string) => {
    setFormData(prev => {
      const newLabels = { ...prev.labels };
      delete newLabels[key];
      return { ...prev, labels: newLabels };
    });
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-80 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-950 border border-gray-200 dark:border-gray-900 rounded-xl shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-900">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white font-space">Create New Container</h2>
          <button
            onClick={handleClose}
                className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-white transition-colors"
            disabled={isLoading}
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {/* Host Selection */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Host Selection</h3>

            <div>
              <label htmlFor="host-select" className="block text-sm font-medium text-gray-700 dark:text-gray-400 mb-1 font-inter">
                Host *
              </label>
              <div className="relative">
                <select
                  id="host-select"
                  value={selectedHostId ?? ''}
                  onChange={(e) => setSelectedHostId(e.target.value || null)}
                  className="w-full px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500"
                  required
                  disabled={isLoading || hostsLoading}
                >
                  <option value="">Select a host...</option>
                  {hosts.map((host: Host) => (
                    <option key={host.id} value={host.id}>
                      {host.name}
                    </option>
                  ))}
                </select>
                <Server className="absolute right-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400 pointer-events-none" />
              </div>
            </div>
          </div>

          {/* Basic Configuration */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Basic Configuration</h3>

            <div>
              <label htmlFor="container-name" className="block text-sm font-medium text-gray-700 dark:text-gray-400 mb-1 font-inter">
                Container Name
              </label>
              <input
                id="container-name"
                type="text"
                value={formData.name}
                onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
                className="w-full px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="my-container (optional)"
                disabled={isLoading}
              />
            </div>

            <div>
              <label htmlFor="container-image" className="block text-sm font-medium text-gray-700 dark:text-gray-400 mb-1 font-inter">
                Image *
              </label>
              <input
                id="container-image"
                type="text"
                value={formData.image}
                onChange={(e) => setFormData(prev => ({ ...prev, image: e.target.value }))}
                className="w-full px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="nginx:latest"
                required
                disabled={isLoading}
              />
            </div>

            <div>
              <label htmlFor="container-command" className="block text-sm font-medium text-gray-700 dark:text-gray-400 mb-1 font-inter">
                Command
              </label>
              <input
                id="container-command"
                type="text"
                value={formData.command}
                onChange={(e) => setFormData(prev => ({ ...prev, command: e.target.value }))}
                className="w-full px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="nginx -g 'daemon off;'"
                disabled={isLoading}
              />
            </div>

            <div>
              <label htmlFor="restart-policy" className="block text-sm font-medium text-gray-700 dark:text-gray-400 mb-1 font-inter">
                Restart Policy
              </label>
              <select
                id="restart-policy"
                value={formData.restart}
                onChange={(e) => setFormData(prev => ({ ...prev, restart: e.target.value as "no" | "on-failure" | "always" | "unless-stopped" }))}
                className="w-full px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500"
                disabled={isLoading}
              >
                <option value="no" className="bg-white dark:bg-black text-gray-900 dark:text-white">No</option>
                <option value="on-failure" className="bg-white dark:bg-black text-gray-900 dark:text-white">On Failure</option>
                <option value="always" className="bg-white dark:bg-black text-gray-900 dark:text-white">Always</option>
                <option value="unless-stopped" className="bg-white dark:bg-black text-gray-900 dark:text-white">Unless Stopped</option>
              </select>
            </div>

            <div className="flex items-center">
              <input
                type="checkbox"
                id="auto_start"
                checked={formData.auto_start}
                onChange={(e) => setFormData(prev => ({ ...prev, auto_start: e.target.checked }))}
                className="h-4 w-4 text-cyan-500 focus:ring-cyan-500 border-gray-300 dark:border-gray-700 rounded"
                disabled={isLoading}
              />
              <label htmlFor="auto_start" className="ml-2 block text-sm text-gray-700 dark:text-gray-400 font-inter">
                Start container immediately after creation
              </label>
            </div>
          </div>

          {/* Environment Variables */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Environment Variables</h3>

            <div className="flex space-x-2">
              <input
                type="text"
                value={envInput}
                onChange={(e) => setEnvInput(e.target.value)}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="KEY=value"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={addEnvVar}
                className="btn btn-primary disabled:opacity-50"
                disabled={isLoading}
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>

            {(formData.env || []).length > 0 && (
              <div className="space-y-2">
                {(formData.env || []).map((env, index) => (
                  <div key={`env-${index}-${env}`} className="flex items-center justify-between bg-gray-100 dark:bg-gray-900 px-3 py-2 rounded-md">
                    <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">{env}</span>
                    <button
                      type="button"
                      onClick={() => removeEnvVar(index)}
                      className="text-red-600 hover:text-red-800"
                      disabled={isLoading}
                    >
                      <Minus className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Port Mappings */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Port Mappings</h3>

            <div className="flex space-x-2">
              <input
                type="text"
                value={portInput.container}
                onChange={(e) => setPortInput(prev => ({ ...prev, container: e.target.value }))}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="Container Port (e.g., 80)"
                disabled={isLoading}
              />
              <input
                type="number"
                value={portInput.host}
                onChange={(e) => setPortInput(prev => ({ ...prev, host: e.target.value }))}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="Host Port (e.g., 8080)"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={addPort}
                className="btn btn-primary disabled:opacity-50"
                disabled={isLoading}
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>

            {Object.keys(formData.ports || {}).length > 0 && (
              <div className="space-y-2">
                {Object.entries(formData.ports || {}).map(([containerPort, hostPort]) => (
                  <div key={containerPort} className="flex items-center justify-between bg-gray-100 dark:bg-gray-900 px-3 py-2 rounded-md">
                    <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">{containerPort} → {hostPort}</span>
                    <button
                      type="button"
                      onClick={() => removePort(containerPort)}
                      className="text-red-600 hover:text-red-800"
                      disabled={isLoading}
                    >
                      <Minus className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Volume Mappings */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Volume Mappings</h3>

            <div className="flex space-x-2">
              <input
                type="text"
                value={volumeInput}
                onChange={(e) => setVolumeInput(e.target.value)}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="/host/path:/container/path"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={addVolume}
                className="btn btn-primary disabled:opacity-50"
                disabled={isLoading}
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>

            {(formData.volumes || []).length > 0 && (
              <div className="space-y-2">
                {(formData.volumes || []).map((volume, index) => (
                  <div key={`volume-${index}-${volume}`} className="flex items-center justify-between bg-gray-100 dark:bg-gray-900 px-3 py-2 rounded-md">
                    <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">{volume}</span>
                    <button
                      type="button"
                      onClick={() => removeVolume(index)}
                      className="text-red-600 hover:text-red-800"
                      disabled={isLoading}
                    >
                      <Minus className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Labels */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Labels</h3>

            <div className="flex space-x-2">
              <input
                type="text"
                value={labelInput.key}
                onChange={(e) => setLabelInput(prev => ({ ...prev, key: e.target.value }))}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="Key"
                disabled={isLoading}
              />
              <input
                type="text"
                value={labelInput.value}
                onChange={(e) => setLabelInput(prev => ({ ...prev, value: e.target.value }))}
                className="flex-1 px-3 py-2 bg-white dark:bg-black border border-gray-300 dark:border-gray-900 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-2 focus:ring-cyan-500 placeholder-gray-500"
                placeholder="Value"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={addLabel}
                className="btn btn-primary disabled:opacity-50"
                disabled={isLoading}
              >
                <Plus className="h-4 w-4" />
              </button>
            </div>

            {Object.keys(formData.labels || {}).length > 0 && (
              <div className="space-y-2">
                {Object.entries(formData.labels || {}).map(([key, value]) => (
                  <div key={key} className="flex items-center justify-between bg-gray-100 dark:bg-gray-900 px-3 py-2 rounded-md">
                    <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">{key}: {value}</span>
                    <button
                      type="button"
                      onClick={() => removeLabel(key)}
                      className="text-red-600 hover:text-red-800"
                      disabled={isLoading}
                    >
                      <Minus className="h-4 w-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Follow Logs Option */}
          <div className="pt-4 border-t border-gray-200 dark:border-gray-900">
            <label className="flex items-center space-x-2 cursor-pointer">
              <input
                type="checkbox"
                checked={followLogs}
                onChange={(e) => setFollowLogs(e.target.checked)}
                className="w-4 h-4 text-cyan-500 focus:ring-cyan-500 border-gray-300 dark:border-gray-700 rounded"
                disabled={isLoading}
              />
              <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">
                Open and follow logs after creation
              </span>
            </label>
          </div>

          {/* Actions */}
          <div className="flex justify-end space-x-3 pt-6 border-t border-gray-200 dark:border-gray-900">
            <button
              type="button"
              onClick={handleReset}
              className="btn btn-secondary disabled:opacity-50"
              disabled={isLoading}
            >
              Reset
            </button>
            <button
              type="button"
              onClick={handleClose}
              className="btn btn-secondary disabled:opacity-50"
              disabled={isLoading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary disabled:opacity-50"
              disabled={isLoading}
            >
              {isLoading ? 'Creating...' : 'Create Container'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default NewContainerModal;
