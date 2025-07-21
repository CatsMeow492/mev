'use client';

import React, { useState, useEffect } from 'react';
import { LiquidationOpportunity, LiquidationMetrics, ProtocolStatus, LiquidationValidationResult } from '@/types/mev';

interface LiquidationPanelProps {
  className?: string;
}

export default function LiquidationPanel({ className = '' }: LiquidationPanelProps) {
  const [opportunities, setOpportunities] = useState<LiquidationOpportunity[]>([]);
  const [metrics, setMetrics] = useState<LiquidationMetrics | null>(null);
  const [protocols, setProtocols] = useState<ProtocolStatus[]>([]);
  const [selectedOpportunity, setSelectedOpportunity] = useState<LiquidationOpportunity | null>(null);
  const [validationResult, setValidationResult] = useState<LiquidationValidationResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchLiquidationData();
    const interval = setInterval(fetchLiquidationData, 5000); // Refresh every 5 seconds
    return () => clearInterval(interval);
  }, []);

  const fetchLiquidationData = async () => {
    try {
      const [opportunitiesRes, metricsRes, protocolsRes] = await Promise.all([
        fetch('/api/liquidation?type=opportunities'),
        fetch('/api/liquidation?type=metrics'),
        fetch('/api/liquidation?type=protocols'),
      ]);

      const [opportunitiesData, metricsData, protocolsData] = await Promise.all([
        opportunitiesRes.json(),
        metricsRes.json(),
        protocolsRes.json(),
      ]);

      if (opportunitiesData.success) setOpportunities(opportunitiesData.data);
      if (metricsData.success) setMetrics(metricsData.data);
      if (protocolsData.success) setProtocols(protocolsData.data);
      
      setLoading(false);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch liquidation data');
      setLoading(false);
    }
  };

  const validateOpportunity = async (opportunity: LiquidationOpportunity) => {
    try {
      const response = await fetch(`/api/liquidation?type=validation&id=${opportunity.id}`);
      const data = await response.json();
      if (data.success) {
        setValidationResult(data.data);
        setSelectedOpportunity(opportunity);
      }
    } catch (err) {
      console.error('Validation failed:', err);
    }
  };

  const executeOpportunity = async (opportunity: LiquidationOpportunity, mode = 'simulation') => {
    try {
      const response = await fetch('/api/liquidation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ opportunityId: opportunity.id, executionMode: mode }),
      });
      
      const data = await response.json();
      if (data.success) {
        alert(`Liquidation ${mode} successful! Net profit: ${formatCurrency(data.data.actualNetProfit)}`);
        fetchLiquidationData(); // Refresh data
      }
    } catch (err) {
      alert(`Execution failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }
  };

  const formatCurrency = (weiAmount: string): string => {
    const value = parseFloat(weiAmount) / 1e18;
    return `$${value.toFixed(2)}`;
  };

  const formatAddress = (address: string): string => {
    return `${address.slice(0, 6)}...${address.slice(-4)}`;
  };

  const getHealthFactorColor = (hf: number): string => {
    if (hf < 1.0) return 'text-red-500';
    if (hf < 1.2) return 'text-yellow-500';
    return 'text-green-500';
  };

  const getRiskColor = (risk: number): string => {
    if (risk < 0.2) return 'text-green-500';
    if (risk < 0.5) return 'text-yellow-500';
    if (risk < 0.8) return 'text-orange-500';
    return 'text-red-500';
  };

  if (loading) {
    return (
      <div className={`bg-white rounded-lg shadow-md p-6 ${className}`}>
        <div className="animate-pulse">
          <div className="h-4 bg-gray-300 rounded w-1/4 mb-4"></div>
          <div className="space-y-3">
            <div className="h-4 bg-gray-300 rounded"></div>
            <div className="h-4 bg-gray-300 rounded w-5/6"></div>
            <div className="h-4 bg-gray-300 rounded w-4/6"></div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={`bg-white rounded-lg shadow-md p-6 ${className}`}>
        <div className="text-red-500">Error: {error}</div>
      </div>
    );
  }

  return (
    <div className={`bg-white rounded-lg shadow-md ${className}`}>
      {/* Header */}
      <div className="bg-gradient-to-r from-purple-600 to-indigo-600 text-white p-6 rounded-t-lg">
        <h2 className="text-2xl font-bold mb-2">Liquidation Detection</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          <div>
            <div className="opacity-80">Total Opportunities</div>
            <div className="text-xl font-semibold">{metrics?.totalOpportunities || 0}</div>
          </div>
          <div>
            <div className="opacity-80">Avg Profit</div>
            <div className="text-xl font-semibold">{metrics ? formatCurrency(metrics.averageProfit) : '$0'}</div>
          </div>
          <div>
            <div className="opacity-80">Avg Health Factor</div>
            <div className="text-xl font-semibold">{metrics?.averageHealthFactor.toFixed(3) || '0.000'}</div>
          </div>
          <div>
            <div className="opacity-80">Active Protocols</div>
            <div className="text-xl font-semibold">{protocols.filter(p => p.isActive).length}</div>
          </div>
        </div>
      </div>

      <div className="p-6 space-y-6">
        {/* Protocol Status */}
        <div>
          <h3 className="text-lg font-semibold mb-3">Protocol Status</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {protocols.map((protocol, index) => (
              <div key={index} className="border rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <h4 className="font-semibold">{protocol.name}</h4>
                  <span className={`px-2 py-1 rounded text-xs ${protocol.isActive ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
                    {protocol.isActive ? 'Active' : 'Inactive'}
                  </span>
                </div>
                <div className="space-y-1 text-sm text-gray-600">
                  <div>Positions: {protocol.totalPositions}</div>
                  <div>Liquidatable: <span className="font-semibold text-red-600">{protocol.liquidatablePositions}</span></div>
                  <div>Avg HF: <span className={getHealthFactorColor(protocol.averageHealthFactor)}>{protocol.averageHealthFactor.toFixed(2)}</span></div>
                  <div>TVL: {formatCurrency(protocol.totalCollateralValue)}</div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Risk Distribution */}
        {metrics && (
          <div>
            <h3 className="text-lg font-semibold mb-3">Risk Distribution</h3>
            <div className="grid grid-cols-4 gap-4">
              <div className="text-center">
                <div className="text-2xl font-bold text-green-500">{metrics.riskDistribution.low}</div>
                <div className="text-sm text-gray-600">Low Risk</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-yellow-500">{metrics.riskDistribution.medium}</div>
                <div className="text-sm text-gray-600">Medium Risk</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-orange-500">{metrics.riskDistribution.high}</div>
                <div className="text-sm text-gray-600">High Risk</div>
              </div>
              <div className="text-center">
                <div className="text-2xl font-bold text-red-500">{metrics.riskDistribution.critical}</div>
                <div className="text-sm text-gray-600">Critical Risk</div>
              </div>
            </div>
          </div>
        )}

        {/* Liquidation Opportunities */}
        <div>
          <h3 className="text-lg font-semibold mb-3">Active Opportunities</h3>
          {opportunities.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No liquidation opportunities detected
            </div>
          ) : (
            <div className="space-y-4">
              {opportunities.map((opportunity) => (
                <div key={opportunity.id} className="border rounded-lg p-4 hover:shadow-md transition-shadow">
                  <div className="grid grid-cols-1 md:grid-cols-6 gap-4 items-center">
                    <div>
                      <div className="font-semibold">{opportunity.protocol.toUpperCase()}</div>
                      <div className="text-sm text-gray-600">{formatAddress(opportunity.borrower)}</div>
                    </div>
                    <div>
                      <div className="font-semibold">{opportunity.collateralToken}/{opportunity.debtToken}</div>
                      <div className="text-sm text-gray-600">
                        HF: <span className={getHealthFactorColor(opportunity.healthFactor)}>{opportunity.healthFactor.toFixed(3)}</span>
                      </div>
                    </div>
                    <div>
                      <div className="font-semibold text-green-600">{formatCurrency(opportunity.netProfit)}</div>
                      <div className="text-sm text-gray-600">Net Profit</div>
                    </div>
                    <div>
                      <div className="font-semibold">Risk: <span className={getRiskColor(opportunity.riskScore)}>{(opportunity.riskScore * 100).toFixed(0)}%</span></div>
                      <div className="text-sm text-gray-600">Confidence: {(opportunity.confidence * 100).toFixed(0)}%</div>
                    </div>
                    <div>
                      <span className={`px-2 py-1 rounded text-xs ${
                        opportunity.status === 'profitable' ? 'bg-green-100 text-green-800' :
                        opportunity.status === 'validated' ? 'bg-blue-100 text-blue-800' :
                        'bg-yellow-100 text-yellow-800'
                      }`}>
                        {opportunity.status}
                      </span>
                    </div>
                    <div className="space-x-2">
                      <button
                        onClick={() => validateOpportunity(opportunity)}
                        className="px-3 py-1 bg-blue-500 text-white rounded text-sm hover:bg-blue-600"
                      >
                        Validate
                      </button>
                      <button
                        onClick={() => executeOpportunity(opportunity, 'simulation')}
                        className="px-3 py-1 bg-green-500 text-white rounded text-sm hover:bg-green-600"
                      >
                        Simulate
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Validation Results Modal */}
        {selectedOpportunity && validationResult && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-white rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto">
              <div className="flex justify-between items-center mb-4">
                <h3 className="text-xl font-semibold">Validation Results</h3>
                <button
                  onClick={() => { setSelectedOpportunity(null); setValidationResult(null); }}
                  className="text-gray-500 hover:text-gray-700"
                >
                  ✕
                </button>
              </div>
              
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="text-sm text-gray-600">Validation Score</div>
                    <div className="text-2xl font-bold text-green-600">{(validationResult.validationScore * 100).toFixed(0)}%</div>
                  </div>
                  <div>
                    <div className="text-sm text-gray-600">Confidence</div>
                    <div className="text-2xl font-bold text-blue-600">{(validationResult.confidence * 100).toFixed(0)}%</div>
                  </div>
                </div>

                <div>
                  <h4 className="font-semibold mb-2">Validation Checks</h4>
                  <div className="grid grid-cols-2 gap-2">
                    <div className={`flex items-center ${validationResult.profitabilityCheck ? 'text-green-600' : 'text-red-600'}`}>
                      {validationResult.profitabilityCheck ? '✓' : '✗'} Profitability
                    </div>
                    <div className={`flex items-center ${validationResult.healthFactorCheck ? 'text-green-600' : 'text-red-600'}`}>
                      {validationResult.healthFactorCheck ? '✓' : '✗'} Health Factor
                    </div>
                    <div className={`flex items-center ${validationResult.collateralValueCheck ? 'text-green-600' : 'text-red-600'}`}>
                      {validationResult.collateralValueCheck ? '✓' : '✗'} Collateral Value
                    </div>
                    <div className={`flex items-center ${validationResult.flashLoanAvailabilityCheck ? 'text-green-600' : 'text-red-600'}`}>
                      {validationResult.flashLoanAvailabilityCheck ? '✓' : '✗'} Flash Loan Available
                    </div>
                  </div>
                </div>

                <div>
                  <h4 className="font-semibold mb-2">Risk Assessment</h4>
                  <div className="mb-2">
                    <div className="text-sm text-gray-600">Overall Risk: <span className={getRiskColor(validationResult.riskAssessment.overallRisk)}>{validationResult.riskAssessment.riskLevel}</span></div>
                  </div>
                  <div className="space-y-2">
                    {validationResult.riskAssessment.riskComponents.map((component, index) => (
                      <div key={index} className="flex justify-between items-center">
                        <span className="text-sm">{component.name.replace(/_/g, ' ')}</span>
                        <div className="flex items-center">
                          <span className={`text-sm font-semibold ${getRiskColor(component.score)}`}>
                            {(component.score * 100).toFixed(0)}%
                          </span>
                          <span className="text-xs text-gray-500 ml-2">({(component.weight * 100).toFixed(0)}% weight)</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h4 className="font-semibold mb-2">Recommendations</h4>
                  <ul className="list-disc list-inside space-y-1 text-sm">
                    {validationResult.recommendations.map((rec, index) => (
                      <li key={index}>{rec}</li>
                    ))}
                  </ul>
                </div>

                <div className="flex space-x-3 pt-4">
                  <button
                    onClick={() => executeOpportunity(selectedOpportunity, 'simulation')}
                    className="flex-1 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
                  >
                    Simulate Execution
                  </button>
                  <button
                    onClick={() => executeOpportunity(selectedOpportunity, 'live')}
                    className="flex-1 px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600"
                  >
                    Execute Live
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
} 