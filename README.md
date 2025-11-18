# Doku CLI

[![Build and Test](https://github.com/dokulabs/doku-cli/actions/workflows/build.yml/badge.svg)](https://github.com/dokulabs/doku-cli/actions/workflows/build.yml)
[![Release](https://github.com/dokulabs/doku-cli/actions/workflows/release.yml/badge.svg)](https://github.com/dokulabs/doku-cli/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dokulabs/doku-cli)](https://goreportcard.com/report/github.com/dokulabs/doku-cli)
[![License](https://img.shields.io/github/license/dokulabs/doku-cli)](LICENSE)

> Local development environment manager with Docker, Traefik, and SSL

Doku is a CLI tool that simplifies running and managing Docker-based services locally with automatic service discovery, DNS routing, and SSL certificates.

## Features

- 🚀 **One-command setup** - Get services running in seconds
- 🔒 **HTTPS by default** - Local SSL certificates that just work
- 🌐 **Clean URLs** - Access services via `service.doku.local` instead of `localhost:port`
- 🔗 **Service discovery** - Automatic connection string generation
- 📦 **Version management** - Run multiple versions of the same service
- 🎯 **Local development focus** - Optimized for developer productivity
- 💪 **Resource control** - Set CPU and memory limits per service
- 🏗️ **API Gateway pattern** - Internal-only services for microservices architecture
- 🔐 **Environment management** - Secure environment variable handling with masking
- 📋 **Service catalog** - Curated collection of 24+ popular development services
- 🔄 **Full lifecycle management** - Start, stop, restart, and remove services with ease
- 🧩 **Multi-container services** - Deploy complex services with multiple containers
- 🔗 **Dependency management** - Automatic installation of service dependencies
- 🔌 **Port mapping** - Map container ports to host for direct access via localhost
- 🐳 **Custom projects** - Build and run from your own Dockerfiles with `--path` flag
- ⚡ **Dynamic configuration** - Update environment variables without rebuilding containers

## Quick Start

### Installation

**Quick Install (Recommended):**

```bash
# Using curl
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash

# Or using wget
wget -qO- https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

**Using Go:**

```bash
# Install latest release
go install github.com/dokulabs/doku-cli/cmd/doku@latest

# Install from main branch
go install github.com/dokulabs/doku-cli/cmd/doku@main

# Install specific version
go install github.com/dokulabs/doku-cli/cmd/doku@v0.2.0
```

**More Options:**

See [INSTALL.md](INSTALL.md) for detailed installation instructions including:
- Installing specific versions
- Building from source
- Custom installation directories
- Platform-specific instructions

**Verify installation:**

```bash
doku version
```

### First-Time Setup

```bash
# Initialize Doku on your system
doku init
```

This will:
- Check Docker availability
- Install SSL certificates (mkcert)
- Configure DNS for `*.doku.local`
- Set up Traefik reverse proxy
- Download service catalog

### Install Your First Service

```bash
# Install PostgreSQL
doku install postgres

# Install with specific version
doku install postgres:14 --name postgres-14

# Install with custom environment variables
doku install postgres \
  --env POSTGRES_PASSWORD=mysecret \
  --env POSTGRES_DB=myapp

# Install with resource limits
doku install redis --memory 512m --cpu 1.0

# Install with port mapping for direct access
doku install postgres --port 5432

# Install with multiple port mappings (e.g., RabbitMQ)
doku install rabbitmq --port 5672 --port 15672

# Install as internal service (no external access)
doku install redis --internal
```

### Install Custom Projects

Run your own applications from Dockerfiles:

```bash
# Install a custom frontend application
doku install frontend --path=./my-frontend-app

# Install backend as internal service
doku install api --path=./backend --internal --port 4000

# Install with environment variables
doku install myapp --path=./myapp \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/mydb \
  --env API_KEY=secret123 \
  --port 8080
```

**📖 See the complete guide:** [Custom Projects Guide](CUSTOM_PROJECTS_GUIDE.md)

### Manage Environment Variables

Update configuration dynamically without rebuilding:

```bash
# View environment variables
doku env frontend

# Set new variables
doku env set frontend API_KEY=newsecret DEBUG=true

# Auto-restart after changes
doku env set frontend API_KEY=newsecret --restart

# Remove variables
doku env unset frontend OLD_KEY
```

### Manage Services

```bash
# List running services
doku list

# List all services (including stopped)
doku list --all

# Start a service
doku start postgres

# Stop a service
doku stop postgres

# Restart a service
doku restart postgres

# View logs
doku logs postgres -f

# Get detailed service info
doku info postgres

# View environment variables
doku env postgres

# Remove a service
doku remove postgres
```

### Upgrade Doku CLI

Keep your doku CLI up to date with the latest features and fixes:

```bash
# Check current version
doku version

# Upgrade to the latest version
doku self upgrade

# Upgrade without confirmation prompt
doku self upgrade --force
```

The upgrade command will:
- Check for the latest version on GitHub
- Download the appropriate binary for your platform
- Replace the current binary with the new version
- Verify the installation

## Architecture

```
┌─────────────────────────────────────────┐
│           User (Browser/CLI)            │
│    https://service.doku.local          │
└────────────────┬────────────────────────┘
                 │
                 ▼
      ┌──────────────────────┐
      │   Traefik Proxy      │
      │   (Port 80/443)      │
      └──────────┬───────────┘
                 │
   ┌─────────────┴─────────────┐
   │  doku-network (bridge)    │
   │                            │
   │  ┌────────┐  ┌────────┐  │
   │  │postgres│  │ redis  │  │
   │  └────────┘  └────────┘  │
   └────────────────────────────┘
```

## Available Services

**📋 See the complete and up-to-date service catalog:**
→ **[Doku Service Catalog](https://github.com/dokulabs/doku-catalog)**

The catalog includes 24+ services across multiple categories:
- **Databases**: PostgreSQL (with pgvector), MySQL, MongoDB, MariaDB, ClickHouse, Redis, Memcached
- **Message Queues**: RabbitMQ, Apache Kafka
- **Search & Analytics**: Elasticsearch
- **Monitoring**: Dozzle, Prometheus, Grafana, Jaeger, SigNoz, Sentry
- **Web Servers**: Nginx
- **Development Tools**: MailHog, Adminer, phpMyAdmin, LocalStack
- **Storage**: MinIO
- **Security**: HashiCorp Vault
- **Coordination**: Zookeeper

**Browse services locally:**

```bash
# List all services in a compact table
doku catalog

# List services with detailed information
doku catalog --verbose

# Filter by category
doku catalog --category database

# Search for services
doku catalog search postgres

# Show service details
doku catalog show postgres --verbose
```

**Quick examples:**

```bash
# Install PostgreSQL with pgvector extension
doku install postgres:17-pgvector

# Install Redis
doku install redis

# Install multiple services
doku install postgres redis rabbitmq
```

## Usage Examples

### Multiple Versions

Run multiple versions of the same service simultaneously:

```bash
# Install PostgreSQL 14
doku install postgres:14 --name postgres-14

# Install PostgreSQL 16
doku install postgres:16 --name postgres-16

# Both are now running on the same network
# Access via:
# - https://postgres-14.doku.local
# - https://postgres-16.doku.local
```

### Service Discovery

Automatic connection string generation:

```bash
$ doku info postgres-14

Connection:
  postgresql://postgres@postgres-14.doku.local:5432

# Use this in your application
DATABASE_URL=postgresql://postgres@postgres-14.doku.local:5432
```

### Environment Variables

View and export environment variables configured for services:

```bash
# View environment variables (sensitive values masked by default)
$ doku env postgres

Environment Variables for postgres
==================================================

🔒 Sensitive values are masked. Use --show-values to display actual values.

  POSTGRES_DB = myapp
  POSTGRES_PASSWORD 🔐 = po***rd
  POSTGRES_USER = postgres

Tip: Use 'doku env postgres --show-values' to see actual values
Tip: Use 'doku env postgres --export' for shell export format

# Show actual values (unmask sensitive data)
doku env postgres --show-values

# Export format for shell sourcing
doku env postgres --export > postgres.env

# Export with actual values
doku env postgres --export --show-values > postgres.env

# Source directly into your shell
eval $(doku env postgres --export --show-values)
```

### Port Mapping

Map container ports to your host machine for direct access via `localhost`:

```bash
# Install PostgreSQL with port mapping
doku install postgres --port 5432

# Connect directly via localhost
psql -h localhost -p 5432 -U postgres

# Install with custom host port (avoid conflicts)
doku install postgres:16 --name postgres-16 --port 5433:5432

# Connect to custom port
psql -h localhost -p 5433 -U postgres
```

#### Multiple Port Mappings

Services like RabbitMQ require multiple ports (AMQP + Management UI):

```bash
# Map both AMQP and Management UI ports
doku install rabbitmq --port 5672 --port 15672

# Access RabbitMQ
# - AMQP: localhost:5672
# - Management UI: http://localhost:15672

# Map to different host ports
doku install rabbitmq \
  --port 5673:5672 \
  --port 15673:15672

# Now accessible at:
# - AMQP: localhost:5673
# - Management UI: http://localhost:15673
```

#### Viewing Port Mappings

```bash
# List services with port mappings
$ doku list

● postgres  [running]
  Service: postgres (v16)
  Port: localhost:5432 → container:5432

● rabbitmq  [running]
  Service: rabbitmq
  Port: localhost:5672 → container:5672
  Port: localhost:15672 → container:15672

# Detailed port information
$ doku info rabbitmq

Network
  Port Mappings:
    localhost:5672 → container:5672
    localhost:15672 → container:15672
```

### API Gateway Pattern

Build microservices architectures with internal-only services:

```bash
# Install backend services as internal (not exposed externally)
doku install user-service --internal
doku install order-service --internal
doku install payment-service --internal

# Install API Gateway as public service
doku install spring-gateway --name api \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082 \
  --env PAYMENT_SERVICE_URL=http://payment-service:8083

# Now only the API Gateway is accessible externally:
# https://api.doku.local

# Backend services communicate internally via container names
```

This pattern mirrors enterprise microservices architectures where:
- API Gateway handles authentication, authorization, and routing
- Backend services are isolated and only accessible within the network
- Services communicate using container names (service discovery)

### Multi-Container Services

Deploy complex services that require multiple containers:

```bash
# Install SigNoz (3 containers: otel-collector, query-service, frontend)
# Automatically installs dependencies: Zookeeper and ClickHouse
doku install signoz

# List all containers
doku list
# Shows:
#   ● zookeeper [running]
#   ● clickhouse [running]
#   ● signoz [running]
#     - otel-collector
#     - query-service
#     - frontend

# Access the UI
# https://signoz.doku.local
```

Multi-container services automatically:
- Install required dependencies in correct order
- Configure network aliases for inter-container communication
- Set up proper startup dependencies
- Mount configuration files from the catalog

### Dependency Management

Services automatically install their dependencies:

```bash
# Installing SignOz automatically installs:
# 1. Zookeeper (required by ClickHouse)
# 2. ClickHouse (required by SignOz for data storage)
# 3. SignOz (the main service with 3 containers)

doku install signoz --yes

# Output:
# 📦 Dependencies required:
#   • zookeeper (latest)
#   • clickhouse (latest)
#   • signoz (latest)
#
# Installing dependency: zookeeper...
# ✓ zookeeper installed
#
# Installing dependency: clickhouse...
# ✓ clickhouse installed
#
# Installing dependency: signoz...
# ✓ signoz installed
```

Dependencies are defined in the catalog and automatically resolved:
- Prevents circular dependencies
- Installs in correct topological order
- Skips already-installed dependencies
- Configures inter-service communication

## Configuration

Doku stores configuration in `~/.doku/`:

```
~/.doku/
├── config.toml          # Main configuration
├── catalog/             # Service catalog
├── traefik/             # Traefik config
├── certs/               # SSL certificates
└── services/            # Service definitions
```

### Custom Domain

By default, Doku uses `doku.local`. You can customize:

```bash
doku init --domain mydev.local
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `doku init` | Initialize Doku on your system |
| `doku version` | Show version information |
| **CLI Management** | |
| `doku self upgrade` | Upgrade doku to the latest version |
| **Catalog** | |
| `doku catalog` | Browse available services |
| `doku catalog search <query>` | Search for services |
| `doku catalog show <service>` | Show service details |
| `doku catalog update` | Update catalog from GitHub |
| **Service Management** | |
| `doku install <service>` | Install a service from catalog |
| `doku install <name> --path=<dir>` | Install a custom project from Dockerfile |
| `doku list` | List all running services |
| `doku list --all` | List all services (including stopped) |
| `doku info <service>` | Show detailed service information |
| `doku env <service>` | Show environment variables |
| `doku env set <service> KEY=VALUE` | Set environment variables |
| `doku env unset <service> KEY` | Remove environment variables |
| `doku start <service>` | Start a stopped service |
| `doku stop <service>` | Stop a running service |
| `doku restart <service>` | Restart a service |
| `doku logs <service>` | View service logs |
| `doku logs <service> -f` | Follow service logs in real-time |
| `doku remove <service>` | Remove a service and its data |
| **Cleanup** | |
| `doku uninstall` | Uninstall Doku and clean up everything |

### Common Flags

- `--help, -h` - Show help for any command
- `--verbose, -v` - Verbose output
- `--quiet, -q` - Quiet mode (minimal output)
- `--yes, -y` - Skip confirmation prompts (for remove/uninstall)
- `--force, -f` - Force operation

### Install Flags

- `--path` - Path to project directory with Dockerfile (for custom projects)
- `--name, -n` - Custom instance name
- `--env, -e` - Environment variables (KEY=VALUE)
- `--memory` - Memory limit (e.g., 512m, 1g)
- `--cpu` - CPU limit (e.g., 0.5, 1.0)
- `--port, -p` - Port mappings (can be specified multiple times)
  - Format: `--port 5432` (maps container port to same host port)
  - Format: `--port 5433:5432` (maps container port 5432 to host port 5433)
- `--volume` - Volume mounts (host:container)
- `--internal` - Install as internal service (no external access)
- `--skip-deps` - Skip dependency installation
- `--no-auto-install-deps` - Prompt before installing dependencies

## Uninstalling

To completely remove Doku from your system:

```bash
# Uninstall with confirmation prompt
doku uninstall

# Force uninstall without prompts
doku uninstall --force

# Uninstall and remove mkcert CA certificates
doku uninstall --all
```

### What Gets Removed Automatically:

- ✅ All Docker containers managed by Doku
- ✅ All Docker volumes created by Doku
- ✅ Doku Docker network
- ✅ Configuration directory (`~/.doku/`)
- ✅ Doku binaries (`doku` and `doku-cli`)

### Manual Cleanup (Optional):

The uninstall command provides OS-specific instructions for:

1. **DNS entries** - Remove `*.doku.local` entries from `/etc/hosts` or resolver
2. **mkcert CA certificates** - Optionally remove with `mkcert -uninstall`

### Complete Removal:

```bash
# Run uninstall and immediately remove the binary
doku uninstall --force && rm -f ~/go/bin/doku ~/go/bin/doku-cli
```

## Requirements

- Docker (Desktop or Engine)
- macOS, Linux, or Windows
- Ports 80 and 443 available

## Development

```bash
# Clone the repository
git clone https://github.com/dokulabs/doku-cli
cd doku-cli

# Install dependencies
go mod download

# Build
make build

# Run
./bin/doku version
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📖 [Documentation](https://docs.doku.dev)
- 🐛 [Report Issues](https://github.com/dokulabs/doku-cli/issues)
- 💬 [Discussions](https://github.com/dokulabs/doku-cli/discussions)

## Project Status

**Status:** ✅ Production Ready (v0.2.0)

### Completed Features ✅

**Core Infrastructure:**
- ✅ Configuration management (TOML-based)
- ✅ Docker SDK integration with full container lifecycle
- ✅ SSL certificate generation (mkcert)
- ✅ Traefik reverse proxy setup with automatic routing
- ✅ DNS configuration (hosts file integration)
- ✅ Network management (doku-network bridge)

**Service Catalog:**
- ✅ GitHub-based catalog system
- ✅ Version management for services
- ✅ Service metadata (icons, descriptions, tags, links)
- ✅ Catalog browsing and search
- ✅ Automatic catalog updates

**Service Management:**
- ✅ Service installation with interactive prompts
- ✅ Service listing with filtering and status
- ✅ Service lifecycle (start, stop, restart)
- ✅ Service removal with cleanup
- ✅ Service information display
- ✅ Environment variable management with masking
- ✅ Log viewing with follow mode
- ✅ Resource limits (CPU/memory)
- ✅ Volume management
- ✅ Internal-only services (API Gateway pattern)
- ✅ Port mapping (single and multiple ports)

**Multi-Container & Dependencies (Phase 3 Complete!):**
- ✅ Multi-container service support
- ✅ Automatic dependency resolution
- ✅ Topological sorting for installation order
- ✅ Network alias automation for inter-container communication
- ✅ Configuration file mounting from catalog
- ✅ Container startup order management
- ✅ Dependency-aware removal (keeps dependencies)

**Custom Projects & Dynamic Configuration (Phase 4 Complete!):**
- ✅ Custom project installation with `--path` flag
- ✅ Automatic Dockerfile build and run
- ✅ Dynamic environment variable management
- ✅ `doku env set` command for updating variables
- ✅ `doku env unset` command for removing variables
- ✅ Auto-restart option for environment changes

**Installation & Distribution:**
- ✅ One-line installation via curl/wget
- ✅ Pre-built binaries for multiple platforms
- ✅ Go install support (@latest, @main, @version)
- ✅ Build from source option
- ✅ Self-upgrade command

**Utilities:**
- ✅ Complete uninstallation with automatic cleanup
- ✅ Fixed uninstall to remove all containers and volumes
- ✅ Version information
- ✅ Help system

### Planned Enhancements 📋
- 📋 Service health checks and monitoring
- 📋 Multi-project workspace support
- 📋 Service templates and custom definitions
- 📋 Environment profiles (dev/staging/prod)
- 📋 Backup/restore functionality
- 📋 Service update command
- 📋 Dashboard UI (web-based management)

---

Made with ❤️ for developers who want a better local development experience.
