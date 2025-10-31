# 👋 Start Here - Quick Onboarding

**Welcome back!** This guide helps you quickly get back into the Doku CLI project.

## 🎯 What is Doku CLI?

A local development tool that simplifies running Docker services with HTTPS, subdomain routing, and automatic service discovery. Think "Docker + Traefik made easy."

## 📍 Current Status

**Phase 4 Complete** - Service installation and management with API Gateway support

You can now:
- ✅ Initialize Doku (`doku init`)
- ✅ Install services from catalog (`doku install postgres`)
- ✅ Configure resources and environment
- ✅ Create internal-only services for microservices (`--internal` flag)

## 🚀 Quick Start

### 1. Build the Project
```bash
cd /Users/kesharinandan/Work/Experiment/dokulabs/doku-cli
go build -o ./bin/doku .
```

### 2. Test It Works
```bash
./bin/doku version
./bin/doku --help
./bin/doku install --help
```

### 3. View Available Services
```bash
cd ../doku-catalog
cat catalog.toml
cd ../doku-cli
```

## 📚 Documentation Files

**Quick Reference (Read First):**
- `SESSION_CONTEXT.md` - Quick overview, 5-minute read
- `START_HERE.md` - This file

**Detailed Information:**
- `DEVELOPMENT.md` - Complete technical documentation
- `README.md` - User-facing documentation
- `CHANGELOG.md` - Version history and changes

**API Gateway Pattern:**
- `../doku-catalog/gateway-pattern.md` - Microservices architecture guide

## 🎯 What You Were Working On

**Latest Feature:** API Gateway Pattern Support

You implemented the `--internal` flag to support enterprise microservices architectures where:
- Backend services are internal-only (no external access)
- API Gateway is the only public-facing service
- Services communicate via container names within Docker network

**Example Usage:**
```bash
# Internal backend services
doku install user-service --internal
doku install order-service --internal

# Public API Gateway
doku install spring-gateway --name api \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082
```

## 🔨 Key Implementation Files

**Most Important Files:**
1. `internal/service/installer.go` (353 lines)
   - Service installation logic
   - **Latest change:** `generateLabels()` now accepts `internal` parameter

2. `cmd/install.go` (316 lines)
   - Interactive installation command
   - **Latest change:** Added `--internal` flag

3. `internal/service/manager.go` (334 lines)
   - Service lifecycle operations (start, stop, etc.)
   - Methods exist but CLI commands not yet wired up

## 📋 Next Steps

If you want to continue development, here are the logical next tasks:

### Option 1: Complete Lifecycle Commands (Recommended)
Implement the remaining service management commands:
- `doku list` - List all services
- `doku start <service>` - Start service
- `doku stop <service>` - Stop service
- `doku logs <service>` - View logs
- `doku remove <service>` - Remove service

**Why:** Manager methods already exist in `internal/service/manager.go`, just need CLI wrappers.

### Option 2: Add Spring Gateway to Catalog
Add Spring Cloud Gateway service definition to `doku-catalog/catalog.toml`

### Option 3: Test Current Features
Install and test services with the new `--internal` flag.

## 🧪 Testing Current Features

```bash
# 1. Check Docker is running
docker ps

# 2. Initialize Doku (if not done)
./bin/doku init

# 3. Test basic installation
./bin/doku catalog update
./bin/doku install redis --name test-redis --yes

# 4. Test internal flag (requires service in catalog)
./bin/doku install <service> --internal

# 5. Check Traefik
open https://traefik.doku.local
```

## 🐛 Common Issues

### Build fails
```bash
go mod tidy
go clean
go build -o ./bin/doku .
```

### Docker not running
```bash
docker ps
# If error, start Docker Desktop
```

### Flag conflicts
- Volume flag has no shorthand (use `--volume`, not `-v`)
- `-v` is reserved for `--verbose`

## 💡 Development Tips

### Read order for understanding:
1. `SESSION_CONTEXT.md` - Current state
2. `pkg/types/types.go` - Data structures
3. `cmd/install.go` - See how commands work
4. `internal/service/installer.go` - See implementation

### Key concepts:
- **Traefik labels** control routing (see `generateLabels()`)
- **doku-network** is the Docker bridge network
- **Container names** are used for service discovery
- **Config** is stored in `~/.doku/config.toml`

### Making changes:
1. Edit files
2. `go build -o ./bin/doku .`
3. `./bin/doku <command>` to test
4. Check logs if issues

## 🎓 Architecture Overview

```
User Request
    ↓
Cobra CLI Command (cmd/)
    ↓
Service Layer (internal/service/)
    ↓
Docker Client (internal/docker/)
    ↓
Docker API
    ↓
Containers & Networks

Config Manager (internal/config/)
    ↓
~/.doku/config.toml
```

**Routing:**
```
Browser → https://service.doku.local
    ↓
Traefik (doku-traefik container)
    ↓
Service Container (via labels)
    ↓
Application
```

## 🔗 Related Repositories

- **Main CLI:** `/Users/kesharinandan/Work/Experiment/dokulabs/doku-cli`
- **Catalog:** `/Users/kesharinandan/Work/Experiment/dokulabs/doku-catalog`

Both are local repositories not yet pushed to GitHub.

## 🆘 Need More Context?

1. **Quick overview:** Read `SESSION_CONTEXT.md`
2. **Full details:** Read `DEVELOPMENT.md`
3. **User docs:** Read `README.md`
4. **Version history:** Read `CHANGELOG.md`
5. **API Gateway:** Read `../doku-catalog/gateway-pattern.md`

## 📞 Your Use Case

You wanted to replicate your organization's architecture:
- Spring API Gateway as entry point
- Multiple microservices behind it
- Gateway handles authentication/authorization
- Backend services not exposed externally

**Solution implemented:** `--internal` flag allows this exact pattern.

---

**Pro tip:** Start every session by running:
```bash
cat SESSION_CONTEXT.md  # Quick refresh
go build -o ./bin/doku .  # Ensure it builds
./bin/doku version  # Verify it works
```

Happy coding! 🚀
