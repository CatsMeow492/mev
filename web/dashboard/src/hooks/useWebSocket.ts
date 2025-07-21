'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { MEVOpportunity, PerformanceMetrics, SystemStatus, Alert, OpportunityStatus, ExecutionMode, ExecutionUpdateMessage } from '@/types/mev';

interface WebSocketMessage {
  type: string;
  data: any;
  timestamp: string;
}

// Helper function to parse latency values from backend
const parseLatencyValue = (latencyStr: string): number => {
  if (!latencyStr || latencyStr === 'N/A') {
    return 0;
  }
  
  // Handle different formats: "15.2ms", "1.5μs", "N/A"
  const numericPart = parseFloat(latencyStr.replace(/[^0-9.]/g, ''));
  
  if (latencyStr.includes('μs')) {
    return numericPart / 1000; // Convert microseconds to milliseconds
  } else if (latencyStr.includes('ms')) {
    return numericPart;
  }
  
  return numericPart || 0;
};

interface WebSocketData {
  opportunities: MEVOpportunity[];
  metrics: PerformanceMetrics | null;
  systemStatus: SystemStatus | null;
  alerts: Alert[];
  isConnected: boolean;
  error: string | null;
  // NEW: Execution-related state
  executionMode: ExecutionMode;
  executionStats: any | null;
}

export const useWebSocket = () => {
  const [data, setData] = useState<WebSocketData>({
    opportunities: [],
    metrics: null,
    systemStatus: null,
    alerts: [],
    isConnected: false,
    error: null,
    // NEW: Default to simulation mode
    executionMode: 'simulation',
    executionStats: null,
  });

  const websocketRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const maxReconnectAttempts = 10;
  const reconnectDelay = 3000;

  const connect = useCallback(() => {
    try {
      // Clean up any existing connection first
      if (websocketRef.current) {
        websocketRef.current.close();
        websocketRef.current = null;
      }

      // Connect to the live MEV engine WebSocket
      const ws = new WebSocket('wss://mev-strategy-dev.fly.dev/ws');
      websocketRef.current = ws;

      // Store ping interval reference for proper cleanup
      let pingInterval: ReturnType<typeof setInterval> | null = null;

      ws.onopen = () => {
        console.log('🔗 Connected to live MEV Engine WebSocket');
        setData(prev => ({
          ...prev,
          isConnected: true,
          error: null,
        }));
        setReconnectAttempts(0);

        // Send subscription message
        ws.send(JSON.stringify({
          type: 'subscribe',
          channel: 'all'
        }));

        // Send JSON ping to keep connection alive
        // Note: Browser WebSocket doesn't support protocol-level ping frames
        pingInterval = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }));
          }
        }, 25000); // Reduced to 25 seconds to be well within the 60-second timeout
      };

      ws.onclose = (event) => {
        console.log('❌ Disconnected from MEV Engine WebSocket:', event.code, event.reason);
        
        // Clean up ping interval
        if (pingInterval) {
          clearInterval(pingInterval as any);
          pingInterval = null;
        }

        setData(prev => ({
          ...prev,
          isConnected: false,
        }));

        // Attempt to reconnect with exponential backoff
        if (reconnectAttempts < maxReconnectAttempts) {
          const backoffDelay = Math.min(reconnectDelay * Math.pow(2, reconnectAttempts), 30000);
          console.log(`🔄 Attempting to reconnect (${reconnectAttempts + 1}/${maxReconnectAttempts}) in ${backoffDelay}ms...`);
          
          reconnectTimeoutRef.current = setTimeout(() => {
            setReconnectAttempts(prev => prev + 1);
            connect();
          }, backoffDelay) as any;
        } else {
          setData(prev => ({
            ...prev,
            error: 'Max reconnection attempts reached',
          }));
        }
      };

      ws.onerror = (error) => {
        console.error('🚨 WebSocket error:', error);
        setData(prev => ({
          ...prev,
          error: 'WebSocket connection error',
        }));
      };

      // Handle pong responses (for protocol-level pings)
      ws.addEventListener('pong', () => {
        console.log('📡 Received pong response');
      });

      ws.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          console.log('📡 Received message:', message.type, message.data);

          switch (message.type) {
            case 'opportunity':
              // Real-time MEV opportunity detected!
              console.log('💰 MEV Opportunity detected:', message.data);
              const opportunity: MEVOpportunity = {
                id: message.data.id,
                strategy: message.data.strategy,
                status: message.data.status as OpportunityStatus,
                targetTx: message.data.targetTx,
                expectedProfit: message.data.expectedProfit,
                gasCost: message.data.gasCost,
                netProfit: message.data.netProfit,
                confidence: message.data.confidence,
                blockNumber: message.data.blockNumber,
                createdAt: message.data.timestamp || new Date().toISOString(),
                executionTxs: message.data.executionTxs || [],
                simulationLatency: parseLatencyValue(message.data.simulationLatency || 'N/A'),
              };

              setData(prev => ({
                ...prev,
                opportunities: [opportunity, ...prev.opportunities.slice(0, 99)],
              }));
              break;

            case 'metrics':
              // Real-time performance metrics with execution data
              console.log('📊 Metrics update:', message.data);
              const metrics: PerformanceMetrics = {
                windowSize: 1000,
                totalTrades: message.data.opportunities_detected || 0,
                profitableTrades: message.data.profitable_opportunities || 0,
                lossRate: message.data.success_rate ? (1 - message.data.success_rate) : 0,
                avgLatency: parseLatencyValue(message.data.avg_latency || 'N/A'),
                totalProfit: message.data.total_profit || '0.000000',
                lastUpdated: new Date().toISOString(),
                successRate: message.data.success_rate || 0,
                avgProfitPerTrade: '0.00',
                
                // NEW: Execution metrics
                executionMode: message.data.execution_mode || 'simulation',
                totalExecutions: message.data.total_executions || 0,
                successfulExecutions: message.data.successful_executions || 0,
                failedExecutions: message.data.failed_executions || 0,
                simulationSuccessRate: message.data.simulation_success_rate || 0,
                liveSuccessRate: message.data.live_success_rate || 0,
                totalExecutionProfit: message.data.total_execution_profit || '0.000000',
                totalExecutionGasCost: message.data.total_execution_gas_cost || '0.000000',
                netExecutionProfit: message.data.net_execution_profit || '0.000000',
                avgExecutionTime: message.data.avg_execution_time || 0,
                avgSlippage: message.data.avg_slippage || 0,
                emergencyStop: message.data.emergency_stop || false,
                dailyVolumeUsed: message.data.daily_volume_used || '0.000000',
              };

              setData(prev => ({
                ...prev,
                metrics,
                executionMode: metrics.executionMode as ExecutionMode,
              }));
              break;

            case 'system_status':
              // System status updates (including execution context)
              console.log('🖥️  System status update:', message.data);
              
              // Check if this is an execution context message from the detector
              if (message.data.execution_mode && message.data.latency) {
                console.log('🎯 Execution context:', {
                  mode: message.data.execution_mode,
                  latency: message.data.latency,
                  simulationStatus: message.data.simulation_status
                });
                
                // Update execution mode and latency in system status
                setData(prev => ({
                  ...prev,
                  systemStatus: {
                    ...prev.systemStatus,
                    isConnected: true,
                    mempoolConnections: prev.systemStatus?.mempoolConnections || 0,
                    activeSimulations: message.data.execution_mode === 'simulation' ? 1 : 0,
                    queueSize: 0,
                    lastBlockNumber: prev.systemStatus?.lastBlockNumber || 0,
                    uptime: prev.systemStatus?.uptime || Math.floor(Date.now() / 1000),
                    memoryUsage: 0,
                    cpuUsage: 0,
                    executionMode: message.data.execution_mode,
                    currentLatency: message.data.latency,
                  }
                }));
              } else {
                // Regular system status update
                const systemStatus: SystemStatus = {
                  isConnected: message.data.blockchain === 'connected',
                  mempoolConnections: message.data.connected_peers || 0,
                  activeSimulations: 0,
                  queueSize: 0,
                  lastBlockNumber: message.data.last_block || 0,
                  uptime: Math.floor(Date.now() / 1000),
                  memoryUsage: 0,
                  cpuUsage: 0,
                };

                setData(prev => ({
                  ...prev,
                  systemStatus,
                }));
              }
              break;

            case 'execution_update':
              // NEW: Handle execution-specific updates
              console.log('🚀 Execution update:', message.data);
              const execUpdate = message.data as ExecutionUpdateMessage;
              
              // Update the opportunity with execution results
              setData(prev => ({
                ...prev,
                opportunities: prev.opportunities.map(opp => 
                  opp.id === execUpdate.opportunity.id 
                    ? {
                        ...opp,
                        ...execUpdate.opportunity,
                        executionMode: execUpdate.executionMode,
                        executionResult: execUpdate.executionResult,
                        simulationComparison: execUpdate.simulationComparison,
                      }
                    : opp
                ),
                executionMode: execUpdate.executionMode,
              }));
              break;

            case 'alert':
              // System alerts
              console.log('🚨 Alert received:', message.data);
              const alert: Alert = {
                id: message.data.id || `alert_${Date.now()}`,
                type: message.data.type || 'info',
                message: message.data.message || '',
                timestamp: message.data.timestamp || new Date().toISOString(),
                acknowledged: message.data.acknowledged || false,
              };

              setData(prev => ({
                ...prev,
                alerts: [alert, ...prev.alerts.slice(0, 49)],
              }));
              break;

            case 'pong':
              // Pong response to keep connection alive (application-level)
              console.log('📡 Received application-level pong');
              break;

            default:
              console.log('🤔 Unknown message type:', message.type);
          }
        } catch (error) {
          console.error('❌ Failed to parse WebSocket message:', error);
        }
      };

    } catch (error) {
      console.error('❌ Failed to create WebSocket connection:', error);
      setData(prev => ({
        ...prev,
        error: 'Failed to establish connection',
      }));
    }
  }, [reconnectAttempts, maxReconnectAttempts, reconnectDelay]);

  const disconnect = useCallback(() => {
    if (websocketRef.current) {
      websocketRef.current.close();
      websocketRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current as any);
      reconnectTimeoutRef.current = null;
    }
  }, []);

  const acknowledgeAlert = useCallback((alertId: string) => {
    setData(prev => ({
      ...prev,
      alerts: prev.alerts.map(alert =>
        alert.id === alertId ? { ...alert, acknowledged: true } : alert
      ),
    }));
  }, []);

  const sendMessage = useCallback((type: string, data: any = {}) => {
    if (websocketRef.current && websocketRef.current.readyState === WebSocket.OPEN) {
      websocketRef.current.send(JSON.stringify({ type, data }));
    }
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
    sendMessage,
    // Extract execution mode from system status for convenience
    executionMode: data.systemStatus?.executionMode || ('simulation' as const),
    currentLatency: data.systemStatus?.currentLatency || 'N/A',
  };
}; 