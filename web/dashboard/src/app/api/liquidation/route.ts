import { NextRequest, NextResponse } from 'next/server';

// Mock data that would come from the Go backend liquidation detection system
const mockLiquidationOpportunities = [
  {
    id: 'liq-001',
    strategy: 'liquidation' as const,
    targetTx: '0x742d35cc6634c0532925a3b8d265ab32be9c6ca1',
    expectedProfit: '450000000000000000000', // 450 USD in wei
    gasCost: '30000000000000000000',        // 30 USD in wei
    netProfit: '420000000000000000000',     // 420 USD in wei
    confidence: 0.92,
    status: 'validated' as const,
    createdAt: new Date(Date.now() - 5000).toISOString(),
    executionTxs: [],
    blockNumber: 18950000,
    
    // Liquidation-specific fields
    protocol: 'aave_v2',
    borrower: '0x742d35Cc6634C0532925a3b8D265AB32Be9C6CA1',
    collateralToken: 'WETH',
    debtToken: 'USDC',
    collateralAmount: '5000000000000000000', // 5 ETH
    debtAmount: '8000000000',               // 8000 USDC
    healthFactor: 0.85,
    liquidationBonus: '500000000000000000000', // 500 USD
    flashLoanFee: '9000000000000000000',       // 9 USD
    riskScore: 0.25,
    validationScore: 0.92,
    expiresAt: new Date(Date.now() + 300000).toISOString(), // 5 minutes
  },
  {
    id: 'liq-002',
    strategy: 'liquidation' as const,
    targetTx: '0x8ba1f109551bd432803012645bd57fbc74f86e2b',
    expectedProfit: '280000000000000000000',
    gasCost: '25000000000000000000',
    netProfit: '255000000000000000000',
    confidence: 0.87,
    status: 'detected' as const,
    createdAt: new Date(Date.now() - 12000).toISOString(),
    executionTxs: [],
    blockNumber: 18950001,
    
    protocol: 'compound_v2',
    borrower: '0x8ba1f109551bd432803012645bd57fbc74f86e2b',
    collateralToken: 'WBTC',
    debtToken: 'DAI',
    collateralAmount: '150000000', // 1.5 WBTC
    debtAmount: '45000000000000000000000', // 45,000 DAI
    healthFactor: 0.92,
    liquidationBonus: '300000000000000000000',
    flashLoanFee: '20000000000000000000',
    riskScore: 0.18,
    validationScore: 0.87,
    expiresAt: new Date(Date.now() + 240000).toISOString(),
  },
  {
    id: 'liq-003',
    strategy: 'liquidation' as const,
    targetTx: '0x47ac0fb4f2d84898e4d9e7b4dab3c24507a6d503',
    expectedProfit: '750000000000000000000',
    gasCost: '40000000000000000000',
    netProfit: '710000000000000000000',
    confidence: 0.95,
    status: 'profitable' as const,
    createdAt: new Date(Date.now() - 8000).toISOString(),
    executionTxs: [],
    blockNumber: 18950002,
    
    protocol: 'aave_v3',
    borrower: '0x47ac0fb4f2d84898e4d9e7b4dab3c24507a6d503',
    collateralToken: 'LINK',
    debtToken: 'USDT',
    collateralAmount: '5000000000000000000000', // 5000 LINK
    debtAmount: '60000000000', // 60,000 USDT
    healthFactor: 0.78,
    liquidationBonus: '800000000000000000000',
    flashLoanFee: '50000000000000000000',
    riskScore: 0.35,
    validationScore: 0.95,
    expiresAt: new Date(Date.now() + 180000).toISOString(),
  }
];

const mockLiquidationMetrics = {
  totalOpportunities: 3,
  averageProfit: '460000000000000000000', // 460 USD
  averageHealthFactor: 0.85,
  protocolDistribution: {
    'aave_v2': 1,
    'aave_v3': 1,
    'compound_v2': 1,
  },
  riskDistribution: {
    low: 1,
    medium: 2,
    high: 0,
    critical: 0,
  },
  flashLoanProviderUsage: {
    'aave_v2': 2,
    'dydx': 1,
    'balancer': 0,
  },
};

const mockProtocolStatus = [
  {
    name: 'Aave V2',
    version: '2.0',
    chainId: 1,
    isActive: true,
    totalPositions: 150,
    liquidatablePositions: 1,
    totalCollateralValue: '15000000000000000000000000', // 15M USD
    totalDebtValue: '12000000000000000000000000',      // 12M USD
    averageHealthFactor: 1.75,
    lastUpdateTime: new Date().toISOString(),
  },
  {
    name: 'Aave V3',
    version: '3.0',
    chainId: 1,
    isActive: true,
    totalPositions: 89,
    liquidatablePositions: 1,
    totalCollateralValue: '8500000000000000000000000',
    totalDebtValue: '6800000000000000000000000',
    averageHealthFactor: 1.68,
    lastUpdateTime: new Date().toISOString(),
  },
  {
    name: 'Compound V2',
    version: '2.0',
    chainId: 1,
    isActive: true,
    totalPositions: 234,
    liquidatablePositions: 1,
    totalCollateralValue: '22000000000000000000000000',
    totalDebtValue: '18500000000000000000000000',
    averageHealthFactor: 1.82,
    lastUpdateTime: new Date().toISOString(),
  },
];

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const type = searchParams.get('type');

    // Simulate network delay
    await new Promise(resolve => setTimeout(resolve, 100));

    switch (type) {
      case 'opportunities':
        return NextResponse.json({
          success: true,
          data: mockLiquidationOpportunities,
          timestamp: new Date().toISOString(),
        });

      case 'metrics':
        return NextResponse.json({
          success: true,
          data: mockLiquidationMetrics,
          timestamp: new Date().toISOString(),
        });

      case 'protocols':
        return NextResponse.json({
          success: true,
          data: mockProtocolStatus,
          timestamp: new Date().toISOString(),
        });

      case 'validation':
        const opportunityId = searchParams.get('id');
        if (!opportunityId) {
          return NextResponse.json(
            { success: false, error: 'Missing opportunity ID' },
            { status: 400 }
          );
        }

        const validationResult = {
          isValid: true,
          validationScore: 0.92,
          confidence: 0.89,
          profitabilityCheck: true,
          healthFactorCheck: true,
          collateralValueCheck: true,
          flashLoanAvailabilityCheck: true,
          riskAssessment: {
            overallRisk: 0.25,
            riskLevel: 'medium',
            riskComponents: [
              {
                name: 'health_factor_risk',
                score: 0.2,
                weight: 0.4,
                description: 'Risk of health factor recovery before execution',
              },
              {
                name: 'gas_risk',
                score: 0.3,
                weight: 0.3,
                description: 'Risk of gas price spikes reducing profitability',
              },
              {
                name: 'competition_risk',
                score: 0.4,
                weight: 0.2,
                description: 'Risk of other liquidators executing first',
              },
              {
                name: 'slippage_risk',
                score: 0.1,
                weight: 0.1,
                description: 'Risk of slippage during collateral conversion',
              },
            ],
          },
          recommendations: [
            'Execute quickly due to moderate competition risk',
            'Monitor gas prices for optimal execution timing',
            'Use flash loan from dYdX for minimal fees',
          ],
          estimatedExecutionTime: 150, // milliseconds
        };

        return NextResponse.json({
          success: true,
          data: validationResult,
          timestamp: new Date().toISOString(),
        });

      default:
        return NextResponse.json({
          success: true,
          data: {
            opportunities: mockLiquidationOpportunities,
            metrics: mockLiquidationMetrics,
            protocols: mockProtocolStatus,
          },
          timestamp: new Date().toISOString(),
        });
    }
  } catch (error) {
    console.error('Liquidation API error:', error);
    return NextResponse.json(
      { 
        success: false, 
        error: error instanceof Error ? error.message : 'Internal server error' 
      },
      { status: 500 }
    );
  }
}

// POST endpoint for executing liquidations (would integrate with Go backend)
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const { opportunityId, executionMode = 'simulation' } = body;

    if (!opportunityId) {
      return NextResponse.json(
        { success: false, error: 'Missing opportunity ID' },
        { status: 400 }
      );
    }

    // Simulate execution (in real implementation, this would call the Go backend)
    const executionResult = {
      success: true,
      mode: executionMode,
      realizedProfit: '418000000000000000000', // Slightly different from expected
      actualGasCost: '32000000000000000000',   // Slightly higher gas
      actualNetProfit: '386000000000000000000',
      executionTime: 147,
      slippage: 0.008,
      submittedTxs: [
        {
          hash: '0x1234567890abcdef1234567890abcdef12345678',
          from: '0x742d35Cc6634C0532925a3b8D265AB32Be9C6CA1',
          to: '0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9', // Aave V2 LendingPool
          value: '0',
          gasPrice: '32000000000',
          gasLimit: 400000,
          data: '0x...', // Liquidation call data
        },
      ],
    };

    return NextResponse.json({
      success: true,
      data: executionResult,
      timestamp: new Date().toISOString(),
    });
  } catch (error) {
    console.error('Liquidation execution error:', error);
    return NextResponse.json(
      { 
        success: false, 
        error: error instanceof Error ? error.message : 'Execution failed' 
      },
      { status: 500 }
    );
  }
} 