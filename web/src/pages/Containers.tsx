import React, { useState, lazy, Suspense } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Plus,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useToast } from '../contexts/useToast';
import ContainerList from '../components/ContainerList';
import NewContainerModal from '../components/NewContainerModal';
import QueryFilterInput from '../components/QueryFilterInput';
import ConfirmModal from '../components/ConfirmModal';

// Lazy load heavy components
const LogModal = lazy(() => import('../components/LogModal'));

const Containers: React.FC = () => {
  const { showSuccess, showError } = useToast();
  const [isNewContainerModalOpen, setIsNewContainerModalOpen] = useState(false);
  const [isCreatingContainer, setIsCreatingContainer] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<{
    isOpen: boolean;
    containerId: string;
    hostId: string;
    containerName: string;
  }>({
    isOpen: false,
    containerId: '',
    hostId: '',
    containerName: '',
  });
  const [isRemovingContainer, setIsRemovingContainer] = useState(false);
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

  // Fetch all containers from all hosts
  const {
    data: containers = [],
    isLoading: containersLoading,
    error: containersError,
    refetch: refetchContainers,
  } = useQuery({
    queryKey: ['all-containers', q],
    queryFn: async () => {
      const data = await apiClient.getAllContainers(q || undefined);
      return data;
    },
    refetchInterval: 10000, // Refetch every 10 seconds
  });

  const hostNames = React.useMemo(() => {
    const set = new Set<string>();
    for (const c of containers) {
      if (c.host_name) set.add(c.host_name);
    }
    return Array.from(set).sort();
  }, [containers]);

  const imageNames = React.useMemo(() => {
    const set = new Set<string>();
    for (const c of containers) {
      if ((c as any).image) set.add((c as any).image as string);
      if ((c as any).image_name) set.add((c as any).image_name as string);
    }
    return Array.from(set).sort();
  }, [containers]);

  const handleContainerAction = async (action: 'start' | 'stop' | 'restart', containerId: string, hostId?: string) => {
    if (!hostId) {
      console.error('Host ID is required for container actions');
      return;
    }

    const container = containers.find((c) => c.id === containerId && c.host_id === hostId);
    const containerName = container?.name;

    // Immediate UI refresh - optimistic update
    refetchContainers();

    // Fire-and-forget API call for stop/restart (these can take a long time)
    if (action === 'stop' || action === 'restart') {
      switch (action) {
        case 'stop':
          apiClient.stopContainer(hostId, containerId, containerName)
            .then(() => showSuccess('Container stopped successfully'))
            .catch((error: any) => {
              console.error(`Failed to ${action} container:`, error);
              showError('Failed to stop container');
              refetchContainers();
            });
          break;
        case 'restart':
          apiClient.restartContainer(hostId, containerId, containerName)
            .then(() => showSuccess('Container restarted successfully'))
            .catch((error: any) => {
              console.error(`Failed to ${action} container:`, error);
              showError('Failed to restart container');
              refetchContainers();
            });
          break;
      }
    } else {
      // For start, wait for it to complete (it's usually fast)
      try {
        await apiClient.startContainer(hostId, containerId, containerName);
        refetchContainers();
        showSuccess('Container started successfully');
      } catch (error: any) {
        console.error(`Failed to ${action} container:`, error);
        showError('Failed to start container');
        refetchContainers();
      }
    }
  };

  const handleRemoveContainer = (containerId: string, hostId?: string) => {
    if (!hostId) {
      console.error('Host ID is required for container removal');
      return;
    }

    const container = containers.find(c => c.id === containerId && c.host_id === hostId);
    setConfirmDelete({
      isOpen: true,
      containerId,
      hostId,
      containerName: container?.name ?? 'this container',
    });
  };

  const handleConfirmRemove = async () => {
    if (!confirmDelete.hostId) return;

    setIsRemovingContainer(true);
    try {
      await apiClient.removeContainer(
        confirmDelete.hostId,
        confirmDelete.containerId,
        confirmDelete.containerName
      );
      refetchContainers();
      setConfirmDelete({ isOpen: false, containerId: '', hostId: '', containerName: '' });
      showSuccess('Container removed successfully');
    } catch (error) {
      console.error('Failed to remove container:', error);
      showError('Failed to remove container');
    } finally {
      setIsRemovingContainer(false);
    }
  };

  const handleNewContainer = () => {
    setIsNewContainerModalOpen(true);
  };

  const handleViewLogs = (containerId: string, hostId?: string, containerName?: string, hostName?: string) => {
    if (!hostId) {
      return;
    }

    setLogModal({
      isOpen: true,
      containerId,
      hostId,
      containerName: containerName ?? 'Unknown',
      hostName,
    });
  };

  const handleCloseLogModal = () => {
    setLogModal({
      isOpen: false,
      containerId: '',
      hostId: '',
      containerName: '',
      hostName: undefined,
    });
  };

  const handleCreateContainer = async (hostId: string, payload: any) => {
    setIsCreatingContainer(true);
    try {
      const response = await apiClient.createContainer(hostId, payload);
      setIsNewContainerModalOpen(false);
      refetchContainers();
      showSuccess(`Container "${payload.name ?? 'container'}" created successfully`);
      return response; // Return the response so the modal can access container_id
    } catch (error: any) {
      console.error('Failed to create container:', error);
      showError('Failed to create container');
      throw error;
    } finally {
      setIsCreatingContainer(false);
    }
  };

  const handleContainerCreated = (hostId: string, containerIdOrName: string) => {
    // The containerIdOrName could be either an ID or a name
    // Try to find by ID first, then by name
    setTimeout(() => {
      const container = containers.find(c => (c.id === containerIdOrName || c.name === containerIdOrName) && c.host_id === hostId);
      if (container) {
        setLogModal({
          isOpen: true,
          containerId: container.id,
          hostId: hostId,
          containerName: container.name,
          hostName: container.host_name,
        });
      } else {
        // If container not found in list yet, try to open with the ID we have
        setLogModal({
          isOpen: true,
          containerId: containerIdOrName,
          hostId: hostId,
          containerName: containerIdOrName,
          hostName: undefined,
        });
      }
    }, 500); // Small delay to ensure container list is updated
  };

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-space">Containers</h1>
            <p className="text-gray-600 dark:text-gray-400 font-inter">Manage containers across all connected hosts</p>
          </div>
          <button
            onClick={handleNewContainer}
            className="btn btn-primary"
          >
            <Plus className="h-4 w-4 mr-2" />
            New Container
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
              placeholder="name:nginx status=running host=prod OR image:postgres"
              statuses={["running","exited","paused","restarting","created"]}
              hosts={hostNames}
              images={imageNames}
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

      {/* Container List */}
      <ContainerList
        containers={containers}
        onAction={handleContainerAction}
        onRemove={handleRemoveContainer}
        onViewLogs={handleViewLogs}
        showHost={true}
        isLoading={containersLoading}
        error={containersError}
        onRetry={() => refetchContainers()}
      />

      {/* New Container Modal */}
      <NewContainerModal
        isOpen={isNewContainerModalOpen}
        onClose={() => setIsNewContainerModalOpen(false)}
        onCreate={handleCreateContainer}
        isLoading={isCreatingContainer}
        preselectedHostId={null}
        onContainerCreated={handleContainerCreated}
      />

      {/* Log Modal */}
      <Suspense fallback={null}>
        <LogModal
          isOpen={logModal.isOpen}
          onClose={handleCloseLogModal}
          containerId={logModal.containerId}
          hostId={logModal.hostId}
          containerName={logModal.containerName}
          hostName={logModal.hostName}
        />
      </Suspense>

      {/* Confirm Delete Modal */}
      <ConfirmModal
        isOpen={confirmDelete.isOpen}
        onClose={() => setConfirmDelete({ isOpen: false, containerId: '', hostId: '', containerName: '' })}
        onConfirm={handleConfirmRemove}
        title="Remove Container"
        message={`Are you sure you want to remove "${confirmDelete.containerName}"? This action cannot be undone.`}
        confirmText="Remove Container"
        cancelText="Cancel"
        variant="danger"
        isLoading={isRemovingContainer}
      />
    </div>
  );
};

export default Containers;
