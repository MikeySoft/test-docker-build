import React, { useState, useEffect, lazy, Suspense } from 'react';
import { useParams, Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Server,
  Container as ContainerIcon,
  Layers,
  Image as ImageIcon,
  Network,
  HardDrive,
  Trash2,
  Plus,
  Edit,
  Play,
  Square,
  RotateCw,
  RefreshCw,
  BarChart3,
  ZoomIn,
} from 'lucide-react';
import { apiClient } from '../api/client';
import type {
  Stack,
  CreateContainerPayload,
  CreateContainerResponse,
  Container,
  DockerImage,
  DockerNetwork,
  DockerVolume,
  ResourceRemovalResult,
  ResourceRemovalConflict,
  ResourceRemovalBlocker,
  ResourceRemovalType,
  ResourceRemovalError,
} from '../types';
import { useAppStore } from '../stores/appStore';
import { useToast } from '../contexts/useToast';
import StatusBadge from '../components/StatusBadge';
import Modal from '../components/Modal';
import NewContainerModal from '../components/NewContainerModal';
import ContainerList from '../components/ContainerList';
import ConfirmModal from '../components/ConfirmModal';
import ImportStackModal from '../components/ImportStackModal';
import StackDetailModal from '../components/StackDetailModal';
import { useAuthStore } from '../stores/authStore';
import StackContainersCollapsible from '../components/StackContainersCollapsible';
import QueryFilterInput from '../components/QueryFilterInput';
import ImageList from '../components/ImageList';
import NetworkList from '../components/NetworkList';
import VolumeList from '../components/VolumeList';
import ResourceConflictModal from '../components/ResourceConflictModal';

// Lazy load heavy components
const ComposeEditor = lazy(() => import('../components/ComposeEditor'));
const LogModal = lazy(() => import('../components/LogModal'));
const HostMetricsPanel = lazy(() => import('../components/HostMetricsPanel'));

interface RemovalConflictState {
  type: ResourceRemovalType;
  resourceIds: string[];
  resourceNames?: string[];
  conflicts: ResourceRemovalConflict[];
  errors?: ResourceRemovalResult['errors'];
  retry: (options?: { force?: boolean; targetIds?: string[] }) => Promise<ResourceRemovalResult>;
  onSuccess?: (result: ResourceRemovalResult) => Promise<void> | void;
  isProcessing: boolean;
  processingActionKey?: string | null;
}

type BlockerActionConfig = {
  key: string;
  label: string;
  intent?: 'primary' | 'secondary' | 'danger';
  perform: () => Promise<void>;
};

const HostDetail: React.FC = () => {
  const { hostId } = useParams<{ hostId: string }>();
  const [activeTab, setActiveTab] = useState<'containers' | 'stacks' | 'images' | 'networks' | 'volumes' | 'metrics'>('containers');
  const [isComposeModalOpen, setIsComposeModalOpen] = useState(false);
  const [isImportStackModalOpen, setIsImportStackModalOpen] = useState(false);
  const [prefilledStackName, setPrefilledStackName] = useState<string | null>(null);
  const [isNewContainerModalOpen, setIsNewContainerModalOpen] = useState(false);
  const [editingStack, setEditingStack] = useState<Stack | null>(null);
  const [isDeploying, setIsDeploying] = useState(false);
  const [isCreatingContainer, setIsCreatingContainer] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<{
    isOpen: boolean;
    containerId: string;
    containerName: string;
  }>({
    isOpen: false,
    containerId: '',
    containerName: '',
  });
  const [isRemovingContainer, setIsRemovingContainer] = useState(false);
  const [confirmStackAction, setConfirmStackAction] = useState<{
    isOpen: boolean;
    stackName: string;
    action: 'remove' | 'start' | 'stop' | 'restart';
  }>({
    isOpen: false,
    stackName: '',
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

  const queryClient = useQueryClient();
  const { setSelectedHost, removeHost } = useAppStore();
  const [searchParams, setSearchParams] = useSearchParams();
  const qContainers = searchParams.get('qContainers') ?? '';
  const qStacks = searchParams.get('qStacks') ?? '';
  const qImages = searchParams.get('qImages') ?? '';
  const qNetworks = searchParams.get('qNetworks') ?? '';
  const qVolumes = searchParams.get('qVolumes') ?? '';
  const [filterDraftContainers, setFilterDraftContainers] = useState(qContainers);
  const [filterDraftStacks, setFilterDraftStacks] = useState(qStacks);
  const [filterDraftImages, setFilterDraftImages] = useState(qImages);
  const [filterDraftNetworks, setFilterDraftNetworks] = useState(qNetworks);
  const [filterDraftVolumes, setFilterDraftVolumes] = useState(qVolumes);
  const [selectedImageIds, setSelectedImageIds] = useState<string[]>([]);
  const [imageAction, setImageAction] = useState<{
    isOpen: boolean;
    type: 'single' | 'bulk' | 'dangling';
    images: DockerImage[];
  }>({
    isOpen: false,
    type: 'single',
    images: [],
  });
  const [isDeletingImages, setIsDeletingImages] = useState(false);
  const [removalConflict, setRemovalConflict] = useState<RemovalConflictState | null>(null);
  const [isRefreshingNetworks, setIsRefreshingNetworks] = useState(false);
  const [isRefreshingVolumes, setIsRefreshingVolumes] = useState(false);
  const { showSuccess, showError } = useToast();
  const navigate = useNavigate();
  const [confirmDeleteHost, setConfirmDeleteHost] = useState<{ isOpen: boolean }>(() => ({ isOpen: false }));
  const [isDeletingHost, setIsDeletingHost] = useState(false);
  const user = useAuthStore((s) => s.user)
  const isAdmin = user?.role === 'admin'
  const [revealSecrets, setRevealSecrets] = useState(false)
  const [networkInspectModal, setNetworkInspectModal] = useState<{
    isOpen: boolean;
    isLoading: boolean;
    network: DockerNetwork | null;
    payload: any;
  }>({
    isOpen: false,
    isLoading: false,
    network: null,
    payload: null,
  });
  const [networkRemoveConfirm, setNetworkRemoveConfirm] = useState<{
    isOpen: boolean;
    network: DockerNetwork | null;
    isLoading: boolean;
  }>({
    isOpen: false,
    network: null,
    isLoading: false,
  });
  const [volumeInspectModal, setVolumeInspectModal] = useState<{
    isOpen: boolean;
    isLoading: boolean;
    volume: DockerVolume | null;
    payload: any;
  }>({
    isOpen: false,
    isLoading: false,
    volume: null,
    payload: null,
  });
  const [volumeRemoveConfirm, setVolumeRemoveConfirm] = useState<{
    isOpen: boolean;
    volume: DockerVolume | null;
    isLoading: boolean;
  }>({
    isOpen: false,
    volume: null,
    isLoading: false,
  });

  // Fetch host details
  const {
    data: host,
    isLoading: hostLoading,
    error: hostError,
  } = useQuery({
    queryKey: ['host', hostId],
    queryFn: () => apiClient.getHost(hostId as string),
    enabled: !!hostId,
  });

  // Fetch live docker/host info (version, cores, memory)
  const { data: hostInfo } = useQuery({
    queryKey: ['host-info', hostId],
    queryFn: () => apiClient.getHostInfo(hostId as string),
    enabled: !!hostId,
    refetchInterval: 60000,
  });

  // NOTE: dynamic suggestions are computed after queries below

  // Fetch containers
  const {
    data: containers = [],
    isLoading: containersLoading,
    error: containersError,
    refetch: refetchContainers,
  } = useQuery<Container[]>({
    queryKey: ['containers', hostId, qContainers],
    queryFn: () => apiClient.getContainers(hostId!, qContainers || undefined),
    enabled: !!hostId,
    refetchInterval: 30000, // Refetch every 30 seconds (reduced frequency)
    staleTime: 5000, // Consider data fresh for 5 seconds
  });

  // Fetch stacks
  const {
    data: stacks = [],
    isLoading: stacksLoading,
    error: stacksError,
    refetch: refetchStacks,
  } = useQuery<Stack[]>({
    queryKey: ['stacks', hostId, revealSecrets, qStacks],
    queryFn: () => apiClient.getStacks(hostId!, revealSecrets, qStacks || undefined),
    enabled: !!hostId,
    refetchInterval: 30000, // Refetch every 30 seconds (reduced frequency)
    staleTime: 5000, // Consider data fresh for 5 seconds
  });

  // Fetch images
  const {
    data: images = [],
    isLoading: imagesLoading,
    error: imagesError,
    refetch: refetchImages,
  } = useQuery<DockerImage[]>({
    queryKey: ['images', hostId, qImages],
    queryFn: () => apiClient.getImages(hostId!, qImages || undefined),
    enabled: !!hostId,
    refetchInterval: 60000,
    staleTime: 5000,
  });

  // Fetch networks
  const {
    data: networks = [],
    isLoading: networksLoading,
    error: networksError,
    refetch: refetchNetworks,
  } = useQuery<DockerNetwork[]>({
    queryKey: ['networks', hostId, qNetworks],
    queryFn: () => apiClient.getNetworks(hostId!, qNetworks || undefined),
    enabled: !!hostId,
    refetchInterval: 60000,
    staleTime: 5000,
  });

  // Fetch volumes
  const {
    data: volumes = [],
    isLoading: volumesLoading,
    error: volumesError,
    refetch: refetchVolumes,
  } = useQuery<DockerVolume[]>({
    queryKey: ['volumes', hostId, qVolumes],
    queryFn: () => apiClient.getVolumes(hostId!, qVolumes || undefined),
    enabled: !!hostId,
    refetchInterval: 60000,
    staleTime: 5000,
  });

  const containerImageNames = React.useMemo(() => {
    const s = new Set<string>();
    for (const c of containers) {
      if (c.image) s.add(c.image);
    }
    return Array.from(s).sort((a, b) => a.localeCompare(b));
  }, [containers]);

  const containerStatuses = React.useMemo(() => {
    const s = new Set<string>();
    for (const c of containers) {
      const st = c.status || c.state || '';
      if (st) s.add(st);
    }
    return Array.from(s).sort((a, b) => a.localeCompare(b));
  }, [containers]);

  const stackStatuses = React.useMemo(() => {
    const s = new Set<string>();
    for (const st of stacks) {
      if (st.status) s.add(st.status);
    }
    return Array.from(s).sort((a, b) => a.localeCompare(b));
  }, [stacks]);

  const imageTags = React.useMemo(() => {
    const tags = new Set<string>();
    for (const img of images) {
      for (const tag of img.tags ?? []) {
        tags.add(tag);
      }
    }
    return Array.from(tags).sort((a, b) => a.localeCompare(b));
  }, [images]);

  const networkDrivers = React.useMemo(() => {
    const drivers = new Set<string>();
    for (const network of networks) {
      if (network.driver) drivers.add(network.driver);
    }
    return Array.from(drivers).sort((a, b) => a.localeCompare(b));
  }, [networks]);

  const volumeDrivers = React.useMemo(() => {
    const drivers = new Set<string>();
    for (const volume of volumes) {
      if (volume.driver) drivers.add(volume.driver);
    }
    return Array.from(drivers).sort((a, b) => a.localeCompare(b));
  }, [volumes]);

  // Sort stacks by name
  const sortedStacks = React.useMemo(() => {
    return [...stacks].sort((a, b) => a.name.localeCompare(b.name));
  }, [stacks]);

  const sortedImages = React.useMemo(() => {
    return [...images].sort((a, b) => {
      const aKey = (a.tags && a.tags.length ? a.tags[0] : a.id).toLowerCase();
      const bKey = (b.tags && b.tags.length ? b.tags[0] : b.id).toLowerCase();
      return aKey.localeCompare(bKey);
    });
  }, [images]);

  const sortedNetworks = React.useMemo(() => {
    return [...networks].sort((a, b) => a.name.localeCompare(b.name));
  }, [networks]);

  const sortedVolumes = React.useMemo(() => {
    return [...volumes].sort((a, b) => a.name.localeCompare(b.name));
  }, [volumes]);

  const selectedImageSet = React.useMemo(() => new Set(selectedImageIds), [selectedImageIds]);
  const selectedImages = React.useMemo(() => sortedImages.filter(img => selectedImageSet.has(img.id)), [sortedImages, selectedImageSet]);
  const danglingImages = React.useMemo(() => sortedImages.filter(img => img.dangling), [sortedImages]);
  const hasDanglingImages = danglingImages.length > 0;
  const isImageInUse = React.useCallback((image: DockerImage) => {
    if (typeof image.containers !== 'number') {
      return false;
    }
    if (image.containers < 0) {
      return false;
    }
    return image.containers > 0;
  }, []);
  const deletableSelectedImages = React.useMemo(() => selectedImages.filter((img) => !isImageInUse(img)), [selectedImages, isImageInUse]);
  const hasDeletableSelected = deletableSelectedImages.length > 0;

  React.useEffect(() => {
    const validIds = new Set(sortedImages.map((img) => img.id));
    setSelectedImageIds((prev) => prev.filter((id) => validIds.has(id)));
  }, [sortedImages]);
  const handleToggleImageSelection = (imageId: string, selected: boolean) => {
    setSelectedImageIds((prev) => {
      if (selected) {
        if (prev.includes(imageId)) return prev;
        return [...prev, imageId];
      }
      return prev.filter((id) => id !== imageId);
    });
  };

  const handleToggleSelectAllImages = (selectAll: boolean) => {
    if (selectAll) {
      setSelectedImageIds(sortedImages.map((img) => img.id));
    } else {
      setSelectedImageIds([]);
    }
  };

  const clearImageSelection = () => setSelectedImageIds([]);

  const handleCopyImage = async (image: DockerImage) => {
    const candidate = image.tag ?? image.image ?? image.tags?.[0] ?? image.digests?.[0] ?? image.id;
    if (!candidate || candidate === "<none>:<none>") {
      showError('No registry reference available to copy');
      return;
    }
    try {
      await navigator.clipboard.writeText(candidate);
      showSuccess(`Copied "${candidate}" to clipboard`);
    } catch (err) {
      console.error('Failed to copy image reference', err);
      showError('Unable to copy registry URL');
    }
  };

  const openImageConfirm = (type: 'single' | 'bulk' | 'dangling', images: DockerImage[]) => {
    setImageAction({
      isOpen: true,
      type,
      images,
    });
  };

  const closeImageActionModal = () => {
    if (isDeletingImages) return;
    setImageAction({
      isOpen: false,
      type: 'single',
      images: [],
    });
  };

  const handleRequestDeleteImage = (image: DockerImage) => {
    if (isImageInUse(image)) {
      const count = image.containers ?? 0;
      const displayName = image.tag ?? image.image ?? image.short_id ?? image.id;
      showError(
        count === 1
          ? `Image "${displayName}" is currently used by 1 container. Stop that container before deleting.`
          : `Image "${displayName}" is currently used by ${count} containers. Stop those containers before deleting.`
      );
      return;
    }
    openImageConfirm('single', [image]);
  };

  const handleDeleteSelectedImages = () => {
    if (!selectedImages.length) {
      showError('Select one or more images to delete');
      return;
    }
    if (!hasDeletableSelected) {
      showError('Selected images are currently in use and cannot be deleted');
      return;
    }
    openImageConfirm('bulk', deletableSelectedImages);
  };

  const handleDeleteDanglingImages = () => {
    if (!hasDanglingImages) {
      showError('There are no dangling images to delete');
      return;
    }
    openImageConfirm('dangling', danglingImages);
  };

  const handleInspectNetwork = async (network: DockerNetwork) => {
    if (!hostId) {
      showError('Host not found');
      return;
    }
    setNetworkInspectModal({
      isOpen: true,
      isLoading: true,
      network,
      payload: null,
    });
    try {
      const payload = await apiClient.inspectNetwork(hostId, network.id);
      setNetworkInspectModal({
        isOpen: true,
        isLoading: false,
        network,
        payload,
      });
    } catch (error) {
      console.error('Failed to inspect network:', error);
      showError('Failed to load network details');
      setNetworkInspectModal({
        isOpen: false,
        isLoading: false,
        network: null,
        payload: null,
      });
    }
  };

  const closeNetworkInspectModal = () => {
    setNetworkInspectModal({
      isOpen: false,
      isLoading: false,
      network: null,
      payload: null,
    });
  };

  const requestRemoveNetwork = (network: DockerNetwork) => {
    setNetworkRemoveConfirm({
      isOpen: true,
      network,
      isLoading: false,
    });
  };

  const handleConfirmRemoveNetwork = async () => {
    if (!hostId || !networkRemoveConfirm.network) {
      showError('Unable to remove network');
      return;
    }
    const target = networkRemoveConfirm.network;
    setNetworkRemoveConfirm((prev) => ({
      ...prev,
      isLoading: true,
    }));
    const executeRemoval = (options?: { force?: boolean }) =>
      apiClient.removeNetwork(hostId, target.id, options?.force ?? false);
    try {
      const result = await executeRemoval();
      await processRemovalOutcome('network', [target.id], result, {
        resourceNames: [target.name ?? target.id],
        makeRetry: (options) => executeRemoval({ force: options?.force }),
        onSuccess: async (res) => {
          if ((res.removed?.length ?? 0) > 0) {
            const label = target.name ?? target.id;
            showSuccess(`Removed network "${label}"`);
          }
          await refetchNetworks();
        },
      });
    } catch (error) {
      console.error('Failed to remove network:', error);
      showError(
        error instanceof Error
          ? error.message
          : 'Failed to remove network'
      );
    } finally {
      setNetworkRemoveConfirm({
        isOpen: false,
        network: null,
        isLoading: false,
      });
    }
  };

  const handleInspectVolume = async (volume: DockerVolume) => {
    if (!hostId) {
      showError('Host not found');
      return;
    }
    setVolumeInspectModal({
      isOpen: true,
      isLoading: true,
      volume,
      payload: null,
    });
    try {
      const payload = await apiClient.inspectVolume(hostId, volume.name);
      setVolumeInspectModal({
        isOpen: true,
        isLoading: false,
        volume,
        payload,
      });
    } catch (error) {
      console.error('Failed to inspect volume:', error);
      showError('Failed to load volume details');
      setVolumeInspectModal({
        isOpen: false,
        isLoading: false,
        volume: null,
        payload: null,
      });
    }
  };

  const closeVolumeInspectModal = () => {
    setVolumeInspectModal({
      isOpen: false,
      isLoading: false,
      volume: null,
      payload: null,
    });
  };

  const handleCopyVolumePath = async (volume: DockerVolume) => {
    const path = volume.mountpoint;
    if (!path) {
      showError('Volume mountpoint unavailable');
      return;
    }
    try {
      if (typeof navigator === 'undefined' || !navigator.clipboard) {
        throw new Error('Clipboard access not available');
      }
      await navigator.clipboard.writeText(path);
      showSuccess(`Copied "${path}" to clipboard`);
    } catch (error) {
      console.error('Failed to copy volume path:', error);
      showError('Unable to copy volume path');
    }
  };

  const removalLabels: Record<ResourceRemovalType, { singular: string; plural: string }> = {
    image: { singular: 'image', plural: 'images' },
    volume: { singular: 'volume', plural: 'volumes' },
    network: { singular: 'network', plural: 'networks' },
  };

  const summarizeRemovalErrors = (type: ResourceRemovalType, errors?: ResourceRemovalError[]) => {
    if (!errors || errors.length === 0) {
      return null;
    }
    const uniqueMessages = Array.from(new Set(errors.map((err) => err.message).filter(Boolean)));
    if (uniqueMessages.length === 0) {
      return `An unknown error prevented the ${removalLabels[type].singular} from being removed`;
    }
    if (uniqueMessages.length === 1) {
      return uniqueMessages[0];
    }
    const summary = uniqueMessages.slice(0, 3).join('; ');
    return uniqueMessages.length > 3 ? `${summary}…` : summary;
  };

  const notifyRemovalErrors = (type: ResourceRemovalType, errors?: ResourceRemovalError[]) => {
    const summary = summarizeRemovalErrors(type, errors);
    if (!summary) {
      return;
    }
    showError(`Failed to remove ${removalLabels[type].plural}: ${summary}`);
  };

  const processRemovalOutcome = async (
    type: ResourceRemovalType,
    targetIds: string[],
    result: ResourceRemovalResult,
    options: {
      onSuccess?: (result: ResourceRemovalResult) => Promise<void> | void;
      makeRetry: (options?: { force?: boolean; targetIds?: string[] }) => Promise<ResourceRemovalResult>;
      resourceNames?: string[];
    }
  ): Promise<boolean> => {
    if (options.onSuccess) {
      await options.onSuccess(result);
    }

    if (result.errors && result.errors.length) {
      notifyRemovalErrors(type, result.errors);
    }

    if (result.conflicts && result.conflicts.length) {
      setRemovalConflict({
        type,
        resourceIds: targetIds,
        resourceNames: options.resourceNames,
        conflicts: result.conflicts,
        errors: result.errors,
        retry: options.makeRetry,
        onSuccess: options.onSuccess,
        isProcessing: false,
        processingActionKey: null,
      });
      return false;
    }

    return !(result.errors && result.errors.length);
  };

  const processConflictRetry = async (state: RemovalConflictState, result: ResourceRemovalResult) => {
    if (state.onSuccess) {
      await state.onSuccess(result);
    }

    if (result.errors && result.errors.length) {
      notifyRemovalErrors(state.type, result.errors);
    }

    if (result.conflicts && result.conflicts.length) {
      setRemovalConflict({
        ...state,
        conflicts: result.conflicts,
        errors: result.errors,
        isProcessing: false,
        processingActionKey: null,
      });
      return;
    }

    setRemovalConflict(null);
  };

  const closeRemovalConflict = () => {
    setRemovalConflict(null);
  };

  const handleRetryRemoval = async () => {
    const currentState = removalConflict;
    if (!currentState || currentState.isProcessing) {
      return;
    }
    setRemovalConflict({
      ...currentState,
      isProcessing: true,
      processingActionKey: 'retry',
    });

    try {
      const result = await currentState.retry();
      await processConflictRetry(currentState, result);
    } catch (error) {
      console.error('Failed to retry removal:', error);
      const message = error instanceof Error ? error.message : 'Failed to retry removal';
      showError(message);
      setRemovalConflict((prev) =>
        prev ? { ...prev, isProcessing: false, processingActionKey: null } : prev
      );
    }
  };

  const handleForceRemoval = async () => {
    const currentState = removalConflict;
    if (!currentState || currentState.isProcessing) {
      return;
    }
    setRemovalConflict({
      ...currentState,
      isProcessing: true,
      processingActionKey: 'force',
    });

    try {
      const result = await currentState.retry({ force: true });
      await processConflictRetry(currentState, result);
    } catch (error) {
      console.error('Failed to force removal:', error);
      const message = error instanceof Error ? error.message : 'Failed to force removal';
      showError(message);
      setRemovalConflict((prev) =>
        prev ? { ...prev, isProcessing: false, processingActionKey: null } : prev
      );
    }
  };

  const handleRemoveImageTag = async (tag: string): Promise<void> => {
    if (!hostId) {
      showError('Host ID unavailable');
      return;
    }
    const currentState = removalConflict;
    if (!currentState || currentState.type !== 'image') {
      return;
    }
    const actionKey = `remove-tag-${tag}`;
    setRemovalConflict({
      ...currentState,
      isProcessing: true,
      processingActionKey: actionKey,
    });

    try {
      const tagResult = await apiClient.removeImages(hostId, [tag]);
      if (tagResult.errors && tagResult.errors.length) {
        notifyRemovalErrors('image', tagResult.errors);
      }
      await refetchImages();
      showSuccess(`Removed tag ${tag}`);
      const retryResult = await currentState.retry();
      await processConflictRetry(currentState, retryResult);
    } catch (error) {
      console.error(`Failed to remove image tag ${tag}:`, error);
      const message = error instanceof Error ? error.message : `Failed to remove tag ${tag}`;
      showError(message);
      setRemovalConflict((prev) =>
        prev ? { ...prev, isProcessing: false, processingActionKey: null } : prev
      );
    }
  };

  const getBlockerActions = (blocker: ResourceRemovalBlocker): BlockerActionConfig[] => {
    if (!removalConflict) {
      return [];
    }
    if (removalConflict.type === 'image' && blocker.kind === 'image_tag' && blocker.name) {
      return [
        {
          key: `remove-tag-${blocker.name}`,
          label: 'Remove tag',
          intent: 'danger',
          perform: () => handleRemoveImageTag(blocker.name as string),
        },
      ];
    }
    return [];
  };

  const requestRemoveVolume = (volume: DockerVolume) => {
    setVolumeRemoveConfirm({
      isOpen: true,
      volume,
      isLoading: false,
    });
  };

  const handleConfirmRemoveVolume = async () => {
    if (!hostId || !volumeRemoveConfirm.volume) {
      showError('Unable to remove volume');
      return;
    }
    const target = volumeRemoveConfirm.volume;
    setVolumeRemoveConfirm((prev) => ({
      ...prev,
      isLoading: true,
    }));
    const executeRemoval = (options?: { force?: boolean }) =>
      apiClient.removeVolume(hostId, target.name, options?.force ?? false);

    try {
      const result = await executeRemoval();
      await processRemovalOutcome('volume', [target.name], result, {
        resourceNames: [target.name],
        makeRetry: (options) => executeRemoval({ force: options?.force }),
        onSuccess: async (res) => {
          if ((res.removed?.length ?? 0) > 0) {
            showSuccess(`Removed volume "${target.name}"`);
          }
          await refetchVolumes();
        },
      });
    } catch (error) {
      console.error('Failed to remove volume:', error);
      showError(
        error instanceof Error
          ? error.message
          : 'Failed to remove volume'
      );
    } finally {
      setVolumeRemoveConfirm({
        isOpen: false,
        volume: null,
        isLoading: false,
      });
    }
  };

  const handleConfirmImageDeletion = async () => {
    if (!hostId || !imageAction.isOpen) return;
    setIsDeletingImages(true);
    try {
      if (imageAction.type === 'dangling') {
        const response = await apiClient.pruneDanglingImages(hostId);
        const removedCount = response.removed?.length ?? 0;
        showSuccess(`Deleted ${removedCount} dangling image${removedCount === 1 ? '' : 's'}`);
        const removedSet = new Set(response.removed ?? []);
        setSelectedImageIds((prev) => prev.filter((id) => !removedSet.has(id)));
        await refetchImages();
      } else {
        const ids = imageAction.images.map((img) => img.id);
        const attemptRemoval = (options?: { force?: boolean; targetIds?: string[] }) =>
          apiClient.removeImages(hostId, options?.targetIds ?? ids, options?.force);
        const result = await attemptRemoval();
        await processRemovalOutcome('image', ids, result, {
          resourceNames: imageAction.images.map((img) => img.tags?.[0] ?? img.id),
          makeRetry: attemptRemoval,
          onSuccess: async (res) => {
            const removedSet = new Set(res.removed ?? []);
            const removedCount = removedSet.size;
            if (removedCount > 0) {
              showSuccess(`Deleted ${removedCount} image${removedCount === 1 ? '' : 's'}`);
            }
            if (removedSet.size > 0) {
              setSelectedImageIds((prev) => prev.filter((id) => !removedSet.has(id)));
            }
            await refetchImages();
          },
        });
      }
    } catch (error) {
      console.error('Failed to delete images:', error);
      const message = error instanceof Error ? error.message : 'Failed to delete images';
      showError(message);
    } finally {
      setIsDeletingImages(false);
      closeImageActionModal();
    }
  };

  const handleRefreshNetworks = async () => {
    if (!hostId || isRefreshingNetworks) return;
    setIsRefreshingNetworks(true);
    try {
      await apiClient.refreshNetworks(hostId);
      await refetchNetworks();
      showSuccess('Network topology refreshed');
    } catch (error: unknown) {
      console.error('Failed to refresh network topology:', error);
      const message = error instanceof Error ? error.message : 'Failed to refresh network topology';
      showError(message);
    } finally {
      setIsRefreshingNetworks(false);
    }
  };

  const handleRefreshVolumes = async () => {
    if (!hostId || isRefreshingVolumes) return;
    setIsRefreshingVolumes(true);
    try {
      await apiClient.refreshVolumes(hostId);
      await refetchVolumes();
      showSuccess('Volume topology refreshed');
    } catch (error: unknown) {
      console.error('Failed to refresh volume topology:', error);
      const message = error instanceof Error ? error.message : 'Failed to refresh volume topology';
      showError(message);
    } finally {
      setIsRefreshingVolumes(false);
    }
  };

  useEffect(() => {
    if (host) {
      setSelectedHost(host);
    }
  }, [host, setSelectedHost]);

  useEffect(() => {
    return () => {
      setSelectedHost(null);
    };
  }, [setSelectedHost]);

  // Remove this useEffect as it's causing an infinite loop
  // The containers are already being set by the React Query

  // Remove this useEffect as it's causing an infinite loop
  // The stacks are already being managed by React Query

  const handleContainerAction = async (action: 'start' | 'stop' | 'restart', containerId: string) => {
    if (!hostId) return;

    const container = containers.find((c) => c.id === containerId);
    const containerName = container?.name;

    // Immediate UI refresh - optimistic update
    refetchContainers();

    // Fire-and-forget API call for stop/restart (these can take a long time)
    if (action === 'stop' || action === 'restart') {
      switch (action) {
        case 'stop':
          apiClient.stopContainer(hostId, containerId, containerName)
            .then(() => showSuccess('Container stopped successfully'))
            .catch((error: unknown) => {
              console.error(`Failed to ${action} container:`, error);
              showError('Failed to stop container');
              refetchContainers();
            });
          break;
        case 'restart':
          apiClient.restartContainer(hostId, containerId, containerName)
            .then(() => showSuccess('Container restarted successfully'))
            .catch((error: unknown) => {
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
      } catch (error: unknown) {
        console.error(`Failed to ${action} container:`, error);
        showError('Failed to start container');
        refetchContainers();
      }
    }
  };

  const handleDeleteHost = () => {
    setConfirmDeleteHost({ isOpen: true });
  };

  const confirmDeleteHostHandler = async () => {
    if (!hostId) return;
    setIsDeletingHost(true);
    try {
      await apiClient.deleteHost(hostId);
      removeHost(hostId);
      setSelectedHost(null);

      queryClient.invalidateQueries({ queryKey: ['hosts'] });
      queryClient.cancelQueries({
        predicate: ({ queryKey }) =>
          Array.isArray(queryKey) && queryKey.includes(hostId),
      });
      queryClient.removeQueries({
        predicate: ({ queryKey }) =>
          Array.isArray(queryKey) && queryKey.includes(hostId),
      });

      showSuccess('Host deleted successfully');
      navigate('/hosts', { replace: true });
    } catch (error) {
      console.error('Failed to delete host:', error);
      showError('Failed to delete host');
    } finally {
      setIsDeletingHost(false);
      setConfirmDeleteHost({ isOpen: false });
    }
  };

  const handleRemoveContainer = (containerId: string) => {
    if (!hostId) return;

    const container = containers.find(c => c.id === containerId);
    setConfirmDelete({
      isOpen: true,
      containerId,
      containerName: container?.name ?? 'this container',
    });
  };

  const handleConfirmRemove = async () => {
    if (!hostId) return;

    setIsRemovingContainer(true);
    try {
      await apiClient.removeContainer(hostId, confirmDelete.containerId, confirmDelete.containerName);
      refetchContainers();
      setConfirmDelete({ isOpen: false, containerId: '', containerName: '' });
      showSuccess('Container removed successfully');
    } catch (error) {
      console.error('Failed to remove container:', error);
      showError('Failed to remove container');
    } finally {
      setIsRemovingContainer(false);
    }
  };

  const handleNewStack = () => {
    setEditingStack(null);
    setIsComposeModalOpen(true);
  };

  const handleImportStack = (stackName: string) => {
    setPrefilledStackName(stackName);
    setIsImportStackModalOpen(true);
  };

  const handleEditStack = async (stack: Stack) => {
    // If stack doesn't have compose_content, we need to fetch it
    if (!stack.compose_content && stack.name) {
      try {
        // For now, just use the stack as-is since we don't have a getStack endpoint yet
        // The compose_content should be in the stack object from list_stacks
        setEditingStack(stack);
      } catch (error) {
        console.error('Failed to fetch stack details:', error);
        // Still open the modal with what we have
        setEditingStack(stack);
      }
    } else {
      setEditingStack(stack);
    }
    setIsComposeModalOpen(true);
  };

  const handleNewContainer = () => {
    setIsNewContainerModalOpen(true);
  };

  const handleViewLogs = (containerId: string, containerHostId?: string, containerName?: string, hostName?: string) => {
    // Use current hostId from URL params if not provided from container
    const effectiveHostId = containerHostId ?? hostId;

    if (!effectiveHostId) {
      console.error('Host ID is required for viewing logs');
      return;
    }

    setLogModal({
      isOpen: true,
      containerId,
      hostId: effectiveHostId,
      containerName: containerName ?? 'Unknown',
      hostName: hostName ?? host?.name,
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

  const handleCreateContainer = async (hostIdParam: string, payload: CreateContainerPayload): Promise<CreateContainerResponse> => {
    setIsCreatingContainer(true);
    try {
      const response = await apiClient.createContainer(hostIdParam, payload);
      setIsNewContainerModalOpen(false);
      refetchContainers();
      showSuccess(`Container "${payload.name ?? 'container'}" created successfully`);
      return response; // Return the response so the modal can access container_id
    } catch (error: unknown) {
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
      const container = containers.find(c => c.id === containerIdOrName || c.name === containerIdOrName);
      if (container) {
        setLogModal({
          isOpen: true,
          containerId: container.id,
          hostId: hostId,
          containerName: container.name,
          hostName: host?.name,
        });
      } else {
        // If container not found in list yet, try to open with the ID we have
        setLogModal({
          isOpen: true,
          containerId: containerIdOrName,
          hostId: hostId,
          containerName: containerIdOrName,
          hostName: host?.name,
        });
      }
    }, 500); // Small delay to ensure container list is updated
  };

  const handleDeployStack = async (content: string, envVars: Record<string, string>) => {
    if (!hostId) return;

    setIsDeploying(true);
    try {
      if (editingStack) {
        // Update existing stack - use stack name, not ID
        await apiClient.updateStack(hostId, editingStack.name, {
          compose_content: content,
          env_vars: envVars,
        });
      } else {
        // Deploy new stack
        await apiClient.deployStack(hostId, {
          name: `stack-${Date.now()}`, // In a real app, you'd parse this from the compose file
          compose_content: content,
          env_vars: envVars,
        });
      }
      setIsComposeModalOpen(false);
      setEditingStack(null);
      refetchStacks();
    } catch (error) {
      console.error('Failed to deploy stack:', error);
    } finally {
      setIsDeploying(false);
    }
  };

  const handleRemoveStack = (stackName: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      action: 'remove',
    });
  };

  const handleStartStack = (stackName: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      action: 'start',
    });
  };

  const handleStopStack = (stackName: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      action: 'stop',
    });
  };

  const handleRestartStack = (stackName: string) => {
    setConfirmStackAction({
      isOpen: true,
      stackName,
      action: 'restart',
    });
  };

  const handleViewStackDetails = (stack: Stack) => {
    setSelectedStack(stack);
    setIsStackDetailModalOpen(true);
  };

  const confirmStackActionHandler = async () => {
    if (!hostId) return;

    const { stackName, action } = confirmStackAction;
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
      setConfirmStackAction({ isOpen: false, stackName: '', action: 'remove' });
      refetchStacks();
    } catch (error: unknown) {
      console.error(`Failed to ${action} stack:`, error);
      const msg = error instanceof Error ? error.message : String(error);
      showError(`Failed to ${action} stack: ${msg}`);
    } finally {
      setIsStackActionLoading(false);
    }
  };

  type TabId = 'containers' | 'stacks' | 'metrics' | 'images' | 'networks' | 'volumes';
  type TabDef = { id: TabId; name: string; icon: React.ComponentType<{ className?: string }>; count: number };
  const tabs: TabDef[] = [
    { id: 'containers', name: 'Containers', icon: ContainerIcon, count: containers.length },
    { id: 'stacks', name: 'Stacks', icon: Layers, count: sortedStacks.length },
    { id: 'metrics', name: 'Metrics', icon: BarChart3, count: 0 },
    { id: 'images', name: 'Images', icon: ImageIcon, count: sortedImages.length },
    { id: 'networks', name: 'Networks', icon: Network, count: sortedNetworks.length },
    { id: 'volumes', name: 'Volumes', icon: HardDrive, count: sortedVolumes.length },
  ];

  const getConfirmMessage = (action: 'remove' | 'start' | 'stop' | 'restart', stackName: string): string => {
    switch (action) {
      case 'remove':
        return `Are you sure you want to remove the stack "${stackName}"? All containers will be stopped and removed.`;
      case 'start':
        return `Are you sure you want to start the stack "${stackName}"?`;
      case 'stop':
        return `Are you sure you want to stop the stack "${stackName}"?`;
      case 'restart':
      default:
        return `Are you sure you want to restart the stack "${stackName}"?`;
    }
  };

  if (hostLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div>
        <span className="ml-2 text-gray-400 font-inter">Loading host details...</span>
      </div>
    );
  }

  if (hostError || !host) {
    return (
      <div className="text-center py-12">
        <div className="text-danger-500 mb-4">
          <Server className="h-12 w-12 mx-auto mb-2" />
          <h3 className="text-lg font-semibold font-space">Host not found</h3>
          <p className="text-sm text-gray-400 mt-1 font-inter">
            {hostError?.message ?? 'The requested host could not be found.'}
          </p>
        </div>
        <Link to="/hosts" className="btn btn-primary">
          Back to Hosts
        </Link>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <Link to="/hosts" className="text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors">
                <Server className="h-5 w-5" />
              </Link>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white font-space">{host.name}</h1>
              <StatusBadge status={host.status} />
            </div>
            <div className="text-gray-600 dark:text-gray-400 font-inter">
              {host.description}
              <div className="mt-1 text-sm flex items-center gap-3">
                <span>Agent v{host.agent_version ?? 'unknown'}</span>
                {hostInfo?.docker_version && (
                  <span>Docker {hostInfo.docker_version}</span>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={handleDeleteHost}
              className="btn btn-danger"
            >
              <Trash2 className="h-4 w-4 mr-2" />
              Delete Host
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-900 mb-6">
        <nav className="-mb-px flex gap-8">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`${
                  activeTab === tab.id
                    ? 'border-cyan-500 text-gray-900 dark:text-white'
                    : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:border-gray-300 dark:hover:border-gray-700'
                } whitespace-nowrap py-2 px-1 border-b-2 font-medium text-sm flex items-center gap-2 transition-colors font-inter`}
              >
                <Icon className="h-4 w-4" />
                <span>{tab.name}</span>
                {tab.count > 0 && (
                  <span className="bg-gray-100 dark:bg-gray-900 text-gray-600 dark:text-gray-400 py-0.5 px-2 rounded-full text-xs border border-gray-300 dark:border-gray-800">
                    {tab.count}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      </div>

      {/* Confirm Delete Host */}
      <ConfirmModal
        isOpen={confirmDeleteHost.isOpen}
        title="Delete Host"
        message={`Are you sure you want to delete host "${host.name}"? All stacks tracked in Flotilla for this host will be removed from the database. This does not remove containers on the host.`}
        confirmText={isDeletingHost ? 'Deleting...' : 'Delete'}
        isLoading={isDeletingHost}
        onClose={() => setConfirmDeleteHost({ isOpen: false })}
        onConfirm={() => { void confirmDeleteHostHandler(); }}
      />

      {/* Tab Content */}
      {activeTab === 'containers' && (
        <div>
          <div className="flex items-center justify-between mb-4 gap-3">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Containers</h2>
            <button
              onClick={handleNewContainer}
              className="btn btn-primary"
            >
              <Plus className="h-4 w-4 mr-2" />
              New Container
            </button>
          </div>

          {/* Filter above containers list */}
          <div className="mb-4 w-full">
            <div className="flex items-center gap-2 w-full">
              <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
              <div className="flex-1">
                <QueryFilterInput
                  value={filterDraftContainers}
                  onChange={setFilterDraftContainers}
                  onSubmit={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraftContainers) { p.set('qContainers', filterDraftContainers); } else { p.delete('qContainers'); } return p; })}
                  placeholder="name:api status=running image:nginx"
                  statuses={containerStatuses.length ? containerStatuses : ["running","exited","paused"]}
                  hosts={host ? [host.name] : []}
                  images={containerImageNames}
                />
              </div>
              <button
                onClick={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraftContainers) { p.set('qContainers', filterDraftContainers); } else { p.delete('qContainers'); } return p; })}
                className="btn btn-secondary"
              >
                Filter
              </button>
            </div>
          </div>

          <ContainerList
            containers={containers}
            onAction={(action, containerId) => handleContainerAction(action, containerId)}
            onRemove={(containerId) => handleRemoveContainer(containerId)}
            onViewLogs={handleViewLogs}
            hostId={hostId ?? undefined}
            showHost={false}
            isLoading={containersLoading}
            error={containersError}
            onRetry={() => refetchContainers()}
          />
        </div>
      )}

      {activeTab === 'stacks' && (
        <div>
        <div className="flex items-center justify-between mb-4 gap-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Stacks</h2>
          <div className="flex gap-2 items-center">
            {isAdmin && (
              <button
                onClick={() => setRevealSecrets((v) => !v)}
                className="btn btn-secondary"
                title={revealSecrets ? 'Hide sensitive values' : 'Reveal sensitive values'}
              >
                {revealSecrets ? 'Hide secrets' : 'Reveal secrets'}
              </button>
            )}
            <button
              onClick={() => setIsImportStackModalOpen(true)}
              className="btn btn-secondary"
            >
              <Plus className="h-4 w-4 mr-2" />
              Import Stack
            </button>
            <button
              onClick={handleNewStack}
              className="btn btn-primary"
            >
              <Plus className="h-4 w-4 mr-2" />
              New Stack
            </button>
          </div>
        </div>

        {/* Filter above stacks list */}
        <div className="mb-4 w-full">
          <div className="flex items-center gap-2 w-full">
            <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
            <div className="flex-1">
              <QueryFilterInput
                value={filterDraftStacks}
                onChange={setFilterDraftStacks}
                onSubmit={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraftStacks) { p.set('qStacks', filterDraftStacks); } else { p.delete('qStacks'); } return p; })}
                placeholder="name:web status=running"
                statuses={stackStatuses.length ? stackStatuses : ["running","stopped","error"]}
                hosts={host ? [host.name] : []}
              />
            </div>
            <button
              onClick={() => setSearchParams((prev) => { const p = new URLSearchParams(prev); if (filterDraftStacks) { p.set('qStacks', filterDraftStacks); } else { p.delete('qStacks'); } return p; })}
              className="btn btn-secondary"
            >
              Filter
            </button>
          </div>
        </div>

          {(() => {
            if (stacksLoading) {
              return (
                <div className="flex items-center justify-center h-32">
                  <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500"></div>
                  <span className="ml-2 text-gray-400 font-inter">Loading stacks...</span>
                </div>
              );
            }

            if (stacksError) {
              return (
                <div className="text-center py-8">
                  <p className="text-danger-500 mb-2 font-inter">Failed to load stacks</p>
                  <button onClick={() => refetchStacks()} className="btn btn-secondary">
                    Try Again
                  </button>
                </div>
              );
            }

            if (sortedStacks.length === 0) {
              return (
                <div className="text-center py-8">
                  <Layers className="h-12 w-12 mx-auto text-gray-700 mb-4" />
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2 font-space">No stacks</h3>
                  <p className="text-gray-600 dark:text-gray-400 font-inter">No Docker Compose stacks are deployed on this host.</p>
                </div>
              );
            }

            return (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {sortedStacks.map((stack) => {
                return (
                <div key={stack.name} className="card">
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex-1">
                      <button
                        onClick={() => handleViewStackDetails(stack)}
                        className="text-left text-lg font-semibold text-gray-900 dark:text-white font-space hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
                        title="View stack details"
                      >
                        {stack.name}
                      </button>
                      {stack.compose_content ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-cyan-100 text-cyan-800 dark:bg-cyan-900 dark:text-cyan-200 mt-1">
                          Flotilla Managed
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200 mt-1">
                          External
                        </span>
                      )}
                    </div>
                    <StatusBadge status={stack.status} />
                  </div>
                  <div className="mb-4">
                    <p className="text-sm text-gray-500 font-inter">
                      Created: {stack.created_at ? new Date(stack.created_at).toLocaleString() : 'Unknown'}
                    </p>
                    <StackContainersCollapsible
                      hostId={hostId as string}
                      stackName={stack.name}
                      hostName={stack.host_name}
                      onViewLogs={(containerId, containerName) =>
                        setLogModal({
                          isOpen: true,
                          containerId,
                          hostId: hostId!,
                          containerName,
                          hostName: stack.host_name,
                        })
                      }
                    />
                    {!stack.compose_content && (
                      <p className="text-xs text-yellow-600 dark:text-yellow-400 font-inter mt-1">
                        Import required to manage this stack
                      </p>
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {stack.compose_content ? (
                      <>
                        <button
                          onClick={() => handleEditStack(stack)}
                          className="btn btn-secondary text-sm"
                        >
                          <Edit className="h-4 w-4 mr-1" />
                          Edit
                        </button>
                        {stack.status === 'stopped' ? (
                          <button
                            onClick={() => handleStartStack(stack.name)}
                            className="btn btn-success text-sm"
                          >
                            <Play className="h-4 w-4 mr-1" />
                            Start
                          </button>
                        ) : (
                          <>
                            <button
                              onClick={() => handleStopStack(stack.name)}
                              className="btn btn-warning text-sm"
                            >
                              <Square className="h-4 w-4 mr-1" />
                              Stop
                            </button>
                            <button
                              onClick={() => handleRestartStack(stack.name)}
                              className="btn btn-info text-sm"
                            >
                              <RotateCw className="h-4 w-4 mr-1" />
                              Restart
                            </button>
                          </>
                        )}
                      </>
                    ) : (
                      <button
                        onClick={() => handleImportStack(stack.name)}
                        className="btn btn-primary text-sm"
                      >
                        <Plus className="h-4 w-4 mr-1" />
                        Import
                      </button>
                    )}
                    <button
                      onClick={() => handleRemoveStack(stack.name)}
                      className="btn btn-danger text-sm"
                    >
                      <Trash2 className="h-4 w-4 mr-1" />
                      Remove
                    </button>
                      <button
                        onClick={() => handleViewStackDetails(stack)}
                        className="btn btn-secondary text-sm"
                        title="View stack details"
                      >
                        <ZoomIn className="h-4 w-4 mr-1" />
                        View
                      </button>
                  </div>
                </div>
                );
              })}
            </div>
            );
          })()}
        </div>
      )}

      {activeTab === 'images' && (
        <div>
          <div className="flex flex-wrap items-center justify-between mb-4 gap-3">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Images</h2>
            <div className="flex flex-wrap items-center gap-2">
              <button
                onClick={handleDeleteSelectedImages}
                className="btn btn-danger"
                disabled={!hasDeletableSelected || isDeletingImages}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                Delete Selected
              </button>
              <button
                onClick={handleDeleteDanglingImages}
                className="btn btn-warning"
                disabled={!hasDanglingImages || isDeletingImages}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                Delete Dangling
              </button>
              <button
                onClick={clearImageSelection}
                className="btn btn-secondary"
                disabled={selectedImageIds.length === 0 || isDeletingImages}
              >
                Clear Selection
              </button>
            </div>
            <div className="text-sm text-gray-600 dark:text-gray-300 font-inter">
              {selectedImageIds.length} selected
            </div>
          </div>
          <div className="mb-4 w-full">
            <div className="flex items-center gap-2 w-full">
              <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
              <div className="flex-1">
                <QueryFilterInput
                  value={filterDraftImages}
                  onChange={setFilterDraftImages}
                  onSubmit={() =>
                    setSearchParams((prev) => {
                      const p = new URLSearchParams(prev);
                      if (filterDraftImages) {
                        p.set('qImages', filterDraftImages);
                      } else {
                        p.delete('qImages');
                      }
                      return p;
                    })
                  }
                  placeholder="tag:nginx dangling=false"
                  statuses={['dangling', 'active']}
                  hosts={host ? [host.name] : []}
                  images={imageTags}
                />
              </div>
              <button
                onClick={() =>
                  setSearchParams((prev) => {
                    const p = new URLSearchParams(prev);
                    if (filterDraftImages) {
                      p.set('qImages', filterDraftImages);
                    } else {
                      p.delete('qImages');
                    }
                    return p;
                  })
                }
                className="btn btn-secondary"
              >
                Filter
              </button>
            </div>
          </div>
          <ImageList
            images={sortedImages}
            isLoading={imagesLoading}
            error={imagesError instanceof Error ? imagesError.message : imagesError ? String(imagesError) : null}
            onRetry={() => refetchImages()}
            selectedIds={selectedImageIds}
            onToggleSelect={handleToggleImageSelection}
            onToggleSelectAll={handleToggleSelectAllImages}
            onCopy={handleCopyImage}
            onDelete={handleRequestDeleteImage}
            isBusy={isDeletingImages}
            isDeleteDisabled={isImageInUse}
            deleteDisabledReason={(image) =>
              isImageInUse(image) ? 'Image is currently in use by running containers' : undefined
            }
          />
        </div>
      )}

      {activeTab === 'networks' && (
        <div>
          <div className="flex items-center justify-between mb-4 gap-3">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Networks</h2>
            <button
              onClick={() => { void handleRefreshNetworks(); }}
              className="btn btn-secondary"
              disabled={isRefreshingNetworks || networksLoading}
            >
              <RefreshCw className={`h-4 w-4 mr-2 ${isRefreshingNetworks ? 'animate-spin' : ''}`} />
              {isRefreshingNetworks ? 'Refreshing...' : 'Refresh topology'}
            </button>
          </div>
          <div className="mb-4 w-full">
            <div className="flex items-center gap-2 w-full">
              <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
              <div className="flex-1">
                <QueryFilterInput
                  value={filterDraftNetworks}
                  onChange={setFilterDraftNetworks}
                  onSubmit={() =>
                    setSearchParams((prev) => {
                      const p = new URLSearchParams(prev);
                      if (filterDraftNetworks) {
                        p.set('qNetworks', filterDraftNetworks);
                      } else {
                        p.delete('qNetworks');
                      }
                      return p;
                    })
                  }
                  placeholder="name:frontend driver=bridge"
                  statuses={['internal', 'external']}
                  hosts={host ? [host.name] : []}
                  images={networkDrivers}
                />
              </div>
              <button
                onClick={() =>
                  setSearchParams((prev) => {
                    const p = new URLSearchParams(prev);
                    if (filterDraftNetworks) {
                      p.set('qNetworks', filterDraftNetworks);
                    } else {
                      p.delete('qNetworks');
                    }
                    return p;
                  })
                }
                className="btn btn-secondary"
              >
                Filter
              </button>
            </div>
          </div>
          <NetworkList
            networks={sortedNetworks}
            isLoading={networksLoading}
            error={networksError instanceof Error ? networksError.message : networksError ? String(networksError) : null}
            onRetry={() => refetchNetworks()}
            onInspect={handleInspectNetwork}
            onRemove={requestRemoveNetwork}
          />
        </div>
      )}

      {activeTab === 'volumes' && (
        <div>
          <div className="flex items-center justify-between mb-4 gap-3">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white font-space">Volumes</h2>
            <button
              onClick={() => { void handleRefreshVolumes(); }}
              className="btn btn-secondary"
              disabled={isRefreshingVolumes || volumesLoading}
            >
              <RefreshCw className={`h-4 w-4 mr-2 ${isRefreshingVolumes ? 'animate-spin' : ''}`} />
              {isRefreshingVolumes ? 'Refreshing...' : 'Refresh topology'}
            </button>
          </div>
          <div className="mb-4 w-full">
            <div className="flex items-center gap-2 w-full">
              <span className="text-sm text-gray-700 dark:text-gray-300 font-inter">Filter</span>
              <div className="flex-1">
                <QueryFilterInput
                  value={filterDraftVolumes}
                  onChange={setFilterDraftVolumes}
                  onSubmit={() =>
                    setSearchParams((prev) => {
                      const p = new URLSearchParams(prev);
                      if (filterDraftVolumes) {
                        p.set('qVolumes', filterDraftVolumes);
                      } else {
                        p.delete('qVolumes');
                      }
                      return p;
                    })
                  }
                  placeholder="name:data driver=local"
                  statuses={['dangling', 'in-use']}
                  hosts={host ? [host.name] : []}
                  images={volumeDrivers}
                />
              </div>
              <button
                onClick={() =>
                  setSearchParams((prev) => {
                    const p = new URLSearchParams(prev);
                    if (filterDraftVolumes) {
                      p.set('qVolumes', filterDraftVolumes);
                    } else {
                      p.delete('qVolumes');
                    }
                    return p;
                  })
                }
                className="btn btn-secondary"
              >
                Filter
              </button>
            </div>
          </div>
          <VolumeList
            volumes={sortedVolumes}
            isLoading={volumesLoading}
            error={volumesError instanceof Error ? volumesError.message : volumesError ? String(volumesError) : null}
            onRetry={() => refetchVolumes()}
            onInspect={handleInspectVolume}
            onCopyPath={handleCopyVolumePath}
            onRemove={requestRemoveVolume}
          />
        </div>
      )}

      {activeTab === 'metrics' && (
        <Suspense fallback={<div className="flex items-center justify-center h-64"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div></div>}>
          <HostMetricsPanel hostId={hostId as string} />
        </Suspense>
      )}

      {/* Image actions modal */}
      <ConfirmModal
        isOpen={imageAction.isOpen}
        onClose={closeImageActionModal}
        onConfirm={() => { void handleConfirmImageDeletion(); }}
        title={
          imageAction.type === 'dangling'
            ? 'Delete dangling images'
            : imageAction.type === 'bulk'
              ? `Delete ${imageAction.images.length} selected image${imageAction.images.length === 1 ? '' : 's'}`
              : `Delete image ${imageAction.images[0]?.tag ?? imageAction.images[0]?.image ?? imageAction.images[0]?.short_id ?? imageAction.images[0]?.id}`
        }
        message={
          imageAction.type === 'dangling'
            ? 'Are you sure you want to delete all dangling images on this host? This will remove any untagged image layers and cannot be undone.'
            : imageAction.type === 'bulk'
              ? 'Are you sure you want to delete all selected images? Images that are in use were excluded from this action.'
              : 'Are you sure you want to delete this image from the host? This cannot be undone.'
        }
        confirmText={
          isDeletingImages
            ? 'Deleting...'
            : imageAction.type === 'dangling'
              ? 'Delete Dangling'
              : 'Delete'
        }
        isLoading={isDeletingImages}
        variant="danger"
      />

      <Modal
        isOpen={networkInspectModal.isOpen}
        onClose={closeNetworkInspectModal}
        title={
          networkInspectModal.network
            ? `Network: ${networkInspectModal.network.name}`
            : 'Network Details'
        }
        size="lg"
      >
        {networkInspectModal.isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500"></div>
            <span className="ml-2 text-gray-600 dark:text-gray-400 font-inter">Loading network details...</span>
          </div>
        ) : (
          <pre className="max-h-96 overflow-auto rounded-lg bg-gray-100 dark:bg-gray-900 p-4 text-sm text-gray-800 dark:text-gray-200 font-mono whitespace-pre-wrap">
            {JSON.stringify(networkInspectModal.payload, null, 2)}
          </pre>
        )}
      </Modal>

      <Modal
        isOpen={volumeInspectModal.isOpen}
        onClose={closeVolumeInspectModal}
        title={
          volumeInspectModal.volume
            ? `Volume: ${volumeInspectModal.volume.name}`
            : 'Volume Details'
        }
        size="lg"
      >
        {volumeInspectModal.isLoading ? (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-cyan-500"></div>
            <span className="ml-2 text-gray-600 dark:text-gray-400 font-inter">Loading volume details...</span>
          </div>
        ) : (
          <pre className="max-h-96 overflow-auto rounded-lg bg-gray-100 dark:bg-gray-900 p-4 text-sm text-gray-800 dark:text-gray-200 font-mono whitespace-pre-wrap">
            {JSON.stringify(volumeInspectModal.payload, null, 2)}
          </pre>
        )}
      </Modal>

      <ConfirmModal
        isOpen={networkRemoveConfirm.isOpen}
        title={
          networkRemoveConfirm.network
            ? `Remove network ${networkRemoveConfirm.network.name}?`
            : 'Remove network'
        }
        message="Removing a network detaches any connected containers and cannot be undone. Continue?"
        onClose={() =>
          setNetworkRemoveConfirm({
            isOpen: false,
            network: null,
            isLoading: false,
          })
        }
        onConfirm={() => { void handleConfirmRemoveNetwork(); }}
        isLoading={networkRemoveConfirm.isLoading}
        confirmText={networkRemoveConfirm.isLoading ? 'Removing...' : 'Remove'}
        variant="danger"
      />

      <ConfirmModal
        isOpen={volumeRemoveConfirm.isOpen}
        title={
          volumeRemoveConfirm.volume
            ? `Remove volume ${volumeRemoveConfirm.volume.name}?`
            : 'Remove volume'
        }
        message="This volume may contain important data. Removing it permanently deletes its contents and cannot be undone. Are you sure you want to continue?"
        onClose={() =>
          setVolumeRemoveConfirm({
            isOpen: false,
            volume: null,
            isLoading: false,
          })
        }
        onConfirm={() => { void handleConfirmRemoveVolume(); }}
        isLoading={volumeRemoveConfirm.isLoading}
        confirmText={volumeRemoveConfirm.isLoading ? 'Removing...' : 'Remove'}
        variant="danger"
      />

      {/* Compose Editor Modal */}
      <Modal
        isOpen={isComposeModalOpen}
        onClose={() => {
          setIsComposeModalOpen(false);
          setEditingStack(null);
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
                }}
                className="btn btn-secondary"
              >
                Close
              </button>
            </div>
          ) : (
            <ComposeEditor
              initialContent={editingStack?.compose_content ?? ''}
              initialEnvVars={editingStack?.env_vars ?? {}}
              onSave={handleDeployStack}
              onCancel={() => {
                setIsComposeModalOpen(false);
                setEditingStack(null);
              }}
              isLoading={isDeploying}
            />
          )}
        </Suspense>
      </Modal>

      {/* New Container Modal */}
      <NewContainerModal
        isOpen={isNewContainerModalOpen}
        onClose={() => setIsNewContainerModalOpen(false)}
        onCreate={handleCreateContainer}
        isLoading={isCreatingContainer}
        preselectedHostId={hostId}
        onContainerCreated={handleContainerCreated}
      />

      {/* Import Stack Modal */}
      {hostId && (
        <ImportStackModal
          isOpen={isImportStackModalOpen}
          onClose={() => {
            setIsImportStackModalOpen(false);
            setPrefilledStackName(null);
          }}
          onSuccess={() => refetchStacks()}
          hostId={hostId}
          prefilledStackName={prefilledStackName}
        />
      )}

      {/* Confirm Stack Action Modal */}
      <ConfirmModal
        isOpen={confirmStackAction.isOpen}
        onClose={() => setConfirmStackAction({ isOpen: false, stackName: '', action: 'remove' })}
        onConfirm={confirmStackActionHandler}
        title={`${confirmStackAction.action.charAt(0).toUpperCase() + confirmStackAction.action.slice(1)} Stack`}
        message={getConfirmMessage(confirmStackAction.action, confirmStackAction.stackName)}
        confirmText={confirmStackAction.action.charAt(0).toUpperCase() + confirmStackAction.action.slice(1)}
        isLoading={isStackActionLoading}
        variant={confirmStackAction.action === 'remove' ? 'danger' : undefined}
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
        onClose={() => setConfirmDelete({ isOpen: false, containerId: '', containerName: '' })}
        onConfirm={handleConfirmRemove}
        title="Remove Container"
        message={`Are you sure you want to remove "${confirmDelete.containerName}"? This action cannot be undone.`}
        confirmText="Remove Container"
        cancelText="Cancel"
        variant="danger"
        isLoading={isRemovingContainer}
      />

      {/* Stack Detail Modal */}
      {selectedStack && hostId && (
        <StackDetailModal
          isOpen={isStackDetailModalOpen}
          onClose={() => {
            setIsStackDetailModalOpen(false);
            setSelectedStack(null);
          }}
          stack={selectedStack}
          hostId={hostId}
        />
      )}

      <ResourceConflictModal
        isOpen={removalConflict !== null}
        onClose={closeRemovalConflict}
        resourceType={removalConflict?.type ?? 'image'}
        resourceIds={removalConflict?.resourceIds ?? []}
        resourceNames={removalConflict?.resourceNames}
        conflicts={removalConflict?.conflicts ?? []}
        errors={removalConflict?.errors ?? undefined}
        isProcessing={removalConflict?.isProcessing ?? false}
        processingActionKey={removalConflict?.processingActionKey ?? undefined}
        onRetry={removalConflict ? handleRetryRemoval : undefined}
        onForce={removalConflict ? handleForceRemoval : undefined}
        blockerActions={removalConflict ? getBlockerActions : undefined}
      />
    </div>
  );
};

export default HostDetail;
