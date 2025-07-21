'use client';

import { useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useApi } from '@/hooks/useApi';
import { MEVOpportunity } from '@/types/mev';
import { OpportunityList } from '@/components/OpportunityList';
import { MetricsPanel } from '@/components/MetricsPanel';
import { SystemStatus } from '@/components/SystemStatus';
import { ExecutionModePanel } from '@/components/ExecutionModePanel';
import LiquidationPanel from '@/components/LiquidationPanel';

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState<'opportunities' | 'metrics' | 'execution' | 'liquidation' | 'system'>('opportunities');
  const [selectedOpportunity, setSelectedOpportunity] = useState<MEVOpportunity | null>(null);

  const {
    opportunities,
    metrics,
    systemStatus,
    alerts,
    isConnected,
    error,
    reconnectAttempts,
    acknowledgeAlert,
    executionMode,
    sendMessage,
  } = useWebSocket();

  const {
    emergencyShutdown,
    restartSystem,
    loading: apiLoading,
    error: apiError,
  } = useApi();

  const handleOpportunityClick = (opportunity: MEVOpportunity) => {
    setSelectedOpportunity(opportunity);
  };

  const handleEmergencyShutdown = async () => {
    const confirmed = window.confirm('Are you sure you want to trigger emergency shutdown?');
    if (confirmed) {
      const reason = prompt('Please provide a reason for the shutdown:');
      if (reason) {
        await emergencyShutdown(reason);
      }
    }
  };

  const handleRestart = async () => {
    const confirmed = window.confirm('Are you sure you want to restart the system?');
    if (confirmed) {
      await restartSystem();
    }
  };

  const handleExecutionModeChange = (newMode: string) => {
    // Send execution mode change request via WebSocket
    if (sendMessage) {
      sendMessage('execution_mode_change', { mode: newMode });
    }
  };

  return (
    <div className="min-h-screen bg-mev-darker">
      {/* Header */}
      <header className="bg-mev-dark border-b border-gray-700 sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <h1 className="text-2xl font-bold text-white">MEV Engine Dashboard</h1>
              <div className="flex items-center space-x-2">
                <div className={`w-3 h-3 rounded-full ${isConnected ? 'bg-mev-primary animate-pulse' : 'bg-mev-danger'}`}></div>
                <span className={`text-sm ${isConnected ? 'text-mev-primary' : 'text-mev-danger'}`}>
                  {isConnected ? 'Connected' : reconnectAttempts > 0 ? `Reconnecting... (${reconnectAttempts})` : 'Disconnected'}
                </span>
              </div>
            </div>

            <div className="flex items-center space-x-4">
              {/* Tab Navigation */}
              <nav className="flex space-x-1">
                <button
                  onClick={() => setActiveTab('opportunities')}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    activeTab === 'opportunities'
                      ? 'bg-mev-primary text-white'
                      : 'text-gray-300 hover:text-white hover:bg-gray-700'
                  }`}
                >
                  Opportunities ({opportunities.length})
                </button>
                <button
                  onClick={() => setActiveTab('metrics')}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    activeTab === 'metrics'
                      ? 'bg-mev-primary text-white'
                      : 'text-gray-300 hover:text-white hover:bg-gray-700'
                  }`}
                >
                  Metrics
                </button>
                <button
                  onClick={() => setActiveTab('execution')}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    activeTab === 'execution'
                      ? 'bg-mev-primary text-white'
                      : 'text-gray-300 hover:text-white hover:bg-gray-700'
                  }`}
                >
                  🚀 Execution
                </button>
                <button
                  onClick={() => setActiveTab('liquidation')}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    activeTab === 'liquidation'
                      ? 'bg-mev-primary text-white'
                      : 'text-gray-300 hover:text-white hover:bg-gray-700'
                  }`}
                >
                  💧 Liquidation
                </button>
                <button
                  onClick={() => setActiveTab('system')}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    activeTab === 'system'
                      ? 'bg-mev-primary text-white'
                      : 'text-gray-300 hover:text-white hover:bg-gray-700'
                  }`}
                >
                  System {alerts.filter(a => !a.acknowledged).length > 0 && (
                    <span className="ml-2 px-2 py-1 bg-mev-danger text-white text-xs rounded-full">
                      {alerts.filter(a => !a.acknowledged).length}
                    </span>
                  )}
                </button>
              </nav>

              {/* Emergency Controls */}
              <div className="flex items-center space-x-2">
                <button
                  onClick={handleRestart}
                  disabled={apiLoading || !isConnected}
                  className="px-3 py-2 bg-mev-warning hover:bg-yellow-600 text-white text-sm rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Restart
                </button>
                <button
                  onClick={handleEmergencyShutdown}
                  disabled={apiLoading}
                  className="px-3 py-2 bg-mev-danger hover:bg-red-600 text-white text-sm rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Emergency Stop
                </button>
              </div>
            </div>
          </div>
        </div>
      </header>

      {/* Error Banner */}
      {(error || apiError) && (
        <div className="bg-red-600 text-white px-6 py-3">
          <div className="max-w-7xl mx-auto">
            <div className="flex items-center justify-between">
              <div>
                <strong>Error:</strong> {error || apiError}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-6 py-6">
        {activeTab === 'opportunities' && (
          <OpportunityList
            opportunities={opportunities}
            onOpportunityClick={handleOpportunityClick}
          />
        )}

        {activeTab === 'metrics' && (
          <MetricsPanel metrics={metrics} isLoading={false} />
        )}

        {activeTab === 'execution' && (
          <ExecutionModePanel
            executionMode={executionMode}
            metrics={metrics}
            onModeChange={handleExecutionModeChange}
          />
        )}

        {activeTab === 'liquidation' && (
          <LiquidationPanel />
        )}

        {activeTab === 'system' && (
          <SystemStatus
            status={systemStatus}
            alerts={alerts}
            onAcknowledgeAlert={acknowledgeAlert}
            isConnected={isConnected}
          />
        )}
      </main>

      {/* Opportunity Detail Modal */}
      {selectedOpportunity && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
          <div className="bg-white dark:bg-mev-dark rounded-lg max-w-2xl w-full max-h-[80vh] overflow-y-auto">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-bold">Opportunity Details</h2>
                <button
                  onClick={() => setSelectedOpportunity(null)}
                  className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">ID</label>
                    <div className="mt-1 font-mono text-sm">{selectedOpportunity.id}</div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Strategy</label>
                    <div className="mt-1">{selectedOpportunity.strategy}</div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Status</label>
                    <div className="mt-1">
                      <span className={`status-${selectedOpportunity.status}`}>
                        {selectedOpportunity.status.toUpperCase()}
                      </span>
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Confidence</label>
                    <div className="mt-1">{(selectedOpportunity.confidence * 100).toFixed(1)}%</div>
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Target Transaction</label>
                  <div className="mt-1 font-mono text-sm break-all">{selectedOpportunity.targetTx}</div>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Expected Profit</label>
                    <div className="mt-1 text-mev-primary font-semibold">
                      {parseFloat(selectedOpportunity.expectedProfit).toFixed(6)} ETH
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Gas Cost</label>
                    <div className="mt-1 text-mev-warning font-semibold">
                      {(parseFloat(selectedOpportunity.gasCost) * 1e9).toFixed(2)} Gwei
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Net Profit</label>
                    <div className={`mt-1 font-semibold ${parseFloat(selectedOpportunity.netProfit) > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
                      {parseFloat(selectedOpportunity.netProfit) > 0 ? '+' : ''}{parseFloat(selectedOpportunity.netProfit).toFixed(6)} ETH
                    </div>
                  </div>
                </div>

                {selectedOpportunity.executionTxs.length > 0 && (
                  <div>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                      Execution Transactions ({selectedOpportunity.executionTxs.length})
                    </label>
                    <div className="space-y-2">
                      {selectedOpportunity.executionTxs.map((tx, index) => (
                        <div key={index} className="p-3 bg-gray-50 dark:bg-gray-800 rounded">
                          <div className="font-mono text-sm break-all">{tx.hash}</div>
                          <div className="text-xs text-gray-500 mt-1">
                            Gas: {tx.gasLimit?.toLocaleString() || 'N/A'} @ {tx.gasPrice ? (parseFloat(tx.gasPrice) / 1e9).toFixed(2) : 'N/A'} Gwei
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
} 