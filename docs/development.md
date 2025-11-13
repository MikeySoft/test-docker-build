# Development Guide

> Looking for production deployment instructions? See [`setup.md`](setup.md).

## Prerequisites

- Go 1.24+
- Node.js 20+
- Docker 24+
- PostgreSQL 15+
- mkcert (for TLS certificates in development)

## Quick Start

1. **Clone and setup:**
   ```bash
   git clone https://github.com/mikeysoft/flotilla.git
   cd flotilla
   make dev-setup
   ```

2. **Generate TLS certificates (for secure development):**
   ```bash
   make generate-certs
   ```

3. **Start development environment:**
   ```bash
   # With TLS (recommended for auth testing)
   TLS_ENABLED=true TLS_CERT_FILE=./tls/cert.pem TLS_KEY_FILE=./tls/key.pem make run-server

   # Or without TLS (not recommended due to Secure cookie requirements)
   make run-server
   ```

4. **Access the application:**
   - With TLS: https://localhost:8080
   - Without TLS: http://localhost:8080

## TLS Development Setup

For proper authentication testing, the application requires HTTPS in development because:
- Refresh tokens are stored in `Secure` HttpOnly cookies
- Browsers only send `Secure` cookies over HTTPS connections
- This matches production security requirements

### Certificate Generation

The Makefile includes a helper to generate trusted local certificates:

```bash
make generate-certs
```

This command:
1. Checks if `mkcert` is installed (install with `brew install mkcert nss`)
2. Installs the mkcert CA in your system trust store
3. Creates a `tls/` directory
4. Generates certificates for `localhost`, `127.0.0.1`, and `::1`

### Running with TLS

```bash
# Set environment variables for TLS
export TLS_ENABLED=true
export TLS_CERT_FILE=./tls/cert.pem
export TLS_KEY_FILE=./tls/key.pem

# Run the server
make run-server
```

Or run directly:
```bash
TLS_ENABLED=true TLS_CERT_FILE=./tls/cert.pem TLS_KEY_FILE=./tls/key.pem ./bin/server
```

## Environment Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `PROD` | Environment mode (`DEV` or `PROD`) |
| `SERVER_HOST` | `localhost` | Server host address |
| `SERVER_PORT` | `8080` | Server port |
| `TLS_ENABLED` | `false` | Enable TLS/HTTPS |
| `TLS_CERT_FILE` | `` | Path to TLS certificate file |
| `TLS_KEY_FILE` | `` | Path to TLS private key file |
| `DATABASE_URL` | `postgres://...` | PostgreSQL connection string |
| `JWT_SECRET` | `your-super-secret...` | Legacy HMAC JWT secret (unused when RSA keys are provided) |
| `JWT_PRIVATE_KEY` | `` | RSA private key (PEM) used to sign access tokens; takes precedence over file path |
| `JWT_PRIVATE_KEY_FILE` | `jwt_private.pem` | Path to RSA private key file; falls back to in-memory key if not found |

### InfluxDB (Optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `INFLUXDB_ENABLED` | `false` | Enable InfluxDB metrics |
| `INFLUXDB_URL` | `http://localhost:8086` | InfluxDB server URL |
| `INFLUXDB_TOKEN` | `` | InfluxDB authentication token |
| `INFLUXDB_ORG` | `flotilla` | InfluxDB organization |
| `INFLUXDB_BUCKET` | `metrics` | InfluxDB bucket name |

## Development Workflow

### Backend Development

1. **Start PostgreSQL:**
   ```bash
   make run-dev  # Starts PostgreSQL + server
   ```

2. **Make changes to Go code**

3. **Rebuild and restart:**
   ```bash
   make build-server
   # Kill existing server and restart
   ```

### Frontend Development

1. **Start development server:**
   ```bash
   cd web
   npm run dev
   ```

2. **Access at:** http://localhost:5173 (Vite dev server)

3. **Build for production:**
   ```bash
   make build-frontend
   ```

### Database Migrations

Migrations run automatically on server startup. The SQL files live in `deployments/migrations/` and are applied in order. For manual execution use your Postgres tooling of choice.

### Security Tooling

#### gosec (Go security static analysis)

1. **Install**
   ```bash
   # macOS (Homebrew)
   brew install gosec

   # or with Go tooling
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   ```
2. **Run locally**
   ```bash
   cd flotilla
   gosec ./...
   ```
3. **CI/CD**
   - Add `gosec ./...` to your `Makefile` or pipeline after `go test`.
   - Fail the pipeline on non-zero exit codes to prevent vulnerable code from merging.

## Authentication in Development

### First User Setup

1. **Start the server** (with or without TLS)
2. **Create first admin user:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/setup \
     -H "Content-Type: application/json" \
     -d '{"username": "admin", "password": "admin123"}'
   ```

3. **Login via UI:**
   - Navigate to https://localhost:8080 (or http://localhost:8080)
   - Use the credentials you created

### API Testing

```bash
# Login
curl -X POST https://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}' \
  -c cookies.txt

# Use access token for API calls
curl -H "Authorization: Bearer <access_token>" \
  https://localhost:8080/api/v1/hosts
```

## Troubleshooting

### Certificate Issues

- **"Certificate not trusted"**: Run `mkcert -install` to install the CA
- **"Connection refused"**: Ensure server is running with TLS enabled
- **"Invalid certificate"**: Regenerate certificates with `make generate-certs`

### Authentication Issues

- **"Refresh token not sent"**: Ensure you're using HTTPS (TLS enabled)
- **"Invalid credentials"**: Check if user exists in database
- **"CSRF token missing"**: Ensure you're including the `X-CSRF-Token` header

### Database Issues

- **"Connection failed"**: Ensure PostgreSQL is running (`make run-dev`)
- **"Migration failed"**: Check database permissions and connection string

## Production Considerations

Refer to [`setup.md`](setup.md#hardening-checklist) for deployment hardening, TLS management, and operational best practices.
