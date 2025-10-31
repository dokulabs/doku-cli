# Doku CLI - Development Progress

## Project Overview

Doku is a CLI tool that simplifies running and managing Docker-based services locally with HTTPS, subdomain routing, and automatic service discovery.

**Key Technologies:**
- Go 1.21+ with Cobra CLI framework
- Docker SDK v25
- Traefik v2.10 (reverse proxy)
- mkcert (local SSL certificates)
- TOML for configuration
- Docker bridge networking (doku-network: 172.20.0.0/16)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client/Browser                       │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTPS
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                    Traefik (Reverse Proxy)                   │
│                     https://*.doku.local                     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                      doku-network                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Service 1  │  │   Service 2  │  │   Service 3  │     │
│  │   (Public)   │  │  (Internal)  │  │  (Internal)  │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## Completed Phases

### Phase 1: Core Infrastructure ✅
**Status:** Complete
**Date:** Initial implementation
**Files Created:**
- `internal/config/config.go` - Configuration management with TOML
- `internal/docker/client.go` - Docker SDK wrapper
- `internal/docker/network.go` - Network management
- `internal/docker/volume.go` - Volume operations
- `pkg/types/types.go` - Core type definitions

**Key Features:**
- Configuration stored in `~/.doku/config.toml`
- Docker client with error handling
- Network creation and management (doku-network)
- Volume lifecycle management

### Phase 2: Setup & Initialization ✅
**Status:** Complete
**Date:** Initial implementation
**Files Created:**
- `internal/certs/manager.go` - SSL certificate generation with mkcert
- `internal/dns/manager.go` - DNS configuration (dnsmasq/hosts file)
- `internal/traefik/manager.go` - Traefik setup and management
- `cmd/init.go` - Interactive initialization command

**Key Features:**
- Self-signed CA certificate generation
- Local domain resolution (.doku.local)
- Traefik container setup with dashboard
- Interactive domain/protocol selection
- Automatic prerequisite checking (Docker, mkcert)

### Phase 3: Catalog System ✅
**Status:** Complete
**Date:** Initial implementation
**Files Created:**
- `internal/catalog/manager.go` - Catalog parsing and management
- `internal/catalog/parser.go` - TOML catalog parser
- `cmd/catalog.go` - Catalog CLI commands (search, list, update, info)

**Companion Repository:**
- `doku-catalog/` - Service definitions repository
- 8 services available: PostgreSQL, MySQL, Redis, MongoDB, RabbitMQ, Elasticsearch, MinIO, Nginx

**Key Features:**
- Git-based catalog with versioning
- Multi-version service support
- Service search and discovery
- Rich service metadata (description, tags, resources)

### Phase 4: Service Management ✅
**Status:** Complete
**Date:** Latest implementation (Oct 30, 2024)
**Files Created:**
- `internal/service/installer.go` (353 lines) - Service installation logic
- `internal/service/manager.go` (334 lines) - Lifecycle management
- `cmd/install.go` (316 lines) - Interactive installation command

**Key Features:**

#### Installation (internal/service/installer.go)
- Pull Docker images from catalog specs
- Create containers with custom configuration
- Generate Traefik labels for routing
- Apply CPU/memory resource limits
- Create named volumes for persistence
- Connect to doku-network
- Support for internal-only services (--internal flag)

**Important Methods:**
- `Install(opts InstallOptions)` - Main installation flow
- `generateLabels()` - Traefik routing label generation
- `generateInstanceName()` - Unique instance naming
- `mergeEnvironment()` - Config override handling
- `applyResourceLimits()` - CPU/memory constraints

#### Lifecycle Management (internal/service/manager.go)
- Start/Stop/Restart operations
- Remove with cleanup (volumes, network)
- Log retrieval (streaming support)
- Status tracking (running/stopped/failed)
- Resource statistics (CPU/memory usage)

**Important Methods:**
- `Start(instanceName)` - Start stopped service
- `Stop(instanceName)` - Graceful shutdown
- `Remove(instanceName, force)` - Full cleanup
- `GetLogs(instanceName, follow)` - Log streaming
- `GetStatus(instanceName)` - Container state
- `GetStats(instanceName)` - Resource metrics

#### Interactive Installation (cmd/install.go)
- Parse service:version format
- Interactive configuration prompts (survey/v2)
- Environment variable overrides
- Resource limit specification
- Volume mount configuration
- Confirmation before installation

**Flag Options:**
- `--name, -n` - Custom instance name
- `--env, -e` - Environment variables (KEY=VALUE)
- `--memory` - Memory limit (512m, 1g)
- `--cpu` - CPU limit (0.5, 1.0)
- `--volume` - Volume mounts (host:container)
- `--yes, -y` - Skip prompts
- `--internal` - Install as internal service (no Traefik exposure)

## Recent Feature: API Gateway Pattern Support

**Problem:** User needed to implement enterprise microservices architecture with API Gateway pattern, similar to their production setup with Spring Cloud Gateway.

**Solution:** Implemented `--internal` flag for services that should NOT be exposed externally via Traefik.

### Implementation Details

**Files Modified:**
1. `internal/service/installer.go:262` - Modified `generateLabels()` function
   - Added `internal bool` parameter
   - Conditional Traefik label generation
   - When `internal=true`: sets `traefik.enable="false"`
   - When `internal=false`: generates full routing labels

2. `cmd/install.go:24` - Added `installInternal bool` flag
   - Wire-through to `InstallOptions.Internal`
   - Updated command examples

3. `pkg/types/types.go` - Added `Internal bool` to `InstallOptions`

### Usage Pattern

**API Gateway Architecture:**
```bash
# 1. Install backend services as internal (NOT exposed)
doku install user-service --internal \
  --env DATABASE_URL=postgresql://postgres:5432/users

doku install order-service --internal \
  --env DATABASE_URL=postgresql://postgres:5432/orders \
  --env USER_SERVICE_URL=http://user-service:8081

doku install payment-service --internal \
  --env DATABASE_URL=postgresql://postgres:5432/payments

# 2. Install API Gateway as public service (exposed via Traefik)
doku install spring-gateway --name api \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082 \
  --env PAYMENT_SERVICE_URL=http://payment-service:8083 \
  --env JWT_SECRET=local-dev-secret

# Result:
# - UI calls: https://api.doku.local (public)
# - API Gateway validates tokens/auth
# - Gateway routes to internal services by container name
# - Backend services NOT accessible externally
```

**Documentation:** See `doku-catalog/gateway-pattern.md` for comprehensive guide.

## Project Structure

```
doku-cli/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command setup
│   ├── init.go            # Initialization command
│   ├── install.go         # Service installation
│   ├── catalog.go         # Catalog management
│   ├── list.go            # List services
│   └── version.go         # Version info
├── internal/
│   ├── catalog/           # Catalog system
│   │   ├── manager.go     # Catalog operations
│   │   └── parser.go      # TOML parsing
│   ├── certs/             # SSL certificate management
│   │   └── manager.go     # mkcert integration
│   ├── config/            # Configuration
│   │   └── config.go      # Config CRUD operations
│   ├── dns/               # DNS management
│   │   └── manager.go     # DNS configuration
│   ├── docker/            # Docker operations
│   │   ├── client.go      # Docker SDK wrapper
│   │   ├── network.go     # Network management
│   │   └── volume.go      # Volume operations
│   ├── service/           # Service management
│   │   ├── installer.go   # Installation logic
│   │   └── manager.go     # Lifecycle operations
│   └── traefik/           # Traefik management
│       └── manager.go     # Traefik setup
├── pkg/
│   └── types/             # Shared types
│       └── types.go       # Type definitions
├── main.go                # Entry point
└── go.mod                 # Dependencies

doku-catalog/              # Separate repository
├── catalog.toml           # Service definitions
├── gateway-pattern.md     # API Gateway guide
└── README.md              # Catalog documentation
```

## Configuration Files

### ~/.doku/config.toml
```toml
[preferences]
protocol = "https"
domain = "doku.local"
catalog_version = "main"
last_update = 2024-10-30T...
dns_setup = "hosts"

[network]
name = "doku-network"
subnet = "172.20.0.0/16"
gateway = "172.20.0.1"

[traefik]
container_name = "doku-traefik"
status = "running"
dashboard_enabled = true
http_port = 80
https_port = 443
dashboard_url = "https://traefik.doku.local"

[certificates]
ca_cert = "~/.doku/certs/rootCA.pem"
ca_key = "~/.doku/certs/rootCA-key.pem"
certs_dir = "~/.doku/certs"

[instances.postgres-16]
name = "postgres-16"
service_type = "postgres"
version = "16"
status = "running"
container_name = "doku-postgres-16"
url = "https://postgres-16.doku.local"
# ... more fields
```

## Build Errors Encountered & Fixed

### Error 1: Missing config.Manager methods
**Error:** `HasInstance undefined`, `UpdateInstance undefined`
**Fix:** Added methods to `internal/config/config.go`:
```go
func (m *Manager) HasInstance(name string) bool
func (m *Manager) UpdateInstance(name string, instance *types.Instance) error
```

### Error 2: DisconnectContainer signature mismatch
**Error:** `not enough arguments` - missing force parameter
**Fix:** Updated all calls to include `force bool` parameter

### Error 3: ContainerLogs signature mismatch
**Error:** Wrong parameters - `(string, int, bool)` vs `(string, bool)`
**Fix:** Rewrote GetLogs() to handle `io.ReadCloser` properly

### Error 4: Variable shadowing
**Error:** Loop variable `i` shadowing receiver `*Installer.i`
**Fix:** Changed loop variable to `num` in `generateInstanceName()`

### Error 5: GetStats return type
**Error:** Returned `map[string]interface{}` instead of `ContainerStats`
**Fix:** Changed return type to `dockerTypes.ContainerStats`

### Error 6: Flag shorthand conflict
**Error:** `--verbose/-v` and `--volume/-v` conflict
**Fix:** Removed shorthand from volume flag, used `StringSliceVar` instead of `StringSliceVarP`

## Known Issues & Limitations

1. **Volume flag**: No shorthand available (conflict with --verbose/-v)
2. **Service discovery**: Currently manual via container names
3. **Health checks**: Not yet implemented
4. **Multi-project isolation**: Not yet supported
5. **Service dependencies**: Manual ordering required

## Next Steps (Not Yet Implemented)

### Phase 5: Service Lifecycle Commands
- [ ] `doku list` - List all running services
- [ ] `doku stop <service>` - Stop a service
- [ ] `doku start <service>` - Start a service
- [ ] `doku restart <service>` - Restart a service
- [ ] `doku remove <service>` - Remove a service
- [ ] `doku logs <service>` - View service logs
- [ ] `doku status <service>` - Show service status
- [ ] `doku stats <service>` - Show resource usage

### Phase 6: Advanced Features
- [ ] Service health checks
- [ ] Dependency management
- [ ] Project support (multi-service grouping)
- [ ] Backup/restore functionality
- [ ] Service templates
- [ ] Environment profiles (dev/staging)

### Phase 7: Catalog Expansion
- [ ] Add Spring Cloud Gateway service
- [ ] Add Kong API Gateway service
- [ ] Add more databases (Cassandra, CouchDB)
- [ ] Add message brokers (Kafka, NATS)
- [ ] Add monitoring (Prometheus, Grafana)
- [ ] Add observability (Jaeger, Zipkin)

## Testing Checklist

### Manual Testing Required
- [ ] `doku init` - Full initialization flow
- [ ] `doku catalog update` - Catalog download
- [ ] `doku install postgres` - Basic installation
- [ ] `doku install postgres:16 --name db1` - Versioned install
- [ ] `doku install redis --internal` - Internal service
- [ ] API Gateway pattern with 3+ services
- [ ] Service-to-service communication
- [ ] Resource limits enforcement
- [ ] Volume persistence
- [ ] Traefik routing correctness

### Build Verification
```bash
# Clean build
go clean
go mod tidy
go build -o ./bin/doku .

# Version check
./bin/doku version

# Command availability
./bin/doku --help
./bin/doku init --help
./bin/doku install --help
./bin/doku catalog --help
```

## Dependencies

```go
require (
    github.com/BurntSushi/toml v1.3.2
    github.com/docker/docker v24.0.7+incompatible
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2
    github.com/AlecAivazis/survey/v2 v2.3.7
    github.com/fatih/color v1.16.0
    // ... more dependencies
)
```

## Important Constants & Defaults

```go
// Configuration
DefaultDomain   = "doku.local"
DefaultProtocol = "https"
ConfigFileName  = "config.toml"
DokuDirName     = ".doku"

// Network
NetworkName    = "doku-network"
NetworkSubnet  = "172.20.0.0/16"
NetworkGateway = "172.20.0.1"

// Traefik
TraefikContainerName = "doku-traefik"
TraefikImage        = "traefik:v2.10"
TraefikHTTPPort     = 80
TraefikHTTPSPort    = 443
TraefikDashboard    = 8080
```

## Git Workflow

**Main Repository:** `dokulabs/doku-cli`
**Catalog Repository:** `dokulabs/doku-catalog`

Both repos are local and not yet pushed to GitHub.

## User Context

**User's Scenario:**
- Current organization uses Spring API Gateway for microservices
- UI → API Gateway → Multiple backend services
- API Gateway handles JWT validation and authorization
- Wanted to replicate this pattern locally with Doku
- Solution: Internal services + public API Gateway pattern

**User's Workflow:**
- Working on this project intermittently
- Needs context preservation across sessions
- Wants to avoid re-explaining architecture each time

## Quick Start for New Sessions

```bash
# 1. Navigate to project
cd /Users/kesharinandan/Work/Experiment/dokulabs/doku-cli

# 2. Verify build
go build -o ./bin/doku .

# 3. Check current state
./bin/doku version
./bin/doku --help

# 4. View available services
cd ../doku-catalog
cat catalog.toml

# 5. Return to CLI
cd ../doku-cli
```

## Key Files Reference

**Most Important Files to Review:**
1. `internal/service/installer.go` - Service installation (353 lines)
2. `internal/service/manager.go` - Lifecycle management (334 lines)
3. `cmd/install.go` - Install command (316 lines)
4. `internal/config/config.go` - Config management (392 lines)
5. `pkg/types/types.go` - Type definitions

**Documentation Files:**
1. `doku-catalog/gateway-pattern.md` - API Gateway implementation guide
2. This file (`DEVELOPMENT.md`) - Current state reference

---

**Last Updated:** October 31, 2024
**Current Status:** Phase 4 Complete - Service Management with API Gateway Support
**Next Task:** Implement remaining lifecycle commands (list, stop, start, etc.)
