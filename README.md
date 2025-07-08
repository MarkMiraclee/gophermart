# Gophermart - Loyalty System

A modern loyalty system built with Go using clean architecture principles. Provides API for user registration, order management, and loyalty points accumulation.

## Features

- User registration and authentication with JWT tokens
- Order number upload with Luhn algorithm validation
- Automatic order verification through external loyalty points calculation system
- Loyalty balance management (points accrual and withdrawal)
- Order history and withdrawal tracking
- Integration with external loyalty points calculation system
- HTTP request/response compression (Gzip)
- Structured logging
- PostgreSQL as primary data storage

## Tech Stack

- **Language**: Go 1.23.10
- **Router**: Chi v5
- **Database**: PostgreSQL with pgx/v5
- **Authentication**: JWT (golang-jwt/jwt/v4)
- **HTTP Client**: Resty v2
- **Logging**: Logrus
- **Configuration**: env + flags
- **UUID**: Google UUID
- **Password Hashing**: bcrypt

## Architecture

The project follows clean architecture principles with layer separation:

- `cmd/` - application entry point
- `internal/` - internal application logic
  - `auth/` - authentication and JWT
  - `config/` - application configuration
  - `handlers/` - HTTP handlers
  - `middlewares/` - middleware components
  - `models/` - data models
  - `storage/` - data access layer
  - `accrual/` - external loyalty calculation system client
  - `luhn/` - order number validation

## Getting Started

### Prerequisites

- Go 1.23.10+
- PostgreSQL

### Installation

```bash
git clone <your-repo-url>
cd gophermart
go mod download
```

### Running

```bash
# Basic run
go run cmd/gophermart/main.go

# With server address configuration
go run cmd/gophermart/main.go -a localhost:8080

# With PostgreSQL connection
export DATABASE_URI="postgres://username:password@localhost:5432/gophermart?sslmode=disable"
go run cmd/gophermart/main.go

# With accrual system address
go run cmd/gophermart/main.go -r http://localhost:8081
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `RUN_ADDRESS` | HTTP server address | `localhost:8080` |
| `DATABASE_URI` | PostgreSQL connection string | - |
| `ACCRUAL_SYSTEM_ADDRESS` | Loyalty points calculation system address | - |
| `JWT_SECRET` | JWT secret key | `supersecretkey` |

## API Examples

### User Registration

```bash
curl -X POST http://localhost:8080/api/user/register \
  -H "Content-Type: application/json" \
  -d '{"login": "user@example.com", "password": "password123"}'
```

### User Authentication

```bash
curl -X POST http://localhost:8080/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"login": "user@example.com", "password": "password123"}'
```

### Upload Order Number

```bash
curl -X POST http://localhost:8080/api/user/orders \
  -H "Content-Type: text/plain" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d "12345678903"
```

### Get Orders List

```bash
curl -X GET http://localhost:8080/api/user/orders \
  -H "Authorization: Bearer <your-jwt-token>"
```

### Get Balance

```bash
curl -X GET http://localhost:8080/api/user/balance \
  -H "Authorization: Bearer <your-jwt-token>"
```

### Withdraw Points

```bash
curl -X POST http://localhost:8080/api/user/balance/withdraw \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{"order": "12345678903", "sum": 100.50}'
```

### Get Withdrawals History

```bash
curl -X GET http://localhost:8080/api/user/withdrawals \
  -H "Authorization: Bearer <your-jwt-token>"
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/handlers
```

## Database Structure

The system uses PostgreSQL with the following main tables:

- `users` - system users
- `orders` - user orders
- `withdrawals` - withdrawal history

## Loyalty Points System

The project integrates with an external loyalty points calculation system via HTTP API. The system automatically:

- Checks new orders every second
- Updates order statuses (NEW → PROCESSING → PROCESSED/INVALID)
- Accrues points according to external system response
- Handles rate limiting

## Security

- Passwords hashed using bcrypt
- JWT tokens for authentication
- Order number validation using Luhn algorithm
- Order uniqueness verification
- Protection against order reuse
