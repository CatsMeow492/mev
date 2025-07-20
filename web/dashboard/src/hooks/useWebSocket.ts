'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { io, Socket } from 'socket.io-client';
import { MEVOpportunity, PerformanceMetrics, SystemStatus, Alert, Transaction, OpportunityStatus } from '@/types/mev';

interface WebSocketData {
  opportunities: MEVOpportunity[];
  metrics: PerformanceMetrics | null;
  systemStatus: SystemStatus | null;
  alerts: Alert[];
  isConnected: boolean;
  error: string | null;
}

export const useWebSocket = () => {
  const [data, setData] = useState<WebSocketData>({
    opportunities: [],
    metrics: null,
    systemStatus: null,
    alerts: [],
    isConnected: false,
    error: null,
  });

  const socketRef = useRef<Socket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const connect = useCallback(() => {
    try {
      const socket = io('ws://localhost:8080', {
        transports: ['websocket'],
        autoConnect: true,
        reconnection: true,
        reconnectionDelay: 1000,
        reconnectionDelayMax: 5000,
        reconnectionAttempts: 10,
      });

      socketRef.current = socket;

      socket.on('connect', () => {
        console.log('Connected to MEV Engine WebSocket');
        setData(prev => ({
          ...prev,
          isConnected: true,
          error: null,
        }));
        setReconnectAttempts(0);

        // Subscribe to real-time streams
        socket.emit('subscribe', 'opportunities');
        socket.emit('subscribe', 'metrics');
        socket.emit('subscribe', 'system_status');
        socket.emit('subscribe', 'alerts');
      });

      socket.on('disconnect', () => {
        console.log('Disconnected from MEV Engine WebSocket');
        setData(prev => ({
          ...prev,
          isConnected: false,
        }));
      });

      socket.on('connect_error', (error) => {
        console.error('WebSocket connection error:', error);
        setData(prev => ({
          ...prev,
          isConnected: false,
          error: error.message,
        }));
        setReconnectAttempts(prev => prev + 1);
      });

      // Handle real-time opportunity updates
      socket.on('opportunity', (opportunity: MEVOpportunity) => {
        setData(prev => ({
          ...prev,
          opportunities: [opportunity, ...prev.opportunities.slice(0, 99)], // Keep last 100
        }));
      });

      // Handle opportunity status updates
      socket.on('opportunity_update', (update: { id: string; status: string; executionTxs?: Transaction[] }) => {
        setData(prev => ({
          ...prev,
          opportunities: prev.opportunities.map(opp =>
            opp.id === update.id
              ? { ...opp, status: update.status as OpportunityStatus, executionTxs: update.executionTxs || opp.executionTxs }
              : opp
          ),
        }));
      });

      // Handle performance metrics updates
      socket.on('metrics', (metrics: PerformanceMetrics) => {
        setData(prev => ({
          ...prev,
          metrics,
        }));
      });

      // Handle system status updates
      socket.on('system_status', (status: SystemStatus) => {
        setData(prev => ({
          ...prev,
          systemStatus: status,
        }));
      });

      // Handle alerts
      socket.on('alert', (alert: Alert) => {
        setData(prev => ({
          ...prev,
          alerts: [alert, ...prev.alerts.slice(0, 49)], // Keep last 50 alerts
        }));
      });

    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
      setData(prev => ({
        ...prev,
        error: 'Failed to establish connection',
      }));
    }
  }, []);

  const disconnect = useCallback(() => {
    if (socketRef.current) {
      socketRef.current.disconnect();
      socketRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
  }, []);

  const acknowledgeAlert = useCallback((alertId: string) => {
    if (socketRef.current) {
      socketRef.current.emit('acknowledge_alert', alertId);
    }
    setData(prev => ({
      ...prev,
      alerts: prev.alerts.map(alert =>
        alert.id === alertId ? { ...alert, acknowledged: true } : alert
      ),
    }));
  }, []);

  useEffect(() => {
    connect();

    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    ...data,
    reconnectAttempts,
    connect,
    disconnect,
    acknowledgeAlert,
  };
}; 