export type StrategyType = 
  | 'sandwich' 
  | 'backrun' 
  | 'frontrun' 
  | 'arbitrage' 
  | 'cross_layer'
  | 'time_bandit'
  | 'liquidation';

export type OpportunityStatus = 
  | 'pending' 
  | 'simulated' 
  | 'profitable' 
  | 'unprofitable' 
  | 'executed' 
  | 'failed'
  | 'detected'
  | 'validated';

// NEW: Execution modes
export type ExecutionMode = 
  | 'simulation'  // Paper trading only
  | 'hybrid'      // Simulate first, execute live if profitable
  | 'live';       // Direct live execution

export interface Transaction {
  hash: string;
  from?: string;
  to?: string;
  value?: string;
  gasPrice?: string;
  gasLimit?: number;
  nonce?: number;
  data?: string;
  chainId?: number;
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
  
  // NEW: Execution-related fields
  executionMode?: ExecutionMode;
  executionResult?: ExecutionResult;
  simulationComparison?: SimulationComparison;
}

// NEW: Execution result data
export interface ExecutionResult {
  success: boolean;
  mode: ExecutionMode;
  realizedProfit: string;
  actualGasCost: string;
  actualNetProfit: string;
  executionTime: number; // milliseconds
  slippage: number;
  error?: string;
  submittedTxs?: Transaction[];
  confirmedTxs?: TransactionReceipt[];
}

// NEW: Simulation vs live comparison
export interface SimulationComparison {
  simulationSuccess: boolean;
  profitDifference: string; // Actual - Expected
  accuracyScore: number; // 0-1
  executionTimeDifference?: number; // Live - Simulation time
}

// NEW: Transaction receipt
export interface TransactionReceipt {
  hash: string;
  status: number;
  blockNumber: string;
  gasUsed: number;
  effectiveGasPrice: string;
  confirmationTime: number;
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
  
  // NEW: Execution-specific metrics
  executionMode?: ExecutionMode;
  totalExecutions?: number;
  successfulExecutions?: number;
  failedExecutions?: number;
  simulationSuccessRate?: number;
  liveSuccessRate?: number;
  totalExecutionProfit?: string;
  totalExecutionGasCost?: string;
  netExecutionProfit?: string;
  avgExecutionTime?: number;
  avgSlippage?: number;
  emergencyStop?: boolean;
  dailyVolumeUsed?: string;
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
  
  // NEW: Execution status
  executionMode?: ExecutionMode;
  currentLatency?: string;
  lastExecution?: string;
  pendingExecutions?: number;
  activePositions?: number;
  dailyVolumeUsed?: string;
  maxDailyVolume?: string;
}

export interface Alert {
  id: string;
  type: 'info' | 'warning' | 'error' | 'success';
  message: string;
  timestamp: string;
  acknowledged: boolean;
  component?: string;
  severity?: 'low' | 'medium' | 'high' | 'critical';
}

export interface HistoricalData {
  timestamp: string;
  metric: string;
  value: number;
}

// NEW: Execution-specific interfaces

export interface ExecutionStats {
  totalExecutions: number;
  successfulExecutions: number;
  failedExecutions: number;
  simulationSuccessRate: number;
  liveSuccessRate: number;
  totalProfit: string;
  totalGasCost: string;
  netProfit: string;
  averageProfitPerTrade: string;
  averageExecutionTime: number;
  averageSlippage: number;
  dailyStats: Record<string, DailyExecutionStats>;
}

export interface DailyExecutionStats {
  date: string;
  executions: number;
  profit: string;
  volume: string;
  successRate: number;
  averageExecutionTime: number;
}

export interface ExecutionConfig {
  mode: ExecutionMode;
  autoProgression: boolean;
  maxDailyVolume: string;
  maxPositionSize: string;
  minProfitThreshold: string;
  maxSlippage: number;
  maxGasPrice: string;
  executorAddress?: string;
  chainId: number;
}

export interface Position {
  token: string;
  amount: string;
  value: string;
  openedAt: string;
  strategy: StrategyType;
}

export interface PositionSummary {
  totalPositions: number;
  totalValue: string;
  positionsByToken: Record<string, string>;
  largestPosition?: Position;
}

// NEW: WebSocket message types for execution updates
export interface ExecutionUpdateMessage {
  opportunity: MEVOpportunity;
  executionMode: ExecutionMode;
  executionResult?: ExecutionResult;
  simulationComparison?: SimulationComparison;
}

// NEW: Execution progression data
export interface ExecutionProgression {
  currentMode: ExecutionMode;
  canAdvance: boolean;
  nextMode?: ExecutionMode;
  requirements: ProgressionRequirement[];
  timeInCurrentMode: number; // days
}

export interface ProgressionRequirement {
  name: string;
  current: number | string;
  required: number | string;
  unit: string;
  met: boolean;
  description: string;
}

// Liquidation-specific types
export interface LiquidationOpportunity extends MEVOpportunity {
  strategy: 'liquidation';
  protocol: string;
  borrower: string;
  collateralToken: string;
  debtToken: string;
  collateralAmount: string;
  debtAmount: string;
  healthFactor: number;
  liquidationBonus: string;
  flashLoanFee: string;
  riskScore: number;
  validationScore: number;
  expiresAt: string;
}

export interface LiquidationMetrics {
  totalOpportunities: number;
  averageProfit: string;
  averageHealthFactor: number;
  protocolDistribution: Record<string, number>;
  riskDistribution: {
    low: number;
    medium: number;
    high: number;
    critical: number;
  };
  flashLoanProviderUsage: Record<string, number>;
}

export interface ProtocolStatus {
  name: string;
  version: string;
  chainId: number;
  isActive: boolean;
  totalPositions: number;
  liquidatablePositions: number;
  totalCollateralValue: string;
  totalDebtValue: string;
  averageHealthFactor: number;
  lastUpdateTime: string;
}

export interface LiquidationValidationResult {
  isValid: boolean;
  validationScore: number;
  confidence: number;
  profitabilityCheck: boolean;
  healthFactorCheck: boolean;
  collateralValueCheck: boolean;
  flashLoanAvailabilityCheck: boolean;
  riskAssessment: {
    overallRisk: number;
    riskLevel: string;
    riskComponents: Array<{
      name: string;
      score: number;
      weight: number;
      description: string;
    }>;
  };
  recommendations: string[];
  estimatedExecutionTime: number;
} 