import React from 'react';

interface StatusBadgeProps {
  status: 'online' | 'offline' | 'error' | 'running' | 'stopped' | 'exited' | 'paused' | 'restarting';
  className?: string;
}

const StatusBadge: React.FC<StatusBadgeProps> = ({ status, className = '' }) => {
  const getStatusConfig = () => {
    switch (status) {
      case 'online':
      case 'running':
        return {
          label: status === 'online' ? 'Online' : 'Running',
          className: 'status-badge status-running',
        };
      case 'offline':
      case 'stopped':
        return {
          label: status === 'offline' ? 'Offline' : 'Stopped',
          className: 'status-badge status-stopped',
        };
      case 'error':
        return {
          label: 'Error',
          className: 'status-badge status-error',
        };
      case 'exited':
        return {
          label: 'Exited',
          className: 'status-badge status-exited',
        };
      case 'paused':
        return {
          label: 'Paused',
          className: 'status-badge status-paused',
        };
      case 'restarting':
        return {
          label: 'Restarting',
          className: 'status-badge status-restarting',
        };
      default:
        return {
          label: status,
          className: 'status-badge status-offline',
        };
    }
  };

  const config = getStatusConfig();

  return (
    <span className={`${config.className} ${className}`}>
      {config.label}
    </span>
  );
};

export default StatusBadge;
