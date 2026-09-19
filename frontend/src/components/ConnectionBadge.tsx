import React from 'react';
import { ConnectionStatus } from '../hooks/usePollWebSocket';

interface ConnectionBadgeProps {
  status: ConnectionStatus;
  viewerCount?: number;
}

export const ConnectionBadge: React.FC<ConnectionBadgeProps> = ({ status, viewerCount }) => {
  const getStatusStyles = () => {
    switch (status) {
      case 'connected':
        return {
          bg: 'bg-emerald-50 text-emerald-700 border-emerald-200',
          dot: 'bg-emerald-500',
          label: 'Live',
        };
      case 'reconnecting':
        return {
          bg: 'bg-amber-50 text-amber-700 border-amber-200',
          dot: 'bg-amber-500 animate-pulse',
          label: 'Reconnecting',
        };
      case 'offline':
      default:
        return {
          bg: 'bg-gray-100 text-gray-600 border-gray-200',
          dot: 'bg-gray-400',
          label: 'Offline',
        };
    }
  };

  const { bg, dot, label } = getStatusStyles();

  return (
    <div className="flex items-center gap-2">
      <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${bg}`}>
        <span className={`w-2 h-2 rounded-full ${dot}`}></span>
        <span>{label}</span>
      </div>

      {status === 'connected' && typeof viewerCount === 'number' && viewerCount > 0 && (
        <div className="text-xs text-gray-500 font-medium">
          {viewerCount} {viewerCount === 1 ? 'person' : 'people'} viewing
        </div>
      )}
    </div>
  );
};
