import React, { useState, lazy, Suspense } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { apiClient } from '../api/client';
import { useToast } from '../contexts/useToast';
import StackList from '../components/StackList';
import ConfirmModal from '../components/ConfirmModal';
import ImportStackModal from '../components/ImportStackModal';
import StackDetailModal from '../components/StackDetailModal';
import Modal from '../components/Modal';
import type { Stack } from '../types';
import QueryFilterInput from '../components/QueryFilterInput';

// Lazy load heavy components
const ComposeEditor = lazy(() => import('../components/ComposeEditor'));
const LogModal = lazy(() => import('../components/LogModal'));

type EditableStack = Partial<Stack> & { host_id?: string };

const Stacks: React.FC = () => {
  const { showSuccess, showError } = useToast();
  const [isComposeModalOpen, setIsComposeModalOpen] = useState(false);
  const [isImportStackModalOpen, setIsImportStackModalOpen] = useState(false);
  const [selectedHostId, setSelectedHostId] = useState<string | null>(null);
  const [prefilledStackName, setPrefilledStackName] = useState<string | null>(null);
  const [editingStack, setEditingStack] = useState<EditableStack | null>(null);
  const [isDeploying, setIsDeploying] = useState(false);
  const [confirmStackAction, setConfirmStackAction] = useState<{
    isOpen: boolean;
    stackName: string;
    hostId: string;
    action: 'remove' | 'start' | 'stop' | 'restart';
  }>({
    isOpen: false,
    stackName: '',
    hostId: '',
    action: 'remove',
  });
  const [isStackActionLoading, setIsStackActionLoading] = useState(false);
  const [selectedStack, setSelectedStack] = useState<Stack | null>(null);
  const [isStackDetailModalOpen, setIsStackDetailModalOpen] = useState(false);
  const [logModal, setLogModal] = useState<{
    isOpen: boolean;
    containerId: string;
    hostId: string;
    containerName: string;
    hostName?: string;
  }>({
    isOpen: false,
    containerId: '',
    hostId: '',
    containerName: '',
    hostName: undefined,
  });

  const [searchParams, setSearchParams] = useSearchParams();
  const q = searchParams.get('q') || '';
  const [filterDraft, setFilterDraft] = useState(q);

  // Fetch all stacks from all hosts
  const {
    data: stacks = [],
    isLoading: stacksLoading,
    error: stacksError,
    refetch: refetchStacks,
  } = useQuery({
    queryKey: ['all-stacks', q],
    queryFn: async () => {
      const data = await apiClient.getAllStacks(q || undefined);
      return data;
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });

  // Fetch hosts for selection
  const {
    data: hosts = [],
    isLoading: hostsLoading,
  } = useQuery({
    queryKey: ['hosts'],
    queryFn: () => apiClient.getHosts(),
  });

  const hostNames = React.useMemo(() => hosts.map(h => h.name).sort(), [hosts]);
  const stackStatuses = React.useMemo(() => {
    const set = new Set<string>();
    for (const s of stacks) {
      if ((s as any).status) set.add((s as any).status as string);
    }
    return Array.from(set).sort();
  }, [stacks]);

  // Sort stacks by host name, then stack name
  const sortedStacks = React.useMemo(() => {
    return [...stacks].sort((a, b) => {
      const hostCompare = (a.host_name ?? '').localeCompare(b.host_name ?? '');
      if (hostCompare !== 0) return hostCompare;
      return a.name.localeCompare(b.name);
    });
  }, [stacks]);

  const handleStackAction = async (action: 'start' | 'stop' | 'restart', stackName: string, hostId: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      hostId,
      action,
    });
  };

  const handleRemoveStack = (stackName: string, hostId: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      hostId,
      action: 'remove',
    });
  };

  const handleEditStack = (stack: EditableStack) => {
    setEditingStack(stack);
    setSelectedHostId(stack.host_id ?? null);
    setIsComposeModalOpen(true);
  };

  const handleImportStack = (stackName: string, hostId: string) => {
    setPrefilledStackName(stackName);
    setSelectedHostId(hostId);
    setIsImportStackModalOpen(true);
  };

  const handleViewStackDetails = (stack: Stack, hostId: string) => {
    setSelectedStack({ ...stack, host_id: hostId });
    setIsStackDetailModalOpen(true);
  };

  const handleNewStack = () => {
    setEditingStack(null);
    setSelectedHostId(null);
    setIsComposeModalOpen(true);
  };

  const errorToMessage = (e: unknown): string => {
    if (e instanceof Error) return e.message;
    try {
      return JSON.stringify(e);
    } catch {
      return String(e);
    }
  };

  const confirmStackActionHandler = async () => {
    const { stackName, hostId, action } = confirmStackAction;
    setIsStackActionLoading(true);

    try {
      switch (action) {
        case 'remove':
          await apiClient.removeStack(hostId, stackName);
          showSuccess(`Stack "${stackName}" removed successfully`);
          break;
        case 'start':
          await apiClient.startStack(hostId, stackName);
          showSuccess(`Stack "${stackName}" started successfully`);
          break;
        case 'stop':
          await apiClient.stopStack(hostId, stackName);
          showSuccess(`Stack "${stackName}" stopped successfully`);
          break;
        case 'restart':
          await apiClient.restartStack(hostId, stackName);
          showSuccess(`Stack "${stackName}" restarted successfully`);
          break;
      }
      setConfirmStackAction({ isOpen: false, stackName: '', hostId: '', action: 'remove' });
      refetchStacks();
    } catch (error: unknown) {
      console.error(`Failed to ${action} stack:`, error);
      showError(`Failed to ${action} stack: ${errorToMessage(error)}`);
    } finally {
      setIsStackActionLoading(false);
    }
  };

  const handleDeployStack = async (hostIdParam: string, content: string, envVars: Record<string, string>) => {
    if (!hostIdParam) {
      showError('Please select a host');
      return;
    }

    setIsDeploying(true);
    try {
      if (editingStack) {
        // Update existing stack
        if (!editingStack.name) {
          showError('Missing stack name to update');
          return;
        }
        await apiClient.updateStack(hostIdParam, editingStack.name, {
          compose_content: content,
          env_vars: envVars,
        });
        showSuccess(`Stack "${editingStack.name}" updated successfully`);
      } else {
        // Deploy new stack
        await apiClient.deployStack(hostIdParam, {
          name: `stack-${Date.now()}`,
          compose_content: content,
          env_vars: envVars,
        });
        showSuccess('Stack deployed successfully');
      }
      setIsComposeModalOpen(false);
      setEditingStack(null);
      setSelectedHostId(null);
      refetchStacks();
    } catch (error: unknown) {
      console.error('Failed to deploy stack:', error);
      showError(`Failed to deploy stack: ${errorToMessage(error)}`);
    } finally {
      setIsDeploying(false);
    }
  };

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-space">Stacks</h1>
            <p className="text-gray-600 dark:text-gray-400 font-inter">Manage Docker Compose stacks across all connected hosts</p>
          </div>
          <button
            onClick={handleNewStack}
            className="btn btn-primary"
          >
            <Plus className="h-4 w-4 mr-2" />
            New Stack
          </button>
        </div>
      </div>

      {/* Filter block above list */}
      <div className="mb-4 w-full">
        <div className="flex items-center gap-2 w-full">
          <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
          <div className="flex-1">
            <QueryFilterInput
              value={filterDraft}
              onChange={setFilterDraft}
              onSubmit={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraft) { p.set('q', filterDraft); } else { p.delete('q'); } return p; })}
              placeholder="name:web status=running host=prod"
              statuses={stackStatuses.length ? stackStatuses : ["running","stopped","error"]}
              hosts={hostNames}
            />
          </div>
          <button
            onClick={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraft) { p.set('q', filterDraft); } else { p.delete('q'); } return p; })}
            className="btn btn-secondary"
          >
            Filter
          </button>
        </div>
      </div>

      {/* Stack List */}
      <StackList
        stacks={sortedStacks}
        onAction={handleStackAction}
        onRemove={handleRemoveStack}
        onEdit={handleEditStack}
        onImport={handleImportStack}
        onViewDetails={handleViewStackDetails}
        onViewLogs={(containerId, containerName, hostId) =>
          setLogModal({
            isOpen: true,
            containerId,
            hostId,
            containerName,
          })
        }
        showHost={true}
        isLoading={stacksLoading}
        error={stacksError}
        onRetry={() => refetchStacks()}
      />

      {/* Compose Editor Modal */}
      <Modal
        isOpen={isComposeModalOpen}
        onClose={() => {
          setIsComposeModalOpen(false);
          setEditingStack(null);
          setSelectedHostId(null);
        }}
        title={editingStack ? 'Edit Stack' : 'Deploy New Stack'}
        size="xl"
      >
        <Suspense fallback={<div className="flex items-center justify-center p-8">Loading editor...</div>}>
          {editingStack && !editingStack.compose_content ? (
            <div className="p-6 text-center">
              <p className="text-gray-600 dark:text-gray-400 mb-4">
                This stack was deployed outside of Flotilla. To edit it, please redeploy with a new compose file.
              </p>
              <button
                onClick={() => {
                  setIsComposeModalOpen(false);
                  setEditingStack(null);
                  setSelectedHostId(null);
                }}
                className="btn btn-secondary"
              >
                Close
              </button>
            </div>
          ) : (
            <div>
              {!editingStack && (
                <div className="mb-4">
                  <label htmlFor="host-select" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 font-inter">
                    Select Host
                  </label>
                  <select
                    id="host-select"
                    value={selectedHostId ?? ''}
                    onChange={(e) => setSelectedHostId(e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white font-inter"
                    disabled={hostsLoading}
                  >
                    <option value="">Select a host...</option>
                    {hosts.map((host) => (
                      <option key={host.id} value={host.id}>
                        {host.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <ComposeEditor
                initialContent={editingStack?.compose_content ?? ''}
                initialEnvVars={editingStack?.env_vars ?? {}}
                onSave={(content, envVars) => handleDeployStack(selectedHostId!, content, envVars)}
                onCancel={() => {
                  setIsComposeModalOpen(false);
                  setEditingStack(null);
                  setSelectedHostId(null);
                }}
                isLoading={isDeploying}
              />
            </div>
          )}
        </Suspense>
      </Modal>

      {/* Import Stack Modal */}
      {selectedHostId && (
        <ImportStackModal
          isOpen={isImportStackModalOpen}
          onClose={() => {
            setIsImportStackModalOpen(false);
            setPrefilledStackName(null);
            setSelectedHostId(null);
          }}
          onSuccess={() => refetchStacks()}
          hostId={selectedHostId}
          prefilledStackName={prefilledStackName}
        />
      )}

      {/* Confirm Stack Action Modal */}
      {(() => {
        const action = confirmStackAction.action;
        const confirmTitle = action.charAt(0).toUpperCase() + action.slice(1) + ' Stack';
        let confirmMessage = '';
        switch (action) {
          case 'remove':
            confirmMessage = `Are you sure you want to remove the stack "${confirmStackAction.stackName}"? All containers will be stopped and removed.`;
            break;
          case 'start':
            confirmMessage = `Are you sure you want to start the stack "${confirmStackAction.stackName}"?`;
            break;
          case 'stop':
            confirmMessage = `Are you sure you want to stop the stack "${confirmStackAction.stackName}"?`;
            break;
          case 'restart':
          default:
            confirmMessage = `Are you sure you want to restart the stack "${confirmStackAction.stackName}"?`;
            break;
        }
        const confirmText = action.charAt(0).toUpperCase() + action.slice(1);
        const variant = action === 'remove' ? 'danger' : undefined;
        return (
          <ConfirmModal
            isOpen={confirmStackAction.isOpen}
            onClose={() => setConfirmStackAction({ isOpen: false, stackName: '', hostId: '', action: 'remove' })}
            onConfirm={confirmStackActionHandler}
            title={confirmTitle}
            message={confirmMessage}
            confirmText={confirmText}
            isLoading={isStackActionLoading}
            variant={variant}
          />
        );
      })()}

      {/* Stack Detail Modal */}
      {selectedStack && (
        <StackDetailModal
          isOpen={isStackDetailModalOpen}
          onClose={() => {
            setIsStackDetailModalOpen(false);
            setSelectedStack(null);
          }}
          stack={selectedStack}
          hostId={selectedStack.host_id}
        />
      )}

      {/* Log Modal */}
      {logModal.isOpen && (
        <Suspense fallback={<div className="flex items-center justify-center p-8">Loading logs...</div>}>
          <LogModal
            isOpen={logModal.isOpen}
            onClose={() => setLogModal({ isOpen: false, containerId: '', hostId: '', containerName: '' })}
            containerId={logModal.containerId}
            hostId={logModal.hostId}
            containerName={logModal.containerName}
            hostName={logModal.hostName}
          />
        </Suspense>
      )}
    </div>
  );
};

export default Stacks;

