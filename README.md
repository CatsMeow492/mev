# L2 MEV Strategy Engine

A high-performance MEV (Maximum Extractable Value) detection and execution pipeline designed for Ethereum Layer 2 networks, specifically Base.

## Features

- Real-time mempool monitoring via WebSocket connections
- Transaction simulation using local EVM forks (Anvil)
- Multiple MEV strategy detection (sandwich, backrun, frontrun, time bandit)
- Profit estimation and risk assessment
- Performance monitoring with automatic shutdown
- REST API and WebSocket interface
- Docker-based development environment

## Project Structure

```
.
├── cmd/
│   └── mev-engine/          # Application entry point
├── internal/
│   ├── app/                 # Application setup and dependency injection
│   └── config/              # Configuration management
├── pkg/
│   ├── interfaces/          # Core interfaces and contracts
│   └── types/               # Common types and models
├── configs/                 # Configuration files
├── scripts/                 # Database initialization scripts
├── monitoring/              # Prometheus and Grafana configuration
├── Dockerfile               # Container build configuration
├── docker-compose.yml       # Development environment setup
└── go.mod                   # Go module definition
```

## Core Interfaces

### Mempool Monitoring
- `WebSocketConnection`: Manages WebSocket connections to RPC endpoints
- `TransactionStream`: Processes incoming transaction data
- `ConnectionManager`: Handles connection pooling and failover

### Transaction Processing
- `PriorityQueue`: Gas price-based transaction prioritization
- `TransactionFilter`: Filters transactions by relevance criteria
- `QueueManager`: Manages queue size and eviction policies

### Simulation Engine
- `ForkManager`: Manages Anvil fork instances
- `TransactionReplayer`: Executes transactions on fork environments
- `StateAnalyzer`: Measures transaction effects and state changes

### Strategy Detection
- `SandwichDetector`: Identifies sandwich attack opportunities
- `BackrunDetector`: Finds arbitrage opportunities from price gaps
- `FrontrunDetector`: Detects frontrunnable high-value transactions
- `TimeBanditDetector`: Analyzes transaction reordering opportunities

### Profit Estimation
- `ProfitCalculator`: Calculates expected profitability
- `GasEstimator`: Estimates gas costs for strategy execution
- `SlippageCalculator`: Models price impact and slippage

## Getting Started

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Foundry (for Anvil)

### Development Setup

1. Clone the repository
2. Copy and configure the config file:
   ```bash
   cp configs/config.yaml configs/config.local.yaml
   # Edit configs/config.local.yaml with your RPC endpoints
   ```

3. Start the development environment:
   ```bash
   docker-compose up -d
   ```

4. Build and run the application:
   ```bash
   go mod tidy
   go run cmd/mev-engine/main.go
   ```

### Configuration

The application uses a YAML configuration file with the following sections:

- **Server**: HTTP server configuration
- **RPC**: Base network RPC endpoints and connection settings
- **Simulation**: Anvil fork configuration
- **Strategies**: Strategy-specific parameters and thresholds
- **Queue**: Transaction queue settings
- **Monitoring**: Performance monitoring and alerting
- **Database**: Redis and PostgreSQL connection settings

### Environment Variables

Key environment variables for configuration:

- `RPC_BASE_URL`: Base network RPC URL
- `RPC_WEBSOCKET_URL`: Base network WebSocket URL
- `DATABASE_REDIS_URL`: Redis connection URL
- `DATABASE_POSTGRES_URL`: PostgreSQL connection URL

## Architecture

The system follows a microservices architecture with distinct components:

1. **Mempool Watcher**: Monitors Base mempool via WebSocket
2. **Transaction Queue**: Prioritizes transactions by gas price
3. **Simulation Engine**: Replays transactions on forked environment
4. **Strategy Engine**: Applies MEV detection strategies
5. **Profit Estimator**: Calculates profitability and risk
6. **Performance Monitor**: Tracks metrics and implements safety mechanisms

## Monitoring

The system includes comprehensive monitoring:

- **Prometheus**: Metrics collection
- **Grafana**: Visualization dashboards (available at http://localhost:3000)
- **Health Checks**: Application health monitoring
- **Performance Alerts**: Automatic shutdown on poor performance

## API Endpoints

- `GET /health`: Health check endpoint
- `GET /metrics`: Prometheus metrics endpoint
- `GET /api/v1/opportunities`: List detected MEV opportunities
- `GET /api/v1/stats`: System performance statistics
- `WebSocket /ws/opportunities`: Real-time opportunity stream

## Safety Features

- **Automatic Shutdown**: Stops trading when loss rate exceeds thresholds
- **Performance Monitoring**: Tracks profitability over rolling windows
- **Circuit Breaker**: Prevents cascade failures
- **Connection Resilience**: Automatic reconnection with exponential backoff

## Development

### Adding New Strategies

1. Define the strategy interface in `pkg/interfaces/strategy.go`
2. Implement the strategy detector
3. Add configuration in `internal/config/config.go`
4. Register the strategy in the strategy engine

### Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test -tags=integration ./...

# Run with coverage
go test -cover ./...
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This software is for educational and research purposes only. MEV extraction may be subject to legal and regulatory restrictions in your jurisdiction. Use at your own risk.