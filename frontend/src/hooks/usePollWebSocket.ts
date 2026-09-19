import { useState, useEffect, useRef, useCallback } from 'react';
import { WSMessage } from '../types';

export type ConnectionStatus = 'connected' | 'reconnecting' | 'offline';

interface UsePollWebSocketProps {
  pollId: string;
  initialVotes?: Record<string, number>;
  initialTotalVotes?: number;
  initialIsClosed?: boolean;
}

export function usePollWebSocket({
  pollId,
  initialVotes = {},
  initialTotalVotes = 0,
  initialIsClosed = false,
}: UsePollWebSocketProps) {
  const [votes, setVotes] = useState<Record<string, number>>(initialVotes);
  const [totalVotes, setTotalVotes] = useState<number>(initialTotalVotes);
  const [viewerCount, setViewerCount] = useState<number>(1);
  const [isClosed, setIsClosed] = useState<boolean>(initialIsClosed);
  const [isDeleted, setIsDeleted] = useState<boolean>(false);
  const [status, setStatus] = useState<ConnectionStatus>('offline');

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptRef = useRef<number>(0);
  const isUnmountingRef = useRef<boolean>(false);

  // Sync initial props
  useEffect(() => {
    setVotes(initialVotes);
  }, [initialVotes]);

  useEffect(() => {
    setTotalVotes(initialTotalVotes);
  }, [initialTotalVotes]);

  useEffect(() => {
    setIsClosed(initialIsClosed);
  }, [initialIsClosed]);

  const getWebSocketUrl = useCallback(() => {
    const envWsUrl = import.meta.env.VITE_WS_URL;
    if (envWsUrl) {
      return `${envWsUrl}/polls/${pollId}`;
    }

    // Default fallback based on window protocol
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.hostname === 'localhost' ? 'localhost:8080' : window.location.host;
    return `${protocol}//${host}/ws/polls/${pollId}`;
  }, [pollId]);

  const connect = useCallback(() => {
    if (isUnmountingRef.current || !pollId) return;

    // Clean up existing instance if any
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const url = getWebSocketUrl();
    const ws = new WebSocket(url);
    wsRef.current = ws;

    setStatus(reconnectAttemptRef.current > 0 ? 'reconnecting' : 'offline');

    ws.onopen = () => {
      if (isUnmountingRef.current) return;
      setStatus('connected');
      reconnectAttemptRef.current = 0;
    };

    ws.onmessage = (event) => {
      if (isUnmountingRef.current) return;
      try {
        const msg: WSMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'INIT_STATE':
            if (msg.votes) setVotes(msg.votes);
            if (typeof msg.totalVotes === 'number') setTotalVotes(msg.totalVotes);
            if (typeof msg.viewerCount === 'number') setViewerCount(msg.viewerCount);
            if (typeof msg.isClosed === 'boolean') setIsClosed(msg.isClosed);
            break;

          case 'VOTE_UPDATE':
            if (msg.votes) setVotes(msg.votes);
            if (typeof msg.totalVotes === 'number') setTotalVotes(msg.totalVotes);
            if (typeof msg.isClosed === 'boolean') setIsClosed(msg.isClosed);
            break;

          case 'VIEWER_UPDATE':
            if (typeof msg.viewerCount === 'number') setViewerCount(msg.viewerCount);
            break;

          case 'POLL_CLOSED':
            setIsClosed(true);
            break;

          case 'POLL_DELETED':
            setIsDeleted(true);
            break;

          default:
            break;
        }
      } catch {
        // Ignore unparseable frames
      }
    };

    ws.onclose = () => {
      if (isUnmountingRef.current) return;
      setStatus('offline');
      wsRef.current = null;

      // Exponential backoff reconnect: 1s, 2s, 4s, max 10s
      const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current), 10000);
      reconnectAttemptRef.current += 1;
      setStatus('reconnecting');

      reconnectTimeoutRef.current = window.setTimeout(() => {
        connect();
      }, delay);
    };

    ws.onerror = () => {
      // onclose will trigger next
    };
  }, [getWebSocketUrl, pollId]);

  useEffect(() => {
    isUnmountingRef.current = false;
    connect();

    return () => {
      isUnmountingRef.current = true;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  return {
    votes,
    totalVotes,
    viewerCount,
    isClosed,
    isDeleted,
    status,
  };
}
