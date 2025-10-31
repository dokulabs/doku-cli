# Doku CLI

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

## Quick Start

### Installation

```bash
# Download the latest release
curl -sSL https://get.doku.dev | bash

# Or using Go (recommended - always gets the latest version)
go install github.com/dokulabs/doku-cli/cmd/doku@latest

# Or install a specific version
go install github.com/dokulabs/doku-cli/cmd/doku@v0.1.0
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
doku install postgres --version 14 --name postgres-14

# Install with resource limits
doku install redis --memory 512m --cpus 1
```

### Manage Services

```bash
# List installed services
doku list

# Start a service
doku start postgres

# Stop a service
doku stop postgres

# View logs
doku logs postgres -f

# Get service info
doku info postgres

# Remove a service
doku remove postgres
```

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

Browse the full catalog:

```bash
doku catalog
```

**Databases:**
- PostgreSQL
- MySQL
- MongoDB
- Redis
- MariaDB

**Message Queues:**
- RabbitMQ
- Apache Kafka
- NATS

**Search:**
- Elasticsearch
- Meilisearch
- OpenSearch

**Monitoring:**
- Prometheus
- Grafana

And many more...

## Usage Examples

### Multiple Versions

Run multiple versions of the same service:

```bash
doku install postgres --version 14 --name postgres-14
doku install postgres --version 16 --name postgres-16

# Both running simultaneously on different ports
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

### Local Projects

Build and run your own projects:

```bash
cd ~/my-app
doku project add

# Doku will detect your Dockerfile and configure routing
# Access your app at: https://my-app.doku.local
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
| `doku install <service>` | Install a service |
| `doku start <instance>` | Start a service |
| `doku stop <instance>` | Stop a service |
| `doku restart <instance>` | Restart a service |
| `doku list` | List all services |
| `doku remove <instance>` | Remove a service |
| `doku info <instance>` | Get service details |
| `doku logs <instance>` | View service logs |
| `doku catalog` | Browse available services |
| `doku dashboard` | Open Traefik dashboard |
| `doku status` | System status overview |
| `doku update` | Update service catalog |
| `doku uninstall` | Uninstall Doku and clean up everything |
| `doku version` | Show version info |

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

**Status:** 🚧 Under active development (v0.1.0-alpha)

### Completed Features ✅
- ✅ Configuration management
- ✅ Docker integration
- ✅ SSL certificate generation (mkcert)
- ✅ Traefik reverse proxy setup
- ✅ Service catalog system
- ✅ Service installation with interactive prompts
- ✅ Resource limits (CPU/memory)
- ✅ Volume management
- ✅ Internal-only services (API Gateway pattern)

### In Progress 🚧
- 🚧 Lifecycle commands (start, stop, list, logs, etc.)
- 🚧 Service health checks
- 🚧 Multi-project support

### Planned 📋
- 📋 Dependency management
- 📋 Service templates
- 📋 Environment profiles
- 📋 Backup/restore functionality

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed progress and [SESSION_CONTEXT.md](SESSION_CONTEXT.md) for quick reference.

---

Made with ❤️ for developers who want a better local development experience.
