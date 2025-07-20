export enum StrategyType {
  SANDWICH = 'sandwich',
  BACKRUN = 'backrun', 
  FRONTRUN = 'frontrun',
  TIME_BANDIT = 'time_bandit',
  CROSS_LAYER = 'cross_layer'
}

export enum OpportunityStatus {
  PENDING = 'pending',
  SIMULATED = 'simulated',
  PROFITABLE = 'profitable',
  UNPROFITABLE = 'unprofitable',
  EXECUTED = 'executed',
  FAILED = 'failed'
}

export interface Transaction {
  hash: string;
  from: string;
  to: string;
  value: string;
  gasPrice: string;
  gasLimit: number;
  nonce: number;
  data: string;
  timestamp: string;
}

export interface MEVOpportunity {
  id: string;
  strategy: StrategyType;
  targetTx: string;
  expectedProfit: string;
  gasCost: string;
  netProfit: string;
  confidence: number;
  status: OpportunityStatus;
  createdAt: string;
  executionTxs: Transaction[];
  simulationLatency?: number;
  blockNumber?: number;
  poolAddress?: string;
  tokenAddresses?: string[];
  slippage?: number;
}

export interface PerformanceMetrics {
  windowSize: number;
  totalTrades: number;
  profitableTrades: number;
  lossRate: number;
  avgLatency: number;
  totalProfit: string;
  lastUpdated: string;
  successRate: number;
  avgProfitPerTrade: string;
}

export interface SystemStatus {
  isConnected: boolean;
  mempoolConnections: number;
  activeSimulations: number;
  queueSize: number;
  lastBlockNumber: number;
  uptime: number;
  memoryUsage: number;
  cpuUsage: number;
}

export interface HistoricalData {
  timestamp: string;
  opportunities: number;
  profitableOps: number;
  totalProfit: string;
  avgLatency: number;
}

export interface Alert {
  id: string;
  type: 'warning' | 'error' | 'info';
  message: string;
  timestamp: string;
  acknowledged: boolean;
} 