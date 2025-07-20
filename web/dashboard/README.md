# MEV Engine Dashboard

A real-time dashboard for monitoring Maximum Extractable Value (MEV) opportunities on Base Layer 2 network. Built with Next.js, TypeScript, and Tailwind CSS.

## Features

### Real-Time Monitoring
- Live WebSocket connection to MEV engine backend
- Real-time opportunity detection and display
- Automatic reconnection with exponential backoff
- Connection status indicators

### Opportunity Management
- Display of MEV opportunities with detailed information
- Strategy-based filtering (Sandwich, Backrun, Frontrun, Time Bandit, Cross Layer)
- Status-based filtering (Pending, Simulated, Profitable, Unprofitable, etc.)
- Sorting by time, profit, and confidence
- Detailed opportunity modal with execution transactions

### Performance Metrics
- Real-time performance tracking
- Success rate and loss rate monitoring
- Latency measurements
- Profit/loss calculations
- System health indicators

### System Status
- Live system metrics (CPU, memory, queue size)
- Alert management with acknowledgment
- Emergency controls (restart, shutdown)
- Mempool connection monitoring

## Technology Stack

- **Framework**: Next.js 14 with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **Real-time**: Socket.IO client
- **HTTP Client**: Axios
- **Testing**: Jest + React Testing Library + Playwright
- **Linting**: ESLint

## Getting Started

### Prerequisites

- Node.js 18+ 
- npm or yarn
- MEV Engine backend running on port 8080

### Installation

1. Install dependencies:
```bash
npm install
```

2. Start the development server:
```bash
npm run dev
```

3. Open [http://localhost:3000](http://localhost:3000) in your browser.

### Build for Production

```bash
npm run build
npm start
```

## Project Structure

```
src/
├── app/                 # Next.js App Router pages
│   ├── globals.css     # Global styles and Tailwind
│   ├── layout.tsx      # Root layout
│   └── page.tsx        # Main dashboard page
├── components/         # React components
│   ├── OpportunityCard.tsx
│   ├── OpportunityList.tsx
│   ├── MetricsPanel.tsx
│   ├── SystemStatus.tsx
│   └── __tests__/      # Component tests
├── hooks/             # Custom React hooks
│   ├── useWebSocket.ts # WebSocket connection management
│   └── useApi.ts       # REST API interactions
├── types/             # TypeScript type definitions
│   └── mev.ts         # MEV-related types
└── utils/             # Utility functions
    └── dateUtils.ts   # Date formatting utilities
```

## Configuration

### Environment Variables

Create a `.env.local` file:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080
```

### Tailwind Configuration

The dashboard uses a custom Tailwind theme with MEV-specific colors:

- `mev-primary`: Green (#10B981)
- `mev-secondary`: Blue (#3B82F6) 
- `mev-warning`: Orange (#F59E0B)
- `mev-danger`: Red (#EF4444)
- `mev-dark`: Dark gray (#1F2937)
- `mev-darker`: Darker gray (#111827)

## API Integration

### WebSocket Events

The dashboard connects to the backend WebSocket server and listens for:

- `opportunity`: New MEV opportunity detected
- `opportunity_update`: Status change for existing opportunity
- `metrics`: Performance metrics update
- `system_status`: System status update
- `alert`: New system alert

### REST Endpoints

- `GET /api/opportunities`: Fetch opportunities with filtering
- `GET /api/metrics`: Get current performance metrics
- `GET /api/status`: Get system status
- `POST /api/shutdown`: Emergency shutdown
- `POST /api/restart`: Restart system

## Testing

### Unit Tests

```bash
npm test                # Run tests once
npm run test:watch     # Run tests in watch mode
```

### End-to-End Tests

```bash
npm run test:e2e       # Run Playwright tests
```

## Performance Considerations

### Real-Time Updates
- WebSocket connection management with automatic reconnection
- Efficient state updates to prevent unnecessary re-renders
- Opportunity list limited to last 100 items for performance

### Responsive Design
- Mobile-first responsive design
- Optimized layouts for different screen sizes
- Touch-friendly controls

### Accessibility
- ARIA labels and semantic HTML
- Keyboard navigation support
- Color contrast compliance
- Screen reader compatibility

## Monitoring Features

### Opportunity Tracking
- Transaction hash and strategy type display
- Expected profit vs actual profit comparison
- Gas cost estimates and execution latency
- Confidence scoring for opportunity validation

### Safety Mechanisms
- Automatic shutdown triggers based on loss rates
- Alert system for system anomalies
- Manual emergency controls
- Performance degradation warnings

### Metrics Dashboard
- Rolling window calculations (50, 100, 500 trades)
- Success rate tracking
- Latency monitoring (target <100ms)
- Profit/loss analysis

## Contributing

1. Follow the existing code style and patterns
2. Add tests for new features
3. Update documentation as needed
4. Ensure all tests pass before submitting

## License

This project is part of the MEV Strategy Engine and follows the same licensing terms. 