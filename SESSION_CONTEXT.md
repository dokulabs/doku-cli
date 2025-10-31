# Session Context - Quick Reference

**Last Session Date:** October 31, 2024

## 🎯 Current Status: Phase 4 Complete

Service installation and management system is fully implemented with API Gateway pattern support.

## 📁 Project Locations
- CLI: `/Users/kesharinandan/Work/Experiment/dokulabs/doku-cli`
- Catalog: `/Users/kesharinandan/Work/Experiment/dokulabs/doku-catalog`

## ✅ What's Working

### Commands Available
- `doku init` - Initialize Doku environment
- `doku catalog update|search|list|info` - Catalog operations
- `doku install <service>` - Install services with full options

### Key Features
- Docker container management
- Traefik reverse proxy with HTTPS
- Local SSL certificates (mkcert)
- Service catalog with 8 services
- Interactive installation with prompts
- Resource limits (CPU/memory)
- Volume management
- **NEW:** Internal-only services (--internal flag)

### Service Installation Options
```bash
doku install <service>[:<version>] \
  --name <custom-name> \
  --env KEY=VALUE \
  --memory 512m \
  --cpu 0.5 \
  --volume /host:/container \
  --internal \  # NEW: No Traefik exposure
  --yes         # Skip prompts
```

## 🏗️ API Gateway Pattern (Latest Feature)

**Use Case:** Microservices with API Gateway (like Spring Cloud Gateway)

```bash
# Backend services (internal only)
doku install user-service --internal
doku install order-service --internal
doku install payment-service --internal

# API Gateway (public facing)
doku install spring-gateway --name api \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082
```

**Result:**
- Only API Gateway exposed: `https://api.doku.local`
- Backend services accessible only within doku-network
- Services communicate via container names

**Documentation:** `doku-catalog/gateway-pattern.md`

## 📋 File Structure

### Core Implementation Files
```
cmd/
  install.go        (316 lines) - Installation command
  init.go           (XXX lines) - Initialization
  catalog.go        (XXX lines) - Catalog commands

internal/service/
  installer.go      (353 lines) - Service installation logic
  manager.go        (334 lines) - Lifecycle management

internal/config/
  config.go         (392 lines) - Config management

internal/docker/
  client.go         - Docker SDK wrapper
  network.go        - Network operations
  volume.go         - Volume operations

internal/catalog/
  manager.go        - Catalog operations
  parser.go         - TOML parsing

internal/traefik/
  manager.go        - Traefik setup

internal/certs/
  manager.go        - SSL certificates
```

## 🔧 Recent Changes

### generateLabels() - internal/service/installer.go:262
```go
// Added 'internal bool' parameter
func (i *Installer) generateLabels(..., internal bool) map[string]string {
    // Management labels (always added)
    labels["managed-by"] = "doku"
    labels["doku.service"] = service.Name

    // Traefik labels (only if NOT internal)
    if !internal && (spec.Protocol == "http" || spec.Protocol == "https") {
        labels["traefik.enable"] = "true"
        // ... routing labels
    } else if internal {
        labels["traefik.enable"] = "false"
    }
}
```

### Install Command - cmd/install.go
- Added `--internal` flag
- Updated examples
- Fixed volume flag conflict (removed -v shorthand)

## 🚧 Not Yet Implemented

### Missing Commands (Phase 5)
- `doku list` - List services
- `doku stop <service>` - Stop service
- `doku start <service>` - Start service
- `doku restart <service>` - Restart service
- `doku remove <service>` - Remove service
- `doku logs <service>` - View logs
- `doku status <service>` - Check status
- `doku stats <service>` - Resource usage

**Note:** Manager methods exist in `internal/service/manager.go` but CLI commands not yet implemented.

## 🐛 Known Issues

1. Volume flag has no shorthand (conflict with --verbose/-v)
2. Health checks not implemented
3. Service dependency management manual
4. No multi-project support yet

## 🧪 Testing Commands

```bash
# Build
cd /Users/kesharinandan/Work/Experiment/dokulabs/doku-cli
go build -o ./bin/doku .

# Test
./bin/doku version
./bin/doku --help
./bin/doku install --help

# Check catalog
cd ../doku-catalog
cat catalog.toml
```

## 📦 Dependencies

- Go 1.21+
- Docker SDK v25
- Cobra (CLI framework)
- Traefik v2.10
- survey/v2 (interactive prompts)
- TOML parser

## 🔑 Key Methods to Know

### Installer (internal/service/installer.go)
- `Install(opts)` - Main installation flow
- `generateLabels(name, service, spec, internal)` - Traefik labels
- `generateInstanceName(service, version)` - Unique naming
- `applyResourceLimits(hostConfig, memory, cpu)` - Constraints

### Manager (internal/service/manager.go)
- `Start(instanceName)` - Start service
- `Stop(instanceName)` - Stop service
- `Remove(instanceName, force)` - Remove + cleanup
- `GetLogs(instanceName, follow)` - Stream logs
- `GetStatus(instanceName)` - Container state

### Config (internal/config/config.go)
- `Get()` - Load config
- `Update(fn)` - Modify config
- `AddInstance(instance)` - Save instance
- `HasInstance(name)` - Check existence

## 💡 Next Session Tasks

1. **Implement lifecycle commands** (Phase 5)
   - Create `cmd/start.go`, `cmd/stop.go`, etc.
   - Wire up to service.Manager methods
   - Add tests

2. **Add Spring Gateway to catalog**
   - Create service definition in `doku-catalog/catalog.toml`
   - Document configuration options

3. **Test API Gateway scenario end-to-end**
   - Install internal services
   - Install gateway
   - Verify isolation

## 📞 User Context

**Organization Setup:**
- Uses Spring API Gateway in production
- Multiple microservices behind gateway
- API Gateway handles JWT validation & authorization
- Wanted to replicate locally with Doku

**Solution Provided:**
- `--internal` flag for backend services
- Container name-based service discovery
- Public gateway + internal services pattern

## 🔍 Where Things Are

**Configuration:** `~/.doku/config.toml`
**Certificates:** `~/.doku/certs/`
**Catalog:** `~/.doku/catalog/`
**Traefik:** `~/.doku/traefik/`

**Containers:**
- Network: `doku-network`
- Traefik: `doku-traefik`
- Services: `doku-<instance-name>`

**Volumes:** `doku-<instance>-<path-hash>`

## 📚 Full Details

See `DEVELOPMENT.md` for comprehensive documentation.

---

**Quick command to show this file:**
```bash
cat /Users/kesharinandan/Work/Experiment/dokulabs/doku-cli/SESSION_CONTEXT.md
```
