'use client';

import { SystemStatus as SystemStatusType, Alert } from '@/types/mev';
import { formatDistanceToNow } from '@/utils/dateUtils';

interface SystemStatusProps {
  status: SystemStatusType | null;
  alerts: Alert[];
  onAcknowledgeAlert?: (alertId: string) => void;
  isConnected: boolean;
}

const formatUptime = (seconds: number): string => {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes}m`;
};

const formatBytes = (bytes: number): string => {
  const MB = bytes / (1024 * 1024);
  return `${MB.toFixed(1)} MB`;
};

export const SystemStatus: React.FC<SystemStatusProps> = ({
  status,
  alerts,
  onAcknowledgeAlert,
  isConnected,
}) => {
  const unacknowledgedAlerts = alerts.filter(alert => !alert.acknowledged);

  return (
    <div className="space-y-6">
      {/* Connection Status */}
      <div className={`card ${isConnected ? 'border-mev-primary bg-green-50 dark:bg-green-900/20' : 'border-mev-danger bg-red-50 dark:bg-red-900/20'}`}>
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-lg font-semibold mb-2">Connection Status</h3>
            <div className={`text-2xl font-bold ${isConnected ? 'text-mev-primary' : 'text-mev-danger'}`}>
              {isConnected ? 'CONNECTED' : 'DISCONNECTED'}
            </div>
          </div>
          <div className={`w-4 h-4 rounded-full ${isConnected ? 'bg-mev-primary animate-pulse' : 'bg-mev-danger'}`}></div>
        </div>
      </div>

      {/* System Metrics */}
      {status && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Mempool Connections</div>
            <div className={`text-2xl font-bold ${status.mempoolConnections > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
              {status.mempoolConnections}
            </div>
            <div className="text-xs text-gray-400">Active connections</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Active Simulations</div>
            <div className="text-2xl font-bold text-mev-secondary">
              {status.activeSimulations}
            </div>
            <div className="text-xs text-gray-400">Running simulations</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Queue Size</div>
            <div className={`text-2xl font-bold ${status.queueSize > 8000 ? 'text-mev-danger' : status.queueSize > 5000 ? 'text-mev-warning' : 'text-mev-primary'}`}>
              {status.queueSize.toLocaleString()}
            </div>
            <div className="text-xs text-gray-400">Pending transactions</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Last Block</div>
            <div className="text-2xl font-bold">
              {status.lastBlockNumber.toLocaleString()}
            </div>
            <div className="text-xs text-gray-400">Latest block number</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Uptime</div>
            <div className="text-2xl font-bold text-mev-primary">
              {formatUptime(status.uptime)}
            </div>
            <div className="text-xs text-gray-400">System uptime</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">Memory Usage</div>
            <div className={`text-2xl font-bold ${status.memoryUsage > 2000 ? 'text-mev-danger' : status.memoryUsage > 1000 ? 'text-mev-warning' : 'text-mev-primary'}`}>
              {formatBytes(status.memoryUsage)}
            </div>
            <div className="text-xs text-gray-400">RAM usage</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">CPU Usage</div>
            <div className={`text-2xl font-bold ${status.cpuUsage > 80 ? 'text-mev-danger' : status.cpuUsage > 60 ? 'text-mev-warning' : 'text-mev-primary'}`}>
              {status.cpuUsage.toFixed(1)}%
            </div>
            <div className="text-xs text-gray-400">CPU utilization</div>
          </div>

          <div className="card">
            <div className="text-sm text-gray-500 mb-1">System Health</div>
            <div className={`text-lg font-bold ${
              status.queueSize < 8000 && status.memoryUsage < 2000 && status.cpuUsage < 80 
                ? 'text-mev-primary' 
                : status.queueSize < 9000 && status.memoryUsage < 3000 && status.cpuUsage < 90
                ? 'text-mev-warning'
                : 'text-mev-danger'
            }`}>
              {status.queueSize < 8000 && status.memoryUsage < 2000 && status.cpuUsage < 80 
                ? 'HEALTHY' 
                : status.queueSize < 9000 && status.memoryUsage < 3000 && status.cpuUsage < 90
                ? 'WARNING'
                : 'CRITICAL'
              }
            </div>
            <div className="text-xs text-gray-400">Overall status</div>
          </div>
        </div>
      )}

      {/* Alerts */}
      {unacknowledgedAlerts.length > 0 && (
        <div className="card border-mev-warning bg-yellow-50 dark:bg-yellow-900/20">
          <h3 className="text-lg font-semibold mb-4 text-mev-warning">Active Alerts</h3>
          <div className="space-y-3">
            {unacknowledgedAlerts.slice(0, 5).map((alert) => (
              <div
                key={alert.id}
                className={`p-3 rounded-lg border ${
                  alert.type === 'error' ? 'border-red-300 bg-red-50 dark:bg-red-900/20' :
                  alert.type === 'warning' ? 'border-yellow-300 bg-yellow-50 dark:bg-yellow-900/20' :
                  'border-blue-300 bg-blue-50 dark:bg-blue-900/20'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className={`text-sm font-medium ${
                      alert.type === 'error' ? 'text-red-800 dark:text-red-200' :
                      alert.type === 'warning' ? 'text-yellow-800 dark:text-yellow-200' :
                      'text-blue-800 dark:text-blue-200'
                    }`}>
                      {alert.type.toUpperCase()}
                    </div>
                    <div className="text-sm text-gray-700 dark:text-gray-300 mt-1">
                      {alert.message}
                    </div>
                    <div className="text-xs text-gray-500 mt-1">
                      {formatDistanceToNow(new Date(alert.timestamp), { addSuffix: true })}
                    </div>
                  </div>
                  {onAcknowledgeAlert && (
                    <button
                      onClick={() => onAcknowledgeAlert(alert.id)}
                      className="ml-3 px-3 py-1 text-xs bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 rounded transition-colors"
                    >
                      Acknowledge
                    </button>
                  )}
                </div>
              </div>
            ))}
            {unacknowledgedAlerts.length > 5 && (
              <div className="text-sm text-gray-600 text-center">
                +{unacknowledgedAlerts.length - 5} more alerts
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}; 