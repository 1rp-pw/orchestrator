# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Policy Orchestrator service - a Go-based microservice that manages policy lifecycle (create, update, version, publish) and executes policies through an external engine service. It acts as a middleware layer between clients and a policy engine.
And a flow system, that can run multiple policies with the same data depending on if the policy returns true/false and can give a true/false response, or a custom output depending on if the flow requires it.

## Development Commands

### Build and Run
```bash
# Build the service
go build -o orchestrator ./cmd/api

# Run the service
go run ./cmd/api/service.go

# Run with environment variables
ENGINE_ADDRESS=localhost:9009 go run ./cmd/api/service.go
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests in a specific package
go test ./internal/structs
go test ./internal/flow
go test ./internal/policy
```

### Code Quality
```bash
# Format code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run

# Check for vulnerabilities
go mod tidy
go mod verify
```

## Architecture

The service follows a clean layered architecture:

1. **API Layer** (`cmd/api/service.go`) - Service configuration and startup
2. **HTTP Routing** (`internal/service.go`) - Route definitions and middleware setup
3. **Business Logic**:
   - `internal/engine/` - Policy engine integration for execution
   - `internal/policy/` - PostgreSQL-based policy storage with versioning
   - `internal/flow/` - PostgreSQL-based flow storage and versioning
   - `internal/structs/policy/` - Core policy models and types

### Key Design Patterns

- **Policy Versioning**: Each policy has a base ID with multiple versions. Drafts exist separately and can be published as new versions.
- **Flow Versioning**: Each flow has a base ID with multiple versions. Draft exist seperate and can be published as new versions.
- **Database Functions**: Complex operations use PostgreSQL functions (`create_policy`, `create_flow`, `update_draft_flow`, `update_draft`,`publish_draft_flow_as_version`, `publish_draft_as_version`,`create_draft_flow_from_version`, `create_draft_from_version`)
- **Engine Integration**: Policies are executed by forwarding to an external engine service at `ENGINE_ADDRESS`

## Environment Configuration

Required environment variables:
```bash
# Service
PORT=3000                        # HTTP port
ENGINE_ADDRESS=localhost:9009    # Policy engine service URL

# Feature Flags
FLAGS_PROJECT_ID=structs
FLAGS_AGENT_ID=orchestrator
FLAGS_ENVIRONMENT_ID=orchestrator

# Database (handled by keloran/go-config)
# PostgreSQL connection details required
```

## API Endpoints

### Policy Execution
- `POST /run` - Execute ad-hoc policy
- `POST /run/{policyId}` - Execute stored policy

### Policy Management
- `POST /policy` - Create new policy (draft)
- `GET /policy/{policyId}` - Get policy details
- `PUT /policy/{policyId}` - Update draft or publish version of flow
- `GET /policies` - List all policies
- `GET /policy/{policyId}/versions` - List policy versions
- `GET /policy/{policyId}/draft` - Create draft from version of policy

### Flow Management
- `POST /flow` - Create a new flow (draft)
- `GET /flow/{flowId}` - Get flow details
- `PUT /flow/{flowId]` - Update draft or publish version of flow
- `GET /flows` - List all flows
- `GET /flow/{flowId]/versions` - List flow versions
- `GET /flow/{flowId]/draft` - Create draft from version of flow
- `POST /flow/test` - Test a flow

## Database Schema

The service uses PostgreSQL with:
- Policy, flow tables for storing rules, versions, and metadata
- `policy_summary` view for efficient policy listing
- `flow_summary` view for efficient flow listing
- Stored procedures for atomic operations
- Version management through `base_policy_id` and `status` fields of policies
- Version management through `base_flow_id` and `status` fields for flows

## Dependencies

Key libraries:
- `bugfixes/go-bugfixes` - Logging and HTTP middleware
- `keloran/go-config` - Database and configuration management
- `jackc/pgx/v5` - PostgreSQL driver with connection pooling
- `gopkg.in/yaml.v3` - Yaml parsing