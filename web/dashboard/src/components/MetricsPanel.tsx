'use client';

import { PerformanceMetrics } from '@/types/mev';
import { formatDistanceToNow } from '@/utils/dateUtils';

interface MetricsPanelProps {
  metrics: PerformanceMetrics | null;
  isLoading?: boolean;
}

const formatETH = (value: string): string => {
  // Backend already sends ETH values, no need to divide by 1e18
  const num = parseFloat(value);
  return num.toFixed(6);
};

const formatLatency = (latencyMs: number): string => {
  return `${latencyMs.toFixed(1)}ms`;
};

export const MetricsPanel: React.FC<MetricsPanelProps> = ({ metrics, isLoading }) => {
  if (isLoading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {[...Array(8)].map((_, i) => (
          <div key={i} className="metric-card animate-pulse">
            <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded mb-2"></div>
            <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
          </div>
        ))}
      </div>
    );
  }

  if (!metrics) {
    return (
      <div className="card text-center text-gray-500">
        <div className="text-lg mb-2">No metrics available</div>
        <div className="text-sm">Waiting for system data...</div>
      </div>
    );
  }

  const successRate = ((metrics.profitableTrades / metrics.totalTrades) * 100) || 0;
  const lossRatePercentage = (metrics.lossRate * 100);

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Total Trades</div>
          <div className="text-2xl font-bold">{metrics.totalTrades.toLocaleString()}</div>
          <div className="text-xs text-gray-400">Window: {metrics.windowSize}</div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Success Rate</div>
          <div className={`text-2xl font-bold ${successRate > 60 ? 'text-mev-primary' : successRate > 30 ? 'text-mev-warning' : 'text-mev-danger'}`}>
            {successRate.toFixed(1)}%
          </div>
          <div className="text-xs text-gray-400">
            {metrics.profitableTrades} / {metrics.totalTrades} profitable
          </div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Loss Rate</div>
          <div className={`text-2xl font-bold ${lossRatePercentage < 30 ? 'text-mev-primary' : lossRatePercentage < 70 ? 'text-mev-warning' : 'text-mev-danger'}`}>
            {lossRatePercentage.toFixed(1)}%
          </div>
          <div className="text-xs text-gray-400">
            {lossRatePercentage >= 70 ? 'CRITICAL' : lossRatePercentage >= 50 ? 'WARNING' : 'HEALTHY'}
          </div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Avg Latency</div>
          <div className={`text-2xl font-bold ${metrics.avgLatency < 100 ? 'text-mev-primary' : metrics.avgLatency < 200 ? 'text-mev-warning' : 'text-mev-danger'}`}>
            {formatLatency(metrics.avgLatency)}
          </div>
          <div className="text-xs text-gray-400">Simulation time</div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Total Profit</div>
          <div className={`text-2xl font-bold ${parseFloat(metrics.totalProfit) > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
            {parseFloat(metrics.totalProfit) > 0 ? '+' : ''}{formatETH(metrics.totalProfit)} ETH
          </div>
          <div className="text-xs text-gray-400">Cumulative</div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Avg Profit/Trade</div>
          <div className={`text-2xl font-bold ${parseFloat(metrics.avgProfitPerTrade) > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
            {parseFloat(metrics.avgProfitPerTrade) > 0 ? '+' : ''}{formatETH(metrics.avgProfitPerTrade)} ETH
          </div>
          <div className="text-xs text-gray-400">Average per trade</div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Success Rate</div>
          <div className={`text-2xl font-bold ${metrics.successRate > 0.6 ? 'text-mev-primary' : metrics.successRate > 0.3 ? 'text-mev-warning' : 'text-mev-danger'}`}>
            {(metrics.successRate * 100).toFixed(1)}%
          </div>
          <div className="text-xs text-gray-400">Execution success</div>
        </div>

        <div className="metric-card">
          <div className="text-sm text-gray-500 mb-1">Last Updated</div>
          <div className="text-lg font-bold">
            {formatDistanceToNow(new Date(metrics.lastUpdated), { addSuffix: true })}
          </div>
          <div className="text-xs text-gray-400">Real-time data</div>
        </div>
      </div>

      {/* Status Indicators */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className={`card text-center ${lossRatePercentage >= 70 ? 'border-mev-danger bg-red-50 dark:bg-red-900/20' : 'border-mev-primary bg-green-50 dark:bg-green-900/20'}`}>
          <div className="text-lg font-semibold mb-2">System Status</div>
          <div className={`text-2xl font-bold ${lossRatePercentage >= 70 ? 'text-mev-danger' : 'text-mev-primary'}`}>
            {lossRatePercentage >= 70 ? 'DANGER' : lossRatePercentage >= 50 ? 'WARNING' : 'HEALTHY'}
          </div>
          <div className="text-sm text-gray-600 mt-1">
            {lossRatePercentage >= 70 && 'Automatic shutdown may trigger'}
            {lossRatePercentage >= 50 && lossRatePercentage < 70 && 'Monitor closely'}
            {lossRatePercentage < 50 && 'Operating normally'}
          </div>
        </div>

        <div className={`card text-center ${metrics.avgLatency > 200 ? 'border-mev-danger bg-red-50 dark:bg-red-900/20' : metrics.avgLatency > 100 ? 'border-mev-warning bg-yellow-50 dark:bg-yellow-900/20' : 'border-mev-primary bg-green-50 dark:bg-green-900/20'}`}>
          <div className="text-lg font-semibold mb-2">Performance</div>
          <div className={`text-2xl font-bold ${metrics.avgLatency > 200 ? 'text-mev-danger' : metrics.avgLatency > 100 ? 'text-mev-warning' : 'text-mev-primary'}`}>
            {metrics.avgLatency > 200 ? 'SLOW' : metrics.avgLatency > 100 ? 'DEGRADED' : 'OPTIMAL'}
          </div>
          <div className="text-sm text-gray-600 mt-1">
            Target: &lt;100ms average
          </div>
        </div>

        <div className={`card text-center ${parseFloat(metrics.totalProfit) < 0 ? 'border-mev-danger bg-red-50 dark:bg-red-900/20' : 'border-mev-primary bg-green-50 dark:bg-green-900/20'}`}>
          <div className="text-lg font-semibold mb-2">Profitability</div>
          <div className={`text-2xl font-bold ${parseFloat(metrics.totalProfit) < 0 ? 'text-mev-danger' : 'text-mev-primary'}`}>
            {parseFloat(metrics.totalProfit) < 0 ? 'LOSS' : 'PROFIT'}
          </div>
          <div className="text-sm text-gray-600 mt-1">
            Net {parseFloat(metrics.totalProfit) < 0 ? 'loss' : 'gain'}: {Math.abs(parseFloat(formatETH(metrics.totalProfit)))} ETH
          </div>
        </div>
      </div>
    </div>
  );
}; 