# Ignis - Code Execution API

Ignis is a scalable, production-ready backend API for executing code in various programming languages. It provides both authenticated user access and API key-based access for external integrations.

## Features

- **Multi-language Support**: Execute code in Python, Go, and other languages
- **Dual Authentication**: Support for Clerk user authentication and API key authentication
- **Asynchronous Execution**: Queue-based job processing with status tracking
- **Rate Limiting**: Built-in rate limiting with Redis support
- **Webhooks**: Real-time notifications for job completion
- **Health Monitoring**: Comprehensive health checks and metrics
- **Docker Support**: Containerized deployment with Docker Compose
- **Database**: PostgreSQL with GORM ORM
- **Message Queue**: NATS for job distribution

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │    │   Job Queue     │    │   Workers       │
│   (Gin/Gonic)   │───▶│   (NATS)        │───▶│   (Docker)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL    │    │     Redis       │    │   Webhooks      │
│   (Jobs, API    │    │   (Rate Limit)  │    │   (HTTP)        │
│    Keys, etc.)  │    └─────────────────┘    └─────────────────┘
└─────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.24.4+
- Docker and Docker Compose
- PostgreSQL (or use Docker container)
- Redis (optional, falls back to in-memory)
- NATS server (optional, defaults to localhost:4222)

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd ignis
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   ```bash
   cp env.example .env
   # Edit .env with your configuration
   ```

4. **Start the database**
   ```bash
   make docker-run
   ```

5. **Run the application**
   ```bash
   make run
   ```

## Configuration

### Environment Variables

Create a `.env` file in the root directory:

```env
# Server Configuration
PORT=8080
APP_ENV=development

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_DATABASE=ignis
DB_USERNAME=ignis_user
DB_PASSWORD=your_password
DB_SCHEMA=public

# Authentication
CLERK_SECRET_KEY=your_clerk_secret_key

# Message Queue (Optional)
NATS_URL=nats://localhost:4222

# Rate Limiting (Optional)
REDIS_URL=redis://localhost:6379
```

## API Documentation

### Authentication

Ignis supports two authentication methods:

1. **Clerk Authentication**: For user management and API key creation
2. **API Key Authentication**: For external API consumers

### Endpoints

#### Public Endpoints (API Key Required)

- `GET /api/v1/public/status` - Get API status
- `POST /api/v1/public/execute` - Submit code for execution
- `GET /api/v1/public/jobs/:job_id` - Get job status
- `GET /api/v1/public/jobs` - Get user's jobs

#### Protected Endpoints (Clerk Auth Required)

- `POST /api/v1/api-keys` - Create API key
- `GET /api/v1/api-keys` - List API keys
- `PATCH /api/v1/api-keys/:id` - Update API key
- `DELETE /api/v1/api-keys/:id` - Delete API key

- `POST /api/v1/webhooks` - Create webhook
- `GET /api/v1/webhooks` - List webhooks
- `PATCH /api/v1/webhooks/:id` - Update webhook
- `DELETE /api/v1/webhooks/:id` - Delete webhook

### Code Execution Example

```bash
curl -X POST http://localhost:8080/api/v1/public/execute \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "language": "python",
    "code": "print(\"Hello, World!\")"
  }'
```

Response:
```json
{
  "data": {
    "job_id": "01HN1234567890ABCDEF",
    "language": "python",
    "status": "received",
    "message": "Code submitted for execution"
  }
}
```

### Job Status Check

```bash
curl -X GET http://localhost:8080/api/v1/public/jobs/01HN1234567890ABCDEF \
  -H "X-API-Key: your-api-key"
```

## Development

### Available Commands

```bash
# Build the application
make build

# Run the application
make run

# Run with live reload (requires air)
make watch

# Run tests
go test ./...

# Start database with Docker
make docker-run

# Stop database
make docker-down

# Clean build artifacts
make clean
```

### Project Structure

```
ignis/
├── cmd/api/                 # Application entry point
├── internal/
│   ├── controllers/         # HTTP request handlers
│   ├── database/           # Database connection and configuration
│   ├── middleware/         # Authentication and rate limiting
│   ├── models/             # Data models and DTOs
│   ├── server/             # Server setup and routing
│   └── services/           # Business logic services
├── docker-compose.yml      # Docker services configuration
├── Dockerfile             # Application container
├── Makefile               # Build and development commands
└── README.md              # This file
```

### Database Schema

The application uses GORM for database operations with the following main entities:

- **Jobs**: Code execution requests and results
- **API Keys**: Authentication tokens for external access
- **Webhooks**: Notification endpoints for job events
- **Webhook Events**: Audit log of webhook deliveries

### Adding New Languages

1. Update the worker service to support the new language
2. Add the language to the supported languages list in `public_api.controller.go`
3. Update validation in the models

## Deployment

### Docker Deployment

1. **Build and run with Docker Compose**
   ```bash
   docker-compose up --build
   ```

2. **Production deployment**
   ```bash
   docker build -t ignis-api .
   docker run -p 8080:8080 --env-file .env ignis-api
   ```

### Environment Setup

For production deployment, ensure:

- PostgreSQL database is accessible
- Redis is configured for rate limiting (optional)
- NATS server is running for job queuing
- Clerk authentication is properly configured
- Environment variables are set securely

## Monitoring

### Health Checks

- `GET /health` - Database health check
- `GET /api/v1/public/health` - API health check

### Metrics

The application provides database connection metrics and health status information through the health endpoints.

## Security

- API key authentication with rate limiting
- Clerk-based user authentication
- CORS configuration for frontend integration
- Secure API key generation and storage
- Input validation and sanitization

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## Support

For support and questions:
- Create an issue in the repository
- Check the documentation for common solutions
- Review the API documentation for integration help