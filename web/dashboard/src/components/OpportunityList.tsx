'use client';

import { useState, useMemo } from 'react';
import { MEVOpportunity, StrategyType, OpportunityStatus } from '@/types/mev';
import { OpportunityCard } from './OpportunityCard';

interface OpportunityListProps {
  opportunities: MEVOpportunity[];
  onOpportunityClick?: (opportunity: MEVOpportunity) => void;
}

export const OpportunityList: React.FC<OpportunityListProps> = ({
  opportunities,
  onOpportunityClick,
}) => {
  const [filterStrategy, setFilterStrategy] = useState<StrategyType | 'all'>('all');
  const [filterStatus, setFilterStatus] = useState<OpportunityStatus | 'all'>('all');
  const [sortBy, setSortBy] = useState<'createdAt' | 'netProfit' | 'confidence'>('createdAt');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [showProfitableOnly, setShowProfitableOnly] = useState(false);

  const filteredAndSortedOpportunities = useMemo(() => {
    let filtered = opportunities;

    // Filter by strategy
    if (filterStrategy !== 'all') {
      filtered = filtered.filter(opp => opp.strategy === filterStrategy);
    }

    // Filter by status
    if (filterStatus !== 'all') {
      filtered = filtered.filter(opp => opp.status === filterStatus);
    }

    // Filter profitable only
    if (showProfitableOnly) {
      filtered = filtered.filter(opp => parseFloat(opp.netProfit) > 0);
    }

    // Sort
    return filtered.sort((a, b) => {
      let aValue: number;
      let bValue: number;

      switch (sortBy) {
        case 'netProfit':
          aValue = parseFloat(a.netProfit);
          bValue = parseFloat(b.netProfit);
          break;
        case 'confidence':
          aValue = a.confidence;
          bValue = b.confidence;
          break;
        case 'createdAt':
        default:
          aValue = new Date(a.createdAt).getTime();
          bValue = new Date(b.createdAt).getTime();
          break;
      }

      return sortOrder === 'desc' ? bValue - aValue : aValue - bValue;
    });
  }, [opportunities, filterStrategy, filterStatus, showProfitableOnly, sortBy, sortOrder]);

  const handleSortChange = (newSortBy: typeof sortBy) => {
    if (sortBy === newSortBy) {
      setSortOrder(sortOrder === 'desc' ? 'asc' : 'desc');
    } else {
      setSortBy(newSortBy);
      setSortOrder('desc');
    }
  };

  const profitableCount = opportunities.filter(opp => parseFloat(opp.netProfit) > 0).length;
  const totalValue = opportunities.reduce((sum, opp) => sum + parseFloat(opp.netProfit) / 1e18, 0);

  return (
    <div className="space-y-6">
      {/* Summary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="card text-center">
          <div className="text-2xl font-bold text-mev-primary">{opportunities.length}</div>
          <div className="text-sm text-gray-500">Total Opportunities</div>
        </div>
        <div className="card text-center">
          <div className="text-2xl font-bold text-mev-primary">{profitableCount}</div>
          <div className="text-sm text-gray-500">Profitable</div>
        </div>
        <div className="card text-center">
          <div className={`text-2xl font-bold ${totalValue > 0 ? 'text-mev-primary' : 'text-mev-danger'}`}>
            {totalValue > 0 ? '+' : ''}{totalValue.toFixed(6)} ETH
          </div>
          <div className="text-sm text-gray-500">Total Value</div>
        </div>
        <div className="card text-center">
          <div className="text-2xl font-bold text-mev-secondary">
            {opportunities.length > 0 ? ((profitableCount / opportunities.length) * 100).toFixed(1) : 0}%
          </div>
          <div className="text-sm text-gray-500">Success Rate</div>
        </div>
      </div>

      {/* Filters and Controls */}
      <div className="card">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex items-center space-x-2">
            <label className="text-sm font-medium">Strategy:</label>
            <select
              value={filterStrategy}
              onChange={(e) => setFilterStrategy(e.target.value as StrategyType | 'all')}
              className="px-3 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-sm"
            >
              <option value="all">All Strategies</option>
              <option value={StrategyType.SANDWICH}>Sandwich</option>
              <option value={StrategyType.BACKRUN}>Backrun</option>
              <option value={StrategyType.FRONTRUN}>Frontrun</option>
              <option value={StrategyType.TIME_BANDIT}>Time Bandit</option>
              <option value={StrategyType.CROSS_LAYER}>Cross Layer</option>
            </select>
          </div>

          <div className="flex items-center space-x-2">
            <label className="text-sm font-medium">Status:</label>
            <select
              value={filterStatus}
              onChange={(e) => setFilterStatus(e.target.value as OpportunityStatus | 'all')}
              className="px-3 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-sm"
            >
              <option value="all">All Status</option>
              <option value={OpportunityStatus.PENDING}>Pending</option>
              <option value={OpportunityStatus.SIMULATED}>Simulated</option>
              <option value={OpportunityStatus.PROFITABLE}>Profitable</option>
              <option value={OpportunityStatus.UNPROFITABLE}>Unprofitable</option>
              <option value={OpportunityStatus.EXECUTED}>Executed</option>
              <option value={OpportunityStatus.FAILED}>Failed</option>
            </select>
          </div>

          <div className="flex items-center space-x-2">
            <label className="text-sm font-medium">Sort by:</label>
            <button
              onClick={() => handleSortChange('createdAt')}
              className={`px-3 py-1 rounded text-sm ${sortBy === 'createdAt' ? 'bg-mev-primary text-white' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
              Time {sortBy === 'createdAt' && (sortOrder === 'desc' ? '↓' : '↑')}
            </button>
            <button
              onClick={() => handleSortChange('netProfit')}
              className={`px-3 py-1 rounded text-sm ${sortBy === 'netProfit' ? 'bg-mev-primary text-white' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
              Profit {sortBy === 'netProfit' && (sortOrder === 'desc' ? '↓' : '↑')}
            </button>
            <button
              onClick={() => handleSortChange('confidence')}
              className={`px-3 py-1 rounded text-sm ${sortBy === 'confidence' ? 'bg-mev-primary text-white' : 'bg-gray-200 dark:bg-gray-700'}`}
            >
              Confidence {sortBy === 'confidence' && (sortOrder === 'desc' ? '↓' : '↑')}
            </button>
          </div>

          <div className="flex items-center space-x-2">
            <input
              type="checkbox"
              id="profitable-only"
              checked={showProfitableOnly}
              onChange={(e) => setShowProfitableOnly(e.target.checked)}
              className="rounded"
            />
            <label htmlFor="profitable-only" className="text-sm font-medium">
              Profitable Only
            </label>
          </div>

          <div className="text-sm text-gray-500 ml-auto">
            Showing {filteredAndSortedOpportunities.length} of {opportunities.length} opportunities
          </div>
        </div>
      </div>

      {/* Opportunities List */}
      <div className="space-y-4">
        {filteredAndSortedOpportunities.length === 0 ? (
          <div className="card text-center text-gray-500 py-12">
            <div className="text-lg mb-2">No opportunities found</div>
            <div className="text-sm">
              {opportunities.length === 0 
                ? 'Waiting for MEV opportunities to be detected...'
                : 'Try adjusting your filters to see more results'
              }
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {filteredAndSortedOpportunities.map((opportunity) => (
              <OpportunityCard
                key={opportunity.id}
                opportunity={opportunity}
                onClick={() => onOpportunityClick?.(opportunity)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}; 