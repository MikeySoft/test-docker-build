import React, { useState } from 'react';
import { X, AlertCircle, Upload } from 'lucide-react';
import apiClient from '../api/client';
import { useToast } from '../contexts/useToast';

interface ImportStackModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  hostId: string;
  prefilledStackName?: string | null;
}

const ImportStackModal: React.FC<ImportStackModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  hostId,
  prefilledStackName,
}) => {
  const [stackName, setStackName] = useState(prefilledStackName || '');
  const [composeContent, setComposeContent] = useState('');
  const [envVars, setEnvVars] = useState<Record<string, string>>({});
  const [importEnvVars, setImportEnvVars] = useState(true);
  const [isImporting, setIsImporting] = useState(false);
  const { showSuccess, showError } = useToast();

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
      const content = e.target?.result as string;
      setComposeContent(content);
    };
    reader.readAsText(file);
  };

  const handleAddEnvVar = () => {
    setEnvVars({ ...envVars, '': '' });
  };

  const handleEnvVarChange = (key: string, value: string) => {
    const newEnvVars = { ...envVars };
    if (key && value) {
      newEnvVars[key] = value;
    }
    setEnvVars(newEnvVars);
  };

  const handleRemoveEnvVar = (key: string) => {
    const newEnvVars = { ...envVars };
    delete newEnvVars[key];
    setEnvVars(newEnvVars);
  };

  const handleImport = async () => {
    if (!stackName.trim()) {
      showError('Stack name is required');
      return;
    }

    if (!composeContent.trim()) {
      showError('Compose file content is required');
      return;
    }

    setIsImporting(true);
    try {
      const payload: any = {
        name: stackName,
        compose: composeContent,
      };

      if (importEnvVars && Object.keys(envVars).length > 0) {
        payload.env_vars = envVars;
      }

      await apiClient.importStack(hostId, payload);
      showSuccess(`Stack "${stackName}" imported successfully`);
      onSuccess();
      handleClose();
    } catch (error: any) {
      console.error('Failed to import stack:', error);
      showError(error.response?.data?.error || 'Failed to import stack');
    } finally {
      setIsImporting(false);
    }
  };

  const handleClose = () => {
    setStackName(prefilledStackName || '');
    setComposeContent('');
    setEnvVars({});
    setImportEnvVars(true);
    onClose();
  };

  // Update stack name when prefilledStackName changes
  React.useEffect(() => {
    if (prefilledStackName) {
      setStackName(prefilledStackName);
    }
  }, [prefilledStackName]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-3xl max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white font-space">
            Import Stack
          </h2>
          <button
            onClick={handleClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {/* Info Box */}
          <div className="mb-6 p-4 bg-cyan-50 dark:bg-cyan-900/20 border border-cyan-200 dark:border-cyan-800 rounded-lg">
            <div className="flex items-start">
              <AlertCircle className="h-5 w-5 text-cyan-600 dark:text-cyan-400 mr-3 mt-0.5" />
              <div className="flex-1">
                <p className="text-sm text-cyan-800 dark:text-cyan-200 font-inter">
                  Import an existing Docker Compose stack to manage it through Flotilla.
                  The stack must already be running on the host.
                </p>
              </div>
            </div>
          </div>

          {/* Stack Name */}
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 font-inter">
              Stack Name
            </label>
            <input
              type="text"
              value={stackName}
              onChange={(e) => setStackName(e.target.value)}
              placeholder="my-stack"
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-cyan-500 focus:border-transparent font-inter"
            />
          </div>

          {/* Compose File Upload */}
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 font-inter">
              Docker Compose File
            </label>
            <div className="flex items-center gap-4 mb-2">
              <label className="flex items-center px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg cursor-pointer transition-colors">
                <Upload className="h-4 w-4 mr-2" />
                <span className="text-sm font-inter">Upload File</span>
                <input
                  type="file"
                  accept=".yml,.yaml"
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>
            </div>
            <textarea
              value={composeContent}
              onChange={(e) => setComposeContent(e.target.value)}
              placeholder="version: '3.8'\nservices:\n  app:\n    image: nginx:latest"
              rows={15}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-cyan-500 focus:border-transparent font-mono text-sm"
            />
          </div>

          {/* Environment Variables Section */}
          <div className="mb-6">
            <div className="flex items-center justify-between mb-3">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 font-inter">
                Environment Variables
              </label>
              <label className="flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={importEnvVars}
                  onChange={(e) => setImportEnvVars(e.target.checked)}
                  className="mr-2"
                />
                <span className="text-sm text-gray-600 dark:text-gray-400 font-inter">
                  Import from containers
                </span>
              </label>
            </div>

            {importEnvVars && (
              <>
                <div className="space-y-2 mb-3">
                  {Object.entries(envVars).map(([key, value], index) => (
                    <div key={index} className="flex gap-2">
                      <input
                        type="text"
                        placeholder="KEY"
                        value={key}
                        onChange={(e) => handleEnvVarChange(key, e.target.value)}
                        className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-cyan-500 focus:border-transparent font-mono text-sm"
                      />
                      <input
                        type="text"
                        placeholder="value"
                        value={value}
                        onChange={(e) => handleEnvVarChange(key, e.target.value)}
                        className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-cyan-500 focus:border-transparent font-mono text-sm"
                      />
                      <button
                        onClick={() => handleRemoveEnvVar(key)}
                        className="px-3 py-2 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                      >
                        <X className="h-4 w-4" />
                      </button>
                    </div>
                  ))}
                </div>
                <button
                  onClick={handleAddEnvVar}
                  className="text-sm text-cyan-600 hover:text-cyan-700 dark:text-cyan-400 dark:hover:text-cyan-300 font-inter"
                >
                  + Add Environment Variable
                </button>
              </>
            )}

            {importEnvVars && (
              <div className="mt-3 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
                <p className="text-xs text-yellow-800 dark:text-yellow-200 font-inter">
                  ⚠️ Imported environment variables will be marked as sensitive. Consider using secrets management in production.
                </p>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-6 border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={handleClose}
            disabled={isImporting}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors font-inter"
          >
            Cancel
          </button>
          <button
            onClick={handleImport}
            disabled={isImporting || !stackName.trim() || !composeContent.trim()}
            className="px-4 py-2 bg-cyan-600 hover:bg-cyan-700 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-inter"
          >
            {isImporting ? 'Importing...' : 'Import Stack'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default ImportStackModal;

