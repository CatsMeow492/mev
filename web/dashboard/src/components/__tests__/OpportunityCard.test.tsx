import { render, screen, fireEvent } from '@testing-library/react'
import { OpportunityCard } from '../OpportunityCard'
import { MEVOpportunity, StrategyType, OpportunityStatus } from '@/types/mev'

const mockOpportunity: MEVOpportunity = {
  id: 'test-id-123',
  strategy: StrategyType.SANDWICH,
  targetTx: '0x1234567890abcdef',
  expectedProfit: '1000000000000000000', // 1 ETH in wei
  gasCost: '50000000000', // 50 Gwei
  netProfit: '500000000000000000', // 0.5 ETH in wei
  confidence: 0.85,
  status: OpportunityStatus.PROFITABLE,
  createdAt: new Date().toISOString(),
  executionTxs: [],
  simulationLatency: 75,
  tokenAddresses: ['0xA0b86a33E6B2a3e8b41b0b9d5b', '0xC0d86a33E6B2a3e8b41b0b9d5b'],
}

describe('OpportunityCard', () => {
  it('renders opportunity information correctly', () => {
    render(<OpportunityCard opportunity={mockOpportunity} />)
    
    expect(screen.getByText('Sandwich')).toBeInTheDocument()
    expect(screen.getByText('PROFITABLE')).toBeInTheDocument()
    expect(screen.getByText('+0.500000 ETH')).toBeInTheDocument()
    expect(screen.getByText('Confidence: 85.0%')).toBeInTheDocument()
    expect(screen.getByText('75ms')).toBeInTheDocument()
  })

  it('applies profit glow class for profitable opportunities', () => {
    const { container } = render(<OpportunityCard opportunity={mockOpportunity} />)
    
    const card = container.firstChild as HTMLElement
    expect(card).toHaveClass('profit-glow')
    expect(card).toHaveClass('border-mev-primary')
  })

  it('calls onClick handler when clicked', () => {
    const mockOnClick = jest.fn()
    render(<OpportunityCard opportunity={mockOpportunity} onClick={mockOnClick} />)
    
    const card = screen.getByRole('button', { name: /sandwich/i }).closest('div')
    fireEvent.click(card!)
    
    expect(mockOnClick).toHaveBeenCalledTimes(1)
  })

  it('displays token addresses when available', () => {
    render(<OpportunityCard opportunity={mockOpportunity} />)
    
    expect(screen.getByText('Tokens')).toBeInTheDocument()
    expect(screen.getByText('0xA0b86a...')).toBeInTheDocument()
    expect(screen.getByText('0xC0d86a...')).toBeInTheDocument()
  })

  it('shows negative profit with danger styling', () => {
    const negativeOpportunity = {
      ...mockOpportunity,
      netProfit: '-100000000000000000', // -0.1 ETH
    }
    
    render(<OpportunityCard opportunity={negativeOpportunity} />)
    
    const profitElement = screen.getByText('-0.100000 ETH')
    expect(profitElement).toHaveClass('text-mev-danger')
  })
}) 