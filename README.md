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
- 📋 **Service catalog** - Curated collection of 25+ popular development services
- 🔄 **Full lifecycle management** - Start, stop, restart, and remove services with ease
- 🧩 **Multi-container services** - Deploy complex services with multiple containers
- 🔗 **Dependency management** - Automatic installation of service dependencies
- 🔌 **Port mapping** - Map container ports to host for direct access via localhost
- 🐳 **Custom projects** - Build and run from your own Dockerfiles with `--path` flag
- ⚡ **Dynamic configuration** - Update environment variables without rebuilding containers

## Quick Start

### Installation

```bash
# Using curl
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash

# Or using wget
wget -qO- https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

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

# View with actual values (unmask sensitive data)
doku env frontend --show-values

# Export format for shell sourcing
doku env frontend --export > frontend.env

# Set new variables
doku env set frontend API_KEY=newsecret DEBUG=true

# Auto-restart after changes
doku env set frontend API_KEY=newsecret --restart

# Remove variables
doku env unset frontend OLD_KEY

# Interactive editing (add, edit, delete variables)
doku env edit frontend
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

**📋 Browse the complete service catalog with install commands:**
**[Doku Service Catalog](https://github.com/dokulabs/doku-catalog)**

The catalog includes 25 services: PostgreSQL, MySQL, MongoDB, MariaDB, ClickHouse, Redis, Memcached, RabbitMQ, Kafka, Elasticsearch, Grafana, Prometheus, Jaeger, SigNoz, Sentry, Dozzle, Nginx, MailHog, Adminer, phpMyAdmin, LocalStack, MinIO, Vault, Keycloak, and Zookeeper.

```bash
# Browse services locally
doku catalog
doku catalog --category database
doku catalog search postgres
doku catalog show postgres
```

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
| `doku start <service>` | Start a stopped service |
| `doku stop <service>` | Stop a running service |
| `doku restart <service>` | Restart a service |
| `doku logs <service>` | View service logs |
| `doku logs <service> -f` | Follow service logs in real-time |
| `doku remove <service>` | Remove a service and its data |
| **Environment Variables** | |
| `doku env <service>` | Show environment variables |
| `doku env <service> --show-values` | Show actual values (unmask sensitive data) |
| `doku env <service> --export` | Output in shell export format |
| `doku env set <service> KEY=VALUE` | Set environment variables |
| `doku env unset <service> KEY` | Remove environment variables |
| `doku env edit <service>` | Interactively edit environment variables |
| **Custom Projects** | |
| `doku project add <path>` | Add a custom project with Dockerfile |
| `doku project list` | List all registered projects |
| `doku project build <name>` | Build a project's Docker image |
| `doku project run <name>` | Run a project's container |
| `doku project remove <name>` | Remove a project |
| **Configuration** | |
| `doku config list` | List all configuration settings |
| `doku config get <key>` | Get a specific config value |
| `doku config set <key> <value>` | Set a config value |
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

### Project Flags

**`doku project add`:**
- `--name, -n` - Project name (defaults to directory name)
- `--dockerfile` - Path to Dockerfile (default: ./Dockerfile)
- `--port, -p` - Main port to expose
- `--ports` - Additional port mappings (host:container)
- `--env, -e` - Environment variables (KEY=VALUE)
- `--depends` - Service dependencies (e.g., postgres:16,redis)
- `--domain` - Custom domain (default: doku.local)
- `--internal` - Internal only (no Traefik/HTTPS)

**`doku project build`:**
- `--no-cache` - Build without using cache
- `--pull` - Pull base image before building
- `--tag, -t` - Custom tag for the image

**`doku project run`:**
- `--build` - Build image before running
- `--install-deps` - Automatically install missing dependencies
- `--detach, -d` - Run in background (default: true)

**`doku project remove`:**
- `--image` - Also remove the Docker image
- `--yes, -y` - Skip confirmation prompt

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

- 🐛 [Report Issues](https://github.com/dokulabs/doku-cli/issues)
- 💬 [Discussions](https://github.com/dokulabs/doku-cli/discussions)

---

Made with ❤️ for developers who want a better local development experience.
