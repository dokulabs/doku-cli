# Session Context - Quick Reference

**Last Session Date:** October 31, 2024

## 🎯 Current Status: Phase 4 & 5 Complete! 🎉

Complete service lifecycle management and CLI self-upgrade capabilities are fully implemented.

## 📁 Project Locations
- CLI: `/Users/kesharinandan/Work/Experiment/dokulabs/doku-cli`
- Catalog: `/Users/kesharinandan/Work/Experiment/dokulabs/doku-catalog`

## ✅ What's Working

### All Commands Available
- `doku init` - Initialize Doku environment
- `doku version` - Show version info
- `doku self upgrade` - Upgrade CLI to latest version
- `doku catalog` - Browse/search/update catalog
- `doku install <service>` - Install services
- `doku list [--all]` - List services
- `doku info <service>` - Detailed service information
- `doku env <service>` - Show environment variables
- `doku start <service>` - Start services
- `doku stop <service>` - Stop services
- `doku restart <service>` - Restart services
- `doku logs <service> [-f]` - View logs
- `doku remove <service>` - Remove services
- `doku uninstall` - Complete cleanup

### Key Features
- ✅ Docker container management
- ✅ Traefik reverse proxy with HTTPS
- ✅ Traefik management (start, stop, restart, logs, info)
- ✅ Local SSL certificates (mkcert)
- ✅ Service catalog with 8+ services
- ✅ Interactive installation with prompts
- ✅ Resource limits (CPU/memory)
- ✅ Volume management
- ✅ Internal-only services (--internal flag)
- ✅ Environment variable management with masking
- ✅ Self-upgrade capability
- ✅ Complete lifecycle management

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

## 🚀 Recently Implemented (Phase 5)

### ✅ All Lifecycle Commands Complete!
- ✅ `doku list` - List services with filtering and status
- ✅ `doku stop <service>` - Stop service (inc. Traefik)
- ✅ `doku start <service>` - Start service (inc. Traefik)
- ✅ `doku restart <service>` - Restart service (inc. Traefik)
- ✅ `doku remove <service>` - Remove service with confirmation
- ✅ `doku logs <service>` - View/stream logs (inc. Traefik)
- ✅ `doku info <service>` - Service details (inc. Traefik)
- ✅ `doku env <service>` - Environment variables with masking
- ✅ `doku self upgrade` - CLI self-upgrade capability

### ✅ Traefik Management Support
All commands now support managing Traefik reverse proxy:
- Accepts both `traefik` and `doku-traefik` as service name
- Special handling for system component operations
- Dashboard URL display for operations
- Warnings when stopping Traefik
- Prevention of Traefik removal (use `doku uninstall` instead)

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

## 💡 Potential Future Enhancements

1. **Service Updates**
   - `doku update <service>` - Update to latest version
   - Automated backup before updates

2. **Health Monitoring**
   - Service health checks
   - Automated recovery
   - Status dashboard

3. **Resource Monitoring**
   - `doku stats <service>` - Real-time resource usage
   - Historical metrics tracking
   - Alerts on resource limits

4. **Backup & Restore**
   - `doku backup <service>` - Create backups
   - `doku restore <service>` - Restore from backup
   - Automated backup scheduling

5. **Advanced Features**
   - Service dependency management
   - Multi-project isolation
   - Service templates
   - Web-based dashboard UI

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
