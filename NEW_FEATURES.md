# New Features: Custom Projects & Environment Management

## Overview

Doku CLI now supports installing custom applications directly from Dockerfiles and managing environment variables dynamically.

## What's New

### 1. 🚀 Custom Project Installation (`--path` flag)

Install your own applications with a single command:

```bash
# Install a custom frontend
doku install frontend --path=./frontend

# Install an internal microservice
doku install user-service --path=./user-service --internal
```

**Features:**
- ✅ Automatic Docker image build
- ✅ Seamless Traefik integration
- ✅ Support for internal services
- ✅ Environment variable injection
- ✅ Port mapping and volume mounts

### 2. 🔧 Dynamic Environment Variable Management

Update configuration without rebuilding containers:

```bash
# Set variables
doku env set frontend API_KEY=secret DEBUG=true

# Remove variables
doku env unset frontend OLD_KEY

# Auto-restart after changes
doku env set frontend API_KEY=secret --restart
```

## Quick Examples

### Full-Stack Application

```bash
# Install dependencies
doku install postgres redis

# Backend (internal)
doku install api --path=./backend --internal --port 4000

# Frontend (public)
doku install frontend --path=./frontend --port 3000 \
  --env API_URL=http://api:4000
```

### Microservices with API Gateway

```bash
# Install microservices (internal)
doku install user-service --path=./user-service --internal --port 8081
doku install order-service --path=./order-service --internal --port 8082

# Install gateway (public)
doku install gateway --path=./gateway --port 8080 \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082

# Install frontend (public)
doku install frontend --path=./frontend --port 3000 \
  --env API_BASE_URL=https://gateway.doku.local
```

## Documentation

- 📖 [Complete Guide](./CUSTOM_PROJECTS_GUIDE.md) - Full documentation with examples
- ⚡ [Quick Reference](./QUICK_REFERENCE.md) - Command cheat sheet

## Benefits

### For Developers
- **Single Command**: Build, configure, and run in one step
- **Consistent Workflow**: Same command for catalog and custom services
- **Quick Iteration**: Update env vars without rebuilding

### For DevOps
- **Internal Services**: Mark backend services as internal-only
- **Environment Control**: Dynamic configuration management
- **Easy Integration**: Works with existing Docker workflows

### For Teams
- **Standardized Setup**: Same commands across all projects
- **Service Discovery**: Automatic DNS and networking
- **Clean URLs**: HTTPS with automatic SSL certificates

## Upgrade Guide

### From `doku project` Commands

**Before:**
```bash
doku project add ./myapp
doku project build myapp
doku project run myapp
```

**After:**
```bash
doku install myapp --path=./myapp
```

**Note:** Old `doku project` commands still work for advanced scenarios.

## Command Reference

### Installation
```bash
doku install <name> --path=<directory> [options]

Options:
  --path string          Path to project with Dockerfile
  --internal             Install as internal service
  --port strings         Port mappings
  --env strings          Environment variables (KEY=VALUE)
  --yes                  Skip confirmation
```

### Environment Management
```bash
# View
doku env <service>
doku env <service> --show-values

# Set
doku env set <service> KEY=VALUE [KEY2=VALUE2...] [--restart]

# Unset
doku env unset <service> KEY [KEY2...] [--restart]
```

## Architecture Support

✅ Monolith applications
✅ Microservices architecture
✅ Full-stack applications
✅ API Gateway pattern
✅ Service mesh setups
✅ Background workers
✅ Multi-container applications

## Requirements

- Doku CLI v0.1.0+
- Docker Desktop or Docker Engine
- Project with Dockerfile

## Compatibility

These features are fully compatible with:
- ✅ Existing catalog services
- ✅ Existing Traefik configuration
- ✅ Existing Docker network
- ✅ All Doku commands (`list`, `logs`, `restart`, etc.)

## Next Steps

1. **Read the docs**: [CUSTOM_PROJECTS_GUIDE.md](./CUSTOM_PROJECTS_GUIDE.md)
2. **Try it out**: Install your first custom project
3. **Give feedback**: Open an issue on GitHub

## Support

- 📚 [Documentation](./CUSTOM_PROJECTS_GUIDE.md)
- 🐛 [Report Issues](https://github.com/dokulabs/doku-cli/issues)
- 💬 [Discussions](https://github.com/dokulabs/doku-cli/discussions)

---

**Version:** 1.0.0
**Release Date:** 2025-11-18
