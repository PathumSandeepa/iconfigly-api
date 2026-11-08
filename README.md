# iconfigly-api

High-throughput, enterprise-grade distributed configuration control plane API engine written in Go 1.27.

---

## Overview

`iconfigly-api` serves as the authoritative backend core of the iConfigly platform. It manages multi-tenant boundaries, microservice topology, environment configurations, dynamic dependency resolution, granular role-based authorization, and tamper-evident cryptographic audit logs. Engineered following Domain-Driven Design (DDD) and Clean Architecture principles with zero external web framework dependencies.

---

## Key Features

- **Standard Library HTTP Engine**: Built on Go 1.27 standard library `net/http` router with sub-millisecond response latency and zero reflection-heavy external routers.
- **Clean Architecture & DDD**: Strict boundaries between domain entities, business logic use cases, database repositories, and HTTP transport layers.
- **Hierarchical RBAC Engine**: Granular permission resolution enforcing a deny-by-default security model with hierarchical ancestor inheritance across Organizations, Projects, and Environments.
- **Cryptographic Audit Ledger**: Immutable SHA-256 hash-chained event log ensuring tamper evidence with on-demand ledger integrity verification.
- **Dual Authentication**: Stateful HTTP-only session cookies with sliding expiration for web sessions, and cryptographically hashed Personal Access Tokens (`icfg_pat_...`) with constant-time verification for CLI and automated workloads.
- **RFC 7807 Problem Details**: Standardized structured error responses across all endpoints including machine-readable error codes and validation error matrices.
- **PostgreSQL Persistence**: Thread-safe connection pooling powered by `pgxpool` with transactional migration safety.

---

## Architecture

The backend codebase is organized according to Clean Architecture and Domain-Driven Design principles:

```
cmd/
└── server/
    └── main.go                        # Entrypoint, dependency injection, graceful shutdown

internal/
├── access/                            # RBAC evaluation engine & role bindings
├── audit/                             # Cryptographic SHA-256 hash-chained audit ledger
├── auth/                              # Authentication, sessions, PAT tokens, middleware
├── binding/                           # Configuration matrix bindings
├── config/                            # Environment-driven 12-factor configuration
├── database/                          # PostgreSQL connection pooling & migrations
├── environment/                       # Environment lifecycle tiers (DEV, QA, UAT, PROD)
├── health/                            # Liveness (/health) & Readiness (/ready) probes
├── organization/                      # Multi-tenant organization boundaries
├── project/                           # Project aggregates & workspace isolation
├── resource/                          # Cloud infrastructure resources & providers
├── server/                            # HTTP server lifecycle & routing pipeline
├── service/                           # Microservice catalog & dependency graphs
├── shared/                            # Enterprise shared primitives
│   ├── apperror/                      # RFC 7807 Problem Details implementation
│   ├── crypto/                        # Secure hashing, token generators, constant-time compare
│   ├── httputil/                      # JSON serializers, response writers, context extractors
│   └── valueobject/                   # Validated domain value objects
├── team/                              # Team aggregates & member rosters
└── user/                              # User identities, credentials, role assignments

db/
└── migrations/
    └── 000001_initial_schema.up.sql   # Complete PostgreSQL relational schema
```

---

## Prerequisites

- **Go**: Version `1.24+` (or Docker `golang:1.27.2-bookworm`)
- **PostgreSQL**: Version `15+` (or Docker `postgres:18-alpine`)

---

## All Backend Commands

### Development Server

Run the API server locally:

```bash
go run ./cmd/server
```

The API server will listen on [http://localhost:8080](http://localhost:8080).

### Build Production Binary

Compile an optimized standalone production binary:

```bash
# Standard compilation
go build -o bin/server ./cmd/server

# Optimized release build (stripped debug symbols)
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server
```

### Running Tests

Execute all unit and integration test suites:

```bash
go test ./...
```

Run all tests with verbose log output:

```bash
go test -v ./...
```

Run tests with data race detection enabled:

```bash
go test -race ./...
```

Run tests with code coverage summary:

```bash
go test -cover ./...
```

Generate an HTML code coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Run tests for an individual package:

```bash
go test -v ./internal/access
go test -v ./internal/auth
go test -v ./internal/audit
```

Run a single specific test by name:

```bash
go test -v -run TestAuthenticate ./internal/auth
```

### Static Analysis & Code Quality

Run the standard Go code analysis tool:

```bash
go vet ./...
```

Format code following standard Go conventions:

```bash
go fmt ./...
```

### Dependency Management

Download required dependencies and clean up unused modules:

```bash
go mod tidy
```

Verify dependency integrity against `go.sum`:

```bash
go mod verify
```

---

## Docker Commands

Operate the backend within Docker Compose:

```bash
# Start backend API
docker compose up iconfigly-api

# Start backend in detached mode with build
docker compose up -d --build iconfigly-api

# Follow container logs
docker logs -f iconfigly-api

# Execute test suite inside container
docker exec -it iconfigly-api go test ./...

# Execute race detection tests inside container
docker exec -it iconfigly-api go test -race ./...

# Access container shell
docker exec -it iconfigly-api /bin/bash
```

---

## Database Management & Inspection

Direct PostgreSQL administration commands using the `iconfigly-postgres` container:

```bash
# Open interactive PostgreSQL shell
docker exec -it iconfigly-postgres psql -U iconfigly -d iconfigly

# List all database tables
\dt

# View all registered users and roles
SELECT id, email, full_name, is_super_admin, status FROM users;

# View active user sessions
SELECT id, user_id, expires_at, created_at FROM sessions;

# Inspect cryptographic SHA-256 audit ledger chain
SELECT sequence_number, action, record_hash, previous_hash, created_at 
FROM audit_logs 
ORDER BY sequence_number ASC;

# View projects and their environments
SELECT p.name AS project, e.name AS environment, e.tier 
FROM projects p 
JOIN environments e ON e.project_id = p.id;

# Exit PostgreSQL shell
\q
```

---

## Environment Variables

| Variable | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | Number | `8080` | Port on which the HTTP server listens |
| `DATABASE_URL` | String | `postgres://iconfigly:iconfigly_secret@localhost:5432/iconfigly?sslmode=disable` | PostgreSQL connection string |
| `SESSION_SECRET` | String | (Required) | Cryptographic key used for session cookie authentication |
| `CORS_ALLOWED_ORIGINS` | String | `http://localhost:3000` | Comma-separated list of permitted CORS origins |
| `BCRYPT_COST` | Number | `12` | Cost factor for password hashing (minimum: 10, default: 12) |

---

## API Endpoints Reference

### Health Probes

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Liveness probe returning process status | No |
| `GET` | `/ready` | Readiness probe verifying PostgreSQL pool connectivity | No |

### Authentication & Setup

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/setup/status` | Check if cluster setup has already been initialized | No |
| `POST` | `/api/v1/auth/setup` | Initialize cluster with primary organization and Super Admin | No |
| `POST` | `/api/v1/auth/login` | Authenticate user and issue encrypted session cookie | No |
| `POST` | `/api/v1/auth/logout` | Invalidate current session and clear session cookie | Yes |
| `GET` | `/api/v1/auth/me` | Fetch authenticated user profile and permissions | Yes |

### Organizations & Projects

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/organizations` | List organizations accessible to current user | Yes |
| `GET` | `/api/v1/projects` | List projects within current organization | Yes |
| `POST` | `/api/v1/projects` | Create a new project workspace | Yes |
| `GET` | `/api/v1/projects/{id}` | Retrieve project details and statistics | Yes |

### Environments & Topology

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/projects/{id}/environments` | List environments (`DEV`, `QA`, `UAT`, `PROD`) | Yes |
| `POST` | `/api/v1/projects/{id}/environments` | Create an environment tier | Yes |
| `GET` | `/api/v1/projects/{id}/services` | List microservices and dependencies | Yes |
| `POST` | `/api/v1/projects/{id}/services` | Register a new microservice | Yes |
| `GET` | `/api/v1/projects/{id}/resources` | List provisioned infrastructure resources | Yes |
| `POST` | `/api/v1/projects/{id}/resources` | Provision a cloud infrastructure resource | Yes |

### Audit Ledger

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/audit/events` | Query immutable audit log with pagination and filters | Yes |
| `GET` | `/api/v1/audit/verify` | Verify cryptographic SHA-256 chain integrity | Yes |

### Identity & Access Control

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/users` | List organization users | Yes (Admin) |
| `POST` | `/api/v1/users` | Provision a new user account | Yes (Admin) |
| `GET` | `/api/v1/teams` | List teams and membership counts | Yes |
| `POST` | `/api/v1/teams` | Create a new collaborative team | Yes |
| `GET` | `/api/v1/tokens` | List active Personal Access Tokens (PAT) | Yes |
| `POST` | `/api/v1/tokens` | Generate a new Personal Access Token | Yes |
| `DELETE` | `/api/v1/tokens/{id}` | Revoke a Personal Access Token | Yes |

---

## Error Handling (RFC 7807)

All non-2xx responses conform to RFC 7807 Problem Details standard:

```json
{
  "type": "https://iconfigly.dev/errors/unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Invalid or expired session.",
  "instance": "/api/v1/auth/me"
}
```
