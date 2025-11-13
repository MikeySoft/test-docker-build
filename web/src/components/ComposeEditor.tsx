import React, { useState, useRef, useEffect } from 'react';
import { Editor } from '@monaco-editor/react';
import { Save, X, Plus, Trash2 } from 'lucide-react';

interface ComposeEditorProps {
  initialContent?: string;
  initialEnvVars?: Record<string, string>;
  onSave?: (content: string, envVars: Record<string, string>) => void;
  onCancel?: () => void;
  isLoading?: boolean;
  className?: string;
}

const DEFAULT_COMPOSE_CONTENT = `version: '3.8'

services:
  app:
    image: nginx:latest
    ports:
      - "80:80"
    environment:
      - NGINX_HOST=localhost
    volumes:
      - ./html:/usr/share/nginx/html
    restart: unless-stopped

networks:
  default:
    driver: bridge

volumes:
  data:
    driver: local`;

const ComposeEditor: React.FC<ComposeEditorProps> = ({
  initialContent = '',
  initialEnvVars = {},
  onSave,
  onCancel,
  isLoading = false,
  className = '',
}) => {
  const [content, setContent] = useState(initialContent);
  const [envVars, setEnvVars] = useState<Record<string, string>>(initialEnvVars);
  const [envVarKey, setEnvVarKey] = useState('');
  const [envVarValue, setEnvVarValue] = useState('');
  const [isValid, setIsValid] = useState(true);
  const [validationError, setValidationError] = useState<string | null>(null);
  const editorRef = useRef<any>(null);

  useEffect(() => {
    if (!content.trim()) {
      setContent(DEFAULT_COMPOSE_CONTENT);
    }
  }, [content]);

  const handleEditorDidMount = (editor: any, monaco: any) => {
    editorRef.current = editor;

    // Configure YAML language features if available
    try {
      if (monaco?.languages?.yaml?.yamlDefaults) {
        monaco.languages.yaml.yamlDefaults.setDiagnosticsOptions({
          validate: true,
          enableSchemaRequest: true,
          schemas: [
            {
              uri: 'https://json.schemastore.org/docker-compose.json',
              fileMatch: ['docker-compose.yml', 'docker-compose.yaml'],
            },
          ],
        });
      }
    } catch (error) {
      console.warn('Failed to configure YAML diagnostics:', error);
    }
  };

  const validateCompose = (yamlContent: string): boolean => {
    try {
      // Basic YAML validation - in a real app, you'd use a proper YAML parser
      if (!yamlContent.trim()) {
        setValidationError('Compose file cannot be empty');
        return false;
      }

      // Check for required fields
      if (!yamlContent.includes('version:') && !yamlContent.includes('services:')) {
        setValidationError('Compose file must include version and services');
        return false;
      }

      setValidationError(null);
      return true;
    } catch (error) {
      console.warn('YAML validation error:', error);
      setValidationError('Invalid YAML syntax');
      return false;
    }
  };

  const handleContentChange = (value: string | undefined) => {
    const newContent = value || '';
    setContent(newContent);
    const valid = validateCompose(newContent);
    setIsValid(valid);
  };

  const handleAddEnvVar = () => {
    if (envVarKey.trim() && envVarValue.trim()) {
      setEnvVars(prev => ({
        ...prev,
        [envVarKey.trim()]: envVarValue.trim(),
      }));
      setEnvVarKey('');
      setEnvVarValue('');
    }
  };

  const handleRemoveEnvVar = (key: string) => {
    setEnvVars(prev => {
      const newVars = { ...prev };
      delete newVars[key];
      return newVars;
    });
  };

  const handleSave = () => {
    if (isValid && onSave) {
      onSave(content, envVars);
    }
  };


  return (
    <div className={`bg-white rounded-lg shadow-sm border border-gray-200 ${className}`}>
      {/* Header */}
      <div className="px-6 py-4 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium text-gray-900">Docker Compose Editor</h3>
          <div className="flex items-center space-x-2">
            {!isValid && (
              <span className="text-sm text-danger-600">
                {validationError || 'Invalid compose file'}
              </span>
            )}
            <button
              onClick={onCancel}
              className="btn btn-secondary"
              disabled={isLoading}
            >
              <X className="h-4 w-4 mr-2" />
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="btn btn-primary"
              disabled={!isValid || isLoading}
            >
              {isLoading ? (
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
              ) : (
                <Save className="h-4 w-4 mr-2" />
              )}
              {isLoading ? 'Deploying...' : 'Deploy Stack'}
            </button>
          </div>
        </div>
      </div>

      <div className="flex">
        {/* Editor */}
        <div className="flex-1">
          <div className="border-b border-gray-200">
            <div className="px-6 py-2 bg-gray-50">
              <span className="text-sm font-medium text-gray-700">docker-compose.yml</span>
            </div>
          </div>
          <div className="h-96">
            <Editor
              height="100%"
              defaultLanguage="yaml"
              value={content}
              onChange={handleContentChange}
              onMount={handleEditorDidMount}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 14,
                lineNumbers: 'on',
                wordWrap: 'on',
                automaticLayout: true,
                tabSize: 2,
                insertSpaces: true,
                renderWhitespace: 'selection',
              }}
            />
          </div>
        </div>

        {/* Environment Variables */}
        <div className="w-80 border-l border-gray-200">
          <div className="px-6 py-4 border-b border-gray-200">
            <h4 className="text-sm font-medium text-gray-900">Environment Variables</h4>
            <p className="text-xs text-gray-500 mt-1">
              Add environment variables for your services
            </p>
          </div>

          <div className="p-6 space-y-4">
            {/* Add new env var */}
            <div className="space-y-2">
              <div className="flex space-x-2">
                <input
                  type="text"
                  placeholder="Key"
                  value={envVarKey}
                  onChange={(e) => setEnvVarKey(e.target.value)}
                  className="flex-1 px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
                <input
                  type="text"
                  placeholder="Value"
                  value={envVarValue}
                  onChange={(e) => setEnvVarValue(e.target.value)}
                  className="flex-1 px-3 py-2 text-sm border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
                <button
                  onClick={handleAddEnvVar}
                  disabled={!envVarKey.trim() || !envVarValue.trim()}
                  className="btn btn-primary px-3 py-2"
                  type="button"
                >
                  <Plus className="h-4 w-4" />
                </button>
              </div>
            </div>

            {/* Existing env vars */}
            <div className="space-y-2">
              {Object.entries(envVars).length === 0 ? (
                <p className="text-sm text-gray-500 text-center py-4">
                  No environment variables added
                </p>
              ) : (
                Object.entries(envVars).map(([key, value]) => (
                  <div
                    key={key}
                    className="flex items-center space-x-2 p-2 bg-gray-50 rounded-md"
                  >
                    <span className="text-sm font-mono text-gray-700 flex-1">
                      {key}={value}
                    </span>
                    <button
                      onClick={() => handleRemoveEnvVar(key)}
                      className="text-danger-600 hover:text-danger-800"
                      type="button"
                      aria-label={`Remove environment variable ${key}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="px-6 py-3 bg-gray-50 border-t border-gray-200">
        <div className="flex items-center justify-between text-sm text-gray-500">
          <div>
            Press <kbd className="px-1 py-0.5 bg-gray-200 rounded text-xs" role="text">Ctrl+Enter</kbd> to deploy
          </div>
          <div>
            {isValid ? (
              <span className="text-success-600">✓ Valid YAML</span>
            ) : (
              <span className="text-danger-600">✗ Invalid YAML</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ComposeEditor;
