'use client';

import React from 'react';
import { ExecutionMode, ExecutionStats, PerformanceMetrics } from '@/types/mev';

interface ExecutionModePanelProps {
  executionMode: ExecutionMode;
  metrics: PerformanceMetrics | null;
  onModeChange?: (mode: ExecutionMode) => void;
  className?: string;
}

export const ExecutionModePanel: React.FC<ExecutionModePanelProps> = ({
  executionMode,
  metrics,
  onModeChange,
  className = '',
}) => {
  const getModeColor = (mode: ExecutionMode) => {
    switch (mode) {
      case 'simulation': return 'bg-blue-500';
      case 'hybrid': return 'bg-yellow-500';
      case 'live': return 'bg-red-500';
      default: return 'bg-gray-500';
    }
  };

  const getModeIcon = (mode: ExecutionMode) => {
    switch (mode) {
      case 'simulation': return '📋';
      case 'hybrid': return '🔄';
      case 'live': return '🚀';
      default: return '❓';
    }
  };

  const getModeDescription = (mode: ExecutionMode) => {
    switch (mode) {
      case 'simulation': 
        return 'Paper trading mode - No real transactions, risk-free testing';
      case 'hybrid': 
        return 'Gradual transition - Simulate first, execute live if highly profitable';
      case 'live': 
        return 'Live trading mode - Direct execution with real funds';
      default: 
        return 'Unknown execution mode';
    }
  };

  const getProgressToNextMode = (currentMode: ExecutionMode, metrics: PerformanceMetrics | null) => {
    if (!metrics) return null;

    switch (currentMode) {
      case 'simulation':
        const simExecution = metrics.totalExecutions || 0;
        const simSuccessRate = metrics.simulationSuccessRate || 0;
        return {
          nextMode: 'hybrid' as ExecutionMode,
          requirements: [
            { name: 'Simulation Runs', current: simExecution, required: 100, met: simExecution >= 100 },
            { name: 'Success Rate', current: `${(simSuccessRate * 100).toFixed(1)}%`, required: '85%', met: simSuccessRate >= 0.85 },
          ]
        };
      case 'hybrid':
        const liveSuccessRate = metrics.liveSuccessRate || 0;
        const totalLiveExecutions = metrics.successfulExecutions || 0;
        return {
          nextMode: 'live' as ExecutionMode,
          requirements: [
            { name: 'Live Executions', current: totalLiveExecutions, required: 50, met: totalLiveExecutions >= 50 },
            { name: 'Live Success Rate', current: `${(liveSuccessRate * 100).toFixed(1)}%`, required: '90%', met: liveSuccessRate >= 0.90 },
          ]
        };
      case 'live':
        return { nextMode: null, requirements: [] };
      default:
        return null;
    }
  };

  const progression = getProgressToNextMode(executionMode, metrics);
  const canAdvance = progression?.requirements.every(req => req.met) || false;

  return (
    <div className={`bg-white dark:bg-gray-800 rounded-lg shadow-lg border p-6 ${className}`}>
      {/* Current Mode Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center space-x-4">
          <div className={`w-4 h-4 rounded-full ${getModeColor(executionMode)} animate-pulse`}></div>
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
              {getModeIcon(executionMode)} {executionMode.toUpperCase()} MODE
            </h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              {getModeDescription(executionMode)}
            </p>
          </div>
        </div>
        
        {/* Emergency Stop Indicator */}
        {metrics?.emergencyStop && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-3 py-1 rounded-full text-sm font-medium">
            🚨 EMERGENCY STOP
          </div>
        )}
      </div>

      {/* Execution Statistics */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4 mb-6">
        <div className="text-center">
          <div className="text-2xl font-bold text-gray-900 dark:text-white">
            {metrics?.totalExecutions || 0}
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">Total Executions</div>
        </div>
        
        <div className="text-center">
          <div className="text-2xl font-bold text-green-600">
            {metrics?.successfulExecutions || 0}
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">Successful</div>
        </div>
        
        <div className="text-center">
          <div className="text-2xl font-bold text-blue-600">
            {metrics?.netExecutionProfit || '0.000'}
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">Net Profit (ETH)</div>
        </div>
        
        <div className="text-center">
          <div className="text-2xl font-bold text-purple-600">
            {metrics?.avgLatency ? `${metrics.avgLatency.toFixed(1)}ms` : 'N/A'}
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">Avg Latency</div>
        </div>
        
        <div className="text-center">
          <div className="text-2xl font-bold text-purple-600">
            {metrics?.avgExecutionTime || 0}ms
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">Avg Execution Time</div>
        </div>
      </div>

      {/* Daily Volume Usage */}
      {metrics?.dailyVolumeUsed && (
        <div className="mb-6">
          <div className="flex justify-between items-center mb-2">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Daily Volume Used</span>
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {metrics.dailyVolumeUsed} ETH
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div 
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ 
                width: `${Math.min((parseFloat(metrics.dailyVolumeUsed) / 10) * 100, 100)}%` 
              }}
            ></div>
          </div>
        </div>
      )}

      {/* Mode Progression */}
      {progression && progression.nextMode && (
        <div className="border-t pt-6">
          <h4 className="text-md font-semibold text-gray-900 dark:text-white mb-4">
            🎯 Progress to {progression.nextMode.toUpperCase()} Mode
          </h4>
          
          <div className="space-y-3">
            {progression.requirements.map((req, index) => (
              <div key={index} className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <div className={`w-3 h-3 rounded-full ${req.met ? 'bg-green-500' : 'bg-gray-300'}`}></div>
                  <span className="text-sm text-gray-700 dark:text-gray-300">{req.name}</span>
                </div>
                <div className="text-sm">
                  <span className={req.met ? 'text-green-600' : 'text-gray-600'}>
                    {req.current}
                  </span>
                  <span className="text-gray-500"> / {req.required}</span>
                </div>
              </div>
            ))}
          </div>

          {canAdvance && onModeChange && (
            <button
              onClick={() => onModeChange(progression.nextMode!)}
              className="mt-4 w-full bg-green-600 hover:bg-green-700 text-white font-medium py-2 px-4 rounded-lg transition-colors"
            >
              🚀 Advance to {progression.nextMode.toUpperCase()} Mode
            </button>
          )}
        </div>
      )}

      {/* Live Mode Controls */}
      {executionMode === 'live' && onModeChange && (
        <div className="border-t pt-6">
          <div className="flex space-x-3">
            <button
              onClick={() => onModeChange('hybrid')}
              className="flex-1 bg-yellow-600 hover:bg-yellow-700 text-white font-medium py-2 px-4 rounded-lg transition-colors"
            >
              ⬇️ Downgrade to Hybrid
            </button>
            <button
              onClick={() => onModeChange('simulation')}
              className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 rounded-lg transition-colors"
            >
              📋 Back to Simulation
            </button>
          </div>
        </div>
      )}

      {/* Success Rates */}
      <div className="mt-6 grid grid-cols-1 lg:grid-cols-2 gap-4">
        {metrics?.simulationSuccessRate !== undefined && (
          <div className="text-center p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
            <div className="text-lg font-bold text-blue-600">
              {(metrics.simulationSuccessRate * 100).toFixed(1)}%
            </div>
            <div className="text-sm text-blue-800 dark:text-blue-400">Simulation Success Rate</div>
          </div>
        )}
        
        {metrics?.liveSuccessRate !== undefined && metrics.liveSuccessRate > 0 && (
          <div className="text-center p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
            <div className="text-lg font-bold text-green-600">
              {(metrics.liveSuccessRate * 100).toFixed(1)}%
            </div>
            <div className="text-sm text-green-800 dark:text-green-400">Live Success Rate</div>
          </div>
        )}
      </div>
    </div>
  );
}; 