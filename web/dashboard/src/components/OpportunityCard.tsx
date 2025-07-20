'use client';

import { MEVOpportunity, StrategyType, OpportunityStatus } from '@/types/mev';
import { formatDistanceToNow } from '@/utils/dateUtils';

interface OpportunityCardProps {
  opportunity: MEVOpportunity;
  onClick?: () => void;
}

const strategyLabels: Record<StrategyType, string> = {
  [StrategyType.SANDWICH]: 'Sandwich',
  [StrategyType.BACKRUN]: 'Backrun',
  [StrategyType.FRONTRUN]: 'Frontrun',
  [StrategyType.TIME_BANDIT]: 'Time Bandit',
  [StrategyType.CROSS_LAYER]: 'Cross Layer',
};

const strategyColors: Record<StrategyType, string> = {
  [StrategyType.SANDWICH]: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  [StrategyType.BACKRUN]: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  [StrategyType.FRONTRUN]: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  [StrategyType.TIME_BANDIT]: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  [StrategyType.CROSS_LAYER]: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
};

const formatETH = (value: string): string => {
  const num = parseFloat(value) / 1e18;
  return num.toFixed(6);
};

const formatGwei = (value: string): string => {
  const num = parseFloat(value) / 1e9;
  return num.toFixed(2);
};

export const OpportunityCard: React.FC<OpportunityCardProps> = ({ opportunity, onClick }) => {
  const isProfitable = opportunity.status === OpportunityStatus.PROFITABLE;
  const netProfitETH = parseFloat(opportunity.netProfit) / 1e18;

  return (
    <div
      className={`card cursor-pointer transition-all duration-200 hover:scale-[1.02] ${
        isProfitable ? 'profit-glow border-mev-primary' : ''
      }`}
      onClick={onClick}
    >
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3">
          <span className={`status-pill ${strategyColors[opportunity.strategy]}`}>
            {strategyLabels[opportunity.strategy]}
          </span>
          <span className={`status-${opportunity.status}`}>
            {opportunity.status.toUpperCase()}
          </span>
        </div>
        <div className="text-right">
          <div className={`text-lg font-bold ${netProfitETH > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
            {netProfitETH > 0 ? '+' : ''}{netProfitETH.toFixed(6)} ETH
          </div>
          <div className="text-sm text-gray-500">
            Confidence: {(opportunity.confidence * 100).toFixed(1)}%
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-4">
        <div>
          <div className="text-sm text-gray-500">Target Tx</div>
          <div className="font-mono text-sm truncate">
            {opportunity.targetTx}
          </div>
        </div>
        <div>
          <div className="text-sm text-gray-500">Created</div>
          <div className="text-sm">
            {formatDistanceToNow(new Date(opportunity.createdAt), { addSuffix: true })}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4 text-sm">
        <div>
          <div className="text-gray-500">Expected Profit</div>
          <div className="font-semibold text-mev-primary">
            {formatETH(opportunity.expectedProfit)} ETH
          </div>
        </div>
        <div>
          <div className="text-gray-500">Gas Cost</div>
          <div className="font-semibold text-mev-warning">
            {formatGwei(opportunity.gasCost)} Gwei
          </div>
        </div>
        <div>
          <div className="text-gray-500">Latency</div>
          <div className="font-semibold">
            {opportunity.simulationLatency ? `${opportunity.simulationLatency}ms` : 'N/A'}
          </div>
        </div>
      </div>

      {opportunity.tokenAddresses && opportunity.tokenAddresses.length > 0 && (
        <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <div className="text-sm text-gray-500 mb-2">Tokens</div>
          <div className="flex flex-wrap gap-2">
            {opportunity.tokenAddresses.slice(0, 3).map((address, index) => (
              <span
                key={index}
                className="px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded text-xs font-mono"
              >
                {address.slice(0, 8)}...{address.slice(-6)}
              </span>
            ))}
            {opportunity.tokenAddresses.length > 3 && (
              <span className="px-2 py-1 bg-gray-100 dark:bg-gray-800 rounded text-xs">
                +{opportunity.tokenAddresses.length - 3} more
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  );
}; 