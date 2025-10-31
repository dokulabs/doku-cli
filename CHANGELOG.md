# Changelog

All notable changes to Doku CLI will be documented in this file.

## [Unreleased]

### Added
- Service installation with interactive prompts
- Resource limits (CPU and memory)
- Volume management and persistence
- Internal-only services support (`--internal` flag)
- API Gateway pattern support
- Multiple environment variable configuration methods
- Interactive configuration prompts based on catalog specs
- Confirmation prompts before installation

### In Development
- Lifecycle commands (start, stop, restart, list, logs, remove, status)
- Service health checks
- Project management

## [0.1.0-alpha] - 2024-10-30

### Added - Phase 4: Service Management

#### Service Installation (`cmd/install.go`)
- Interactive installation command with prompts
- Service version selection (e.g., `postgres:16`)
- Custom instance naming with `--name` flag
- Environment variable overrides with `--env` flag
- Resource limits with `--memory` and `--cpu` flags
- Volume mounts with `--volume` flag
- Skip confirmation with `--yes` flag
- Internal-only services with `--internal` flag
- Configuration prompts based on catalog service specs
- Support for bool, select, and string configuration types

#### Service Installer (`internal/service/installer.go`)
- Full service installation workflow
- Docker image pulling with progress
- Container creation and configuration
- Traefik label generation for routing
- Resource limit application (CPU/memory)
- Named volume creation and mounting
- Network connection (doku-network)
- Instance name generation with conflict resolution
- Environment variable merging (defaults + overrides)
- Connection string generation
- Service URL building
- Internal service support (no Traefik exposure)

**Key Methods:**
- `Install(opts InstallOptions)` - Main installation orchestration
- `generateLabels(name, service, spec, internal)` - Traefik routing labels
- `generateInstanceName(service, version)` - Unique instance naming
- `mergeEnvironment(defaults, overrides)` - Config merging
- `applyResourceLimits(hostConfig, memory, cpu)` - Resource constraints
- `buildServiceURL(instanceName)` - URL generation
- `buildConnectionString(instance, spec, env)` - Connection strings

#### Service Manager (`internal/service/manager.go`)
- Service lifecycle management
- Container state tracking
- Log retrieval with streaming support
- Resource statistics monitoring
- Volume cleanup on removal

**Key Methods:**
- `Start(instanceName)` - Start stopped service
- `Stop(instanceName)` - Graceful shutdown with timeout
- `Restart(instanceName)` - Restart service
- `Remove(instanceName, force)` - Full cleanup (container, volumes, network)
- `GetLogs(instanceName, follow)` - Stream logs
- `GetStatus(instanceName)` - Real-time container state
- `GetStats(instanceName)` - CPU/memory usage
- `RefreshStatus()` - Update all instance statuses
- `GetConnectionInfo(instanceName)` - Connection details

#### Configuration Enhancements (`internal/config/config.go`)
- `HasInstance(name)` - Check instance existence
- `UpdateInstance(name, instance)` - Update existing instance

### Added - Phase 3: Catalog System

#### Catalog Management (`internal/catalog/`)
- Service catalog with version support
- TOML-based service definitions
- Git-based catalog updates
- Service search and discovery
- Category and tag-based organization

#### Commands
- `doku catalog update` - Download/update catalog
- `doku catalog list` - List all services
- `doku catalog search <query>` - Search services
- `doku catalog info <service>` - Service details

### Added - Phase 2: Setup & Initialization

#### Certificate Management (`internal/certs/manager.go`)
- mkcert integration for local SSL
- Root CA generation
- Service certificate creation
- Certificate trust installation

#### DNS Management (`internal/dns/manager.go`)
- dnsmasq configuration
- /etc/hosts file management
- *.doku.local domain resolution

#### Traefik Management (`internal/traefik/manager.go`)
- Traefik container setup
- Dashboard configuration
- HTTP/HTTPS entrypoints
- Certificate resolver
- Dynamic configuration

#### Initialization Command (`cmd/init.go`)
- Interactive setup wizard
- Docker availability check
- mkcert installation verification
- SSL certificate generation
- DNS configuration
- Traefik deployment
- Catalog download

### Added - Phase 1: Core Infrastructure

#### Configuration (`internal/config/config.go`)
- TOML-based configuration
- Stored in `~/.doku/config.toml`
- Instance tracking
- Project management
- Preferences (domain, protocol, catalog version)
- Network configuration
- Traefik settings
- Certificate paths

#### Docker Integration (`internal/docker/`)
- Docker SDK wrapper (`client.go`)
- Container operations (create, start, stop, remove, inspect)
- Network management (`network.go`)
- Volume operations (`volume.go`)
- Image operations (pull, list, remove)
- Log streaming support

#### Type Definitions (`pkg/types/types.go`)
- Configuration structures
- Service instance models
- Catalog service specs
- Network configuration
- Traefik configuration
- Resource limits
- Installation options

#### CLI Framework (`cmd/`)
- Cobra-based command structure
- Root command with global flags
- Version command
- Viper configuration binding

## Key Features by Version

### v0.1.0-alpha

**Architecture:**
- Docker bridge networking (doku-network)
- Traefik reverse proxy with dynamic configuration
- Local SSL certificates via mkcert
- Subdomain routing (*.doku.local)
- Container-based service discovery

**Service Management:**
- Interactive installation with prompts
- Multi-version support
- Resource limits (CPU/memory)
- Volume persistence
- Environment variable configuration
- Connection string generation
- Internal-only services (API Gateway pattern)

**Catalog:**
- Git-based service catalog
- 8+ pre-configured services
- Version management
- Search and discovery
- Service metadata (tags, categories, resources)

## Breaking Changes

None yet (alpha version).

## Bug Fixes

### Build Errors Fixed (2024-10-30)

1. **Missing config.Manager methods**
   - Added `HasInstance()` and `UpdateInstance()` methods
   - Fixed AddInstance signature (removed extra parameter)

2. **DisconnectContainer signature mismatch**
   - Added missing `force bool` parameter to all calls

3. **ContainerLogs signature mismatch**
   - Rewrote GetLogs() to properly handle io.ReadCloser
   - Removed unsupported tail parameter

4. **Variable shadowing in generateInstanceName**
   - Changed loop variable from `i` to `num` to avoid shadowing receiver

5. **GetStats return type mismatch**
   - Changed return type from `map[string]interface{}` to `dockerTypes.ContainerStats`

6. **Flag shorthand conflict**
   - Removed `-v` shorthand from `--volume` flag (conflicts with `--verbose`)
   - Used `StringSliceVar` instead of `StringSliceVarP`

## Technical Debt

1. Volume flag has no shorthand (due to conflict with --verbose)
2. No health checks implemented yet
3. Service dependency management is manual
4. No multi-project isolation yet
5. Limited error recovery in installation flow

## Dependencies

- Go 1.21+
- Docker SDK v25
- Cobra CLI framework
- Viper configuration
- survey/v2 (interactive prompts)
- BurntSushi/toml (TOML parser)
- fatih/color (terminal colors)
- mkcert (local certificates)
- Traefik v2.10

## Documentation

- [DEVELOPMENT.md](DEVELOPMENT.md) - Comprehensive development guide
- [SESSION_CONTEXT.md](SESSION_CONTEXT.md) - Quick reference for context
- [README.md](README.md) - User-facing documentation
- [doku-catalog/gateway-pattern.md](../doku-catalog/gateway-pattern.md) - API Gateway pattern guide

---

## Format

This changelog follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Categories:**
- `Added` - New features
- `Changed` - Changes to existing functionality
- `Deprecated` - Soon-to-be removed features
- `Removed` - Removed features
- `Fixed` - Bug fixes
- `Security` - Security fixes
