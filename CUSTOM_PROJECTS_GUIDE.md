# Custom Projects & Environment Management Guide

This guide covers the new features for installing custom Dockerfile projects and managing environment variables in Doku CLI.

## Table of Contents

- [Overview](#overview)
- [New Features](#new-features)
- [Quick Start](#quick-start)
- [Installation Methods](#installation-methods)
- [Environment Variable Management](#environment-variable-management)
- [Complete Workflow Examples](#complete-workflow-examples)
- [Advanced Configuration](#advanced-configuration)
- [Troubleshooting](#troubleshooting)
- [Command Reference](#command-reference)

---

## Overview

Doku now supports installing custom applications from Dockerfiles directly using the `install` command, making it easy to run your microservices alongside catalog services. You can also manage environment variables dynamically without recreating containers.

### Key Benefits

- **Single Command Installation**: Build and run custom projects with one command
- **Seamless Integration**: Custom projects work with Doku's networking and Traefik
- **Internal Services**: Mark services as internal-only (no external access)
- **Dynamic Configuration**: Update environment variables without rebuilding
- **Consistent Workflow**: Use the same `doku install` command for both catalog and custom services

---

## New Features

### 1. Custom Project Installation (`--path` flag)

Install services directly from Dockerfiles:

```bash
doku install frontend --path=./frontend
```

**What it does:**
1. Adds the project to Doku
2. Builds the Docker image from your Dockerfile
3. Starts the container with proper networking
4. Integrates with Traefik (or marks as internal)

### 2. Environment Variable Management

Set and update environment variables for any service:

```bash
# Set variables
doku env set frontend API_KEY=secret

# Remove variables
doku env unset frontend OLD_KEY

# Auto-restart after changes
doku env set frontend DEBUG=true --restart
```

### 3. Internal Services (`--internal` flag)

Mark services as internal-only (no Traefik exposure):

```bash
doku install user-service --path=./user-service --internal
```

Internal services are only accessible within the `doku-network` by their container name.

---

## Quick Start

### Prerequisites

1. Doku CLI installed and initialized:
   ```bash
   doku init
   ```

2. Project with a Dockerfile:
   ```
   my-app/
   ├── Dockerfile
   ├── src/
   └── package.json
   ```

### Basic Installation

```bash
# Install a custom frontend application
doku install frontend --path=./my-frontend-app

# Access it at: https://frontend.doku.local
```

---

## Installation Methods

### Method 1: Public Service (Default)

Services are exposed via Traefik with HTTPS and a clean URL.

```bash
doku install frontend --path=./frontend --port 3000
```

**Result:**
- URL: `https://frontend.doku.local`
- Accessible from your browser
- Automatic SSL certificate
- Integrated with Traefik

### Method 2: Internal Service

Services only accessible within the Docker network.

```bash
doku install user-service --path=./user-service --internal --port 8081
```

**Result:**
- No external URL
- Accessible as `http://user-service:8081` from other containers
- Not exposed via Traefik
- Ideal for backend microservices

### Method 3: With Environment Variables

```bash
doku install api --path=./api \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/mydb \
  --env API_KEY=secret123 \
  --env NODE_ENV=production \
  --port 8080
```

### Method 4: With Custom Name

```bash
doku install my-api --name api-v2 --path=./api
```

**Result:**
- Instance name: `api-v2`
- URL: `https://api-v2.doku.local`

### Method 5: Skip Confirmation

```bash
doku install frontend --path=./frontend --yes
```

Useful for automation and CI/CD pipelines.

---

## Environment Variable Management

### View Environment Variables

```bash
# Show variables (sensitive values masked)
doku env frontend

# Show actual values
doku env frontend --show-values

# Export format (for shell)
doku env frontend --export > frontend.env
source frontend.env
```

### Set Environment Variables

```bash
# Set a single variable
doku env set frontend API_KEY=new_secret

# Set multiple variables
doku env set frontend \
  API_URL=https://api.example.com \
  NODE_ENV=production \
  DEBUG=false

# Set and auto-restart
doku env set frontend API_KEY=new_secret --restart
```

**Output Example:**
```
Updating environment variables for frontend

  API_KEY: old_value → new_value
  DEBUG: true

✓ Environment variables updated

⚠️  Note: Service needs to be restarted for changes to take effect
   Run: doku restart frontend
```

### Remove Environment Variables

```bash
# Remove a single variable
doku env unset frontend OLD_API_KEY

# Remove multiple variables
doku env unset frontend OLD_KEY DEPRECATED_VAR

# Remove and auto-restart
doku env unset frontend OLD_KEY --restart
```

### When to Restart

Environment variable changes are saved immediately but require a container restart to take effect:

**Option 1: Auto-restart with flag**
```bash
doku env set frontend API_KEY=new --restart
```

**Option 2: Manual restart**
```bash
doku env set frontend API_KEY=new
doku restart frontend
```

---

## Complete Workflow Examples

### Example 1: Full-Stack Application with API Gateway

This example shows a complete microservices setup with Spring Gateway.

#### Project Structure

```
my-project/
├── frontend/
│   ├── Dockerfile
│   └── src/
├── gateway/
│   ├── Dockerfile
│   └── src/
├── user-service/
│   ├── Dockerfile
│   └── src/
├── order-service/
│   ├── Dockerfile
│   └── src/
└── payment-service/
    ├── Dockerfile
    └── src/
```

#### Step 1: Install Dependencies

```bash
# Install PostgreSQL for shared database
doku install postgres:16
```

#### Step 2: Install Backend Microservices (Internal)

```bash
# User service
doku install user-service \
  --path=./user-service \
  --internal \
  --port 8081 \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/users \
  --env SERVICE_PORT=8081

# Order service
doku install order-service \
  --path=./order-service \
  --internal \
  --port 8082 \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/orders \
  --env SERVICE_PORT=8082

# Payment service
doku install payment-service \
  --path=./payment-service \
  --internal \
  --port 8083 \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/payments \
  --env SERVICE_PORT=8083
```

#### Step 3: Install Spring Gateway (Public)

```bash
doku install gateway \
  --path=./gateway \
  --port 8080 \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082 \
  --env PAYMENT_SERVICE_URL=http://payment-service:8083 \
  --env SPRING_PROFILES_ACTIVE=production
```

**Access:** `https://gateway.doku.local`

#### Step 4: Install Frontend (Public)

```bash
doku install frontend \
  --path=./frontend \
  --port 3000 \
  --env API_BASE_URL=https://gateway.doku.local \
  --env NODE_ENV=production
```

**Access:** `https://frontend.doku.local`

#### Step 5: Verify Installation

```bash
# List all services
doku list

# Check logs
doku logs gateway
doku logs user-service

# Test connectivity
curl https://gateway.doku.local/health
curl https://frontend.doku.local
```

#### Architecture Diagram

```
User Browser
     ↓
https://frontend.doku.local (Frontend Container)
     ↓
https://gateway.doku.local (Spring Gateway)
     ↓
doku-network (Docker Bridge Network)
     ├── user-service:8081 (Internal)
     ├── order-service:8082 (Internal)
     └── payment-service:8083 (Internal)
```

---

### Example 2: React Frontend + Node.js Backend

#### Install Backend (Internal)

```bash
doku install api \
  --path=./backend \
  --internal \
  --port 4000 \
  --env MONGODB_URL=mongodb://mongo:27017/myapp \
  --env JWT_SECRET=your_secret_key \
  --env NODE_ENV=production
```

#### Install Frontend (Public)

```bash
doku install frontend \
  --path=./frontend \
  --port 3000 \
  --env REACT_APP_API_URL=http://api:4000
```

#### Update Configuration Later

```bash
# Change API URL
doku env set frontend REACT_APP_API_URL=http://new-api:4000 --restart

# Add new environment variable
doku env set api REDIS_URL=redis://redis:6379 --restart
```

---

### Example 3: Python Flask API with Workers

#### Install Redis

```bash
doku install redis
```

#### Install Flask API (Public)

```bash
doku install flask-api \
  --path=./api \
  --port 5000 \
  --env REDIS_URL=redis://redis:6379 \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/flask_db
```

#### Install Background Workers (Internal)

```bash
doku install celery-worker \
  --path=./worker \
  --internal \
  --env REDIS_URL=redis://redis:6379 \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/flask_db
```

---

## Advanced Configuration

### Custom Dockerfile Path

If your Dockerfile is not in the root:

```bash
# This is handled by the --path flag pointing to the directory
# Doku looks for Dockerfile in the specified path
doku install myapp --path=./docker/myapp
```

If your Dockerfile has a different name, you can still use the existing `doku project add` command:

```bash
doku project add ./myapp --dockerfile docker/Dockerfile.prod
doku project build myapp
doku project run myapp
```

### Multiple Port Mappings

```bash
doku install myapp \
  --path=./myapp \
  --port 8080 \
  --port 9090 \
  --port 3000
```

### Volume Mounts

```bash
doku install myapp \
  --path=./myapp \
  --volume /host/data:/container/data \
  --volume /host/logs:/container/logs
```

### Memory and CPU Limits

```bash
doku install myapp \
  --path=./myapp \
  --memory 2g \
  --cpu 1.5
```

---

## Advanced Traefik Routing

For advanced routing scenarios (like routing `/api/*` to gateway and `/` to frontend), you can use Docker Compose with custom labels:

### Example: Path-Based Routing

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  # Spring Gateway - handles /api/* routes
  gateway:
    image: doku-project-gateway:latest
    container_name: doku-gateway
    networks:
      - doku-network
    environment:
      - USER_SERVICE_URL=http://user-service:8081
      - ORDER_SERVICE_URL=http://order-service:8082
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api-gateway.rule=Host(`app.doku.local`) && PathPrefix(`/api`)"
      - "traefik.http.routers.api-gateway.entrypoints=websecure"
      - "traefik.http.routers.api-gateway.tls=true"
      - "traefik.http.routers.api-gateway.priority=100"
      - "traefik.http.services.api-gateway.loadbalancer.server.port=8080"
      - "traefik.docker.network=doku-network"

  # Frontend - handles all other routes
  frontend:
    image: doku-project-frontend:latest
    container_name: doku-frontend
    networks:
      - doku-network
    environment:
      - API_BASE_URL=https://app.doku.local/api
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.frontend.rule=Host(`app.doku.local`)"
      - "traefik.http.routers.frontend.entrypoints=websecure"
      - "traefik.http.routers.frontend.tls=true"
      - "traefik.http.routers.frontend.priority=10"
      - "traefik.http.services.frontend.loadbalancer.server.port=80"
      - "traefik.docker.network=doku-network"

networks:
  doku-network:
    external: true
```

**Start the services:**
```bash
docker-compose up -d
```

**Result:**
- `https://app.doku.local/` → Frontend
- `https://app.doku.local/api/*` → Gateway → Microservices

---

## Troubleshooting

### Issue 1: Build Fails

**Problem:** Docker build fails during installation

**Solution:**
```bash
# Check Dockerfile syntax
cd ./your-project
docker build -t test:latest .

# Check Doku logs
doku logs your-service

# Verify Dockerfile exists
ls -la ./your-project/Dockerfile
```

### Issue 2: Service Not Accessible

**Problem:** Can't access service via URL

**Solutions:**

1. **Check if service is running:**
   ```bash
   doku list
   docker ps | grep doku-your-service
   ```

2. **Check DNS setup:**
   ```bash
   cat /etc/hosts | grep doku.local
   # Should show: 127.0.0.1 *.doku.local
   ```

3. **Check Traefik dashboard:**
   ```bash
   open https://traefik.doku.local
   # Verify your service appears in routers
   ```

4. **Check if service is internal:**
   ```bash
   doku info your-service
   # Internal services won't have a URL
   ```

### Issue 3: Environment Variables Not Applied

**Problem:** Changed env vars but service still uses old values

**Solution:**
```bash
# Restart the service
doku restart your-service

# Or set with auto-restart
doku env set your-service KEY=value --restart
```

### Issue 4: Container Fails to Start

**Problem:** Container stops immediately after starting

**Solutions:**

1. **Check logs:**
   ```bash
   doku logs your-service
   ```

2. **Check environment variables:**
   ```bash
   doku env your-service --show-values
   ```

3. **Verify dependencies:**
   ```bash
   # Make sure dependent services are running
   doku list
   ```

4. **Test the image manually:**
   ```bash
   docker run -it doku-project-your-service:latest /bin/sh
   ```

### Issue 5: Internal Service Can't Connect to Other Services

**Problem:** Service can't reach other containers

**Solutions:**

1. **Verify network:**
   ```bash
   docker network inspect doku-network
   # Verify both containers are in the network
   ```

2. **Test connectivity:**
   ```bash
   docker exec -it doku-your-service curl http://other-service:port
   ```

3. **Check service names:**
   ```bash
   # Use container name, not URL
   # ✓ Correct: http://postgres:5432
   # ✗ Wrong: https://postgres.doku.local:5432
   ```

### Issue 6: Port Already in Use

**Problem:** Port conflict during installation

**Solution:**
```bash
# Use custom port mapping
doku install myapp --path=./myapp --port 8081:8080

# Or stop the conflicting service
lsof -i :8080
kill -9 <PID>
```

---

## Command Reference

### `doku install` with `--path`

Install a custom project from a Dockerfile.

**Syntax:**
```bash
doku install <name> --path=<directory> [options]
```

**Options:**
- `--path string`: Path to project directory with Dockerfile (required for custom projects)
- `--name string`: Custom instance name (default: service name)
- `--port strings`: Port mappings (format: `host:container` or `port`)
- `--env strings`: Environment variables (format: `KEY=VALUE`)
- `--internal`: Install as internal service (no Traefik exposure)
- `--volume strings`: Volume mounts (format: `host:container`)
- `--memory string`: Memory limit (e.g., `512m`, `1g`)
- `--cpu string`: CPU limit (e.g., `0.5`, `1.0`)
- `--yes`: Skip confirmation prompts

**Examples:**
```bash
# Basic installation
doku install frontend --path=./frontend

# With all options
doku install api --path=./api \
  --name api-v2 \
  --port 8080 \
  --env DATABASE_URL=postgres://... \
  --env API_KEY=secret \
  --internal \
  --memory 1g \
  --cpu 0.5 \
  --yes
```

### `doku env set`

Set or update environment variables for a service.

**Syntax:**
```bash
doku env set <service> <KEY=VALUE> [KEY2=VALUE2...] [options]
```

**Options:**
- `--restart`: Automatically restart service after setting variables

**Examples:**
```bash
# Single variable
doku env set frontend API_KEY=new_value

# Multiple variables
doku env set frontend API_KEY=secret DEBUG=true NODE_ENV=prod

# With auto-restart
doku env set frontend API_KEY=secret --restart
```

### `doku env unset`

Remove environment variables from a service.

**Syntax:**
```bash
doku env unset <service> <KEY> [KEY2...] [options]
```

**Options:**
- `--restart`: Automatically restart service after unsetting variables

**Examples:**
```bash
# Single variable
doku env unset frontend OLD_API_KEY

# Multiple variables
doku env unset frontend OLD_KEY DEPRECATED_VAR

# With auto-restart
doku env unset frontend OLD_KEY --restart
```

### `doku env` (view)

Display environment variables for a service.

**Syntax:**
```bash
doku env <service> [options]
```

**Options:**
- `--show-values`: Show actual values (unmask sensitive data)
- `--export`: Output in shell export format

**Examples:**
```bash
# View variables (masked)
doku env frontend

# View actual values
doku env frontend --show-values

# Export for shell
doku env frontend --export > .env
source .env
```

### Other Useful Commands

```bash
# List all services
doku list

# View service details
doku info frontend

# View logs
doku logs frontend
doku logs frontend -f  # Follow logs

# Start/Stop/Restart
doku start frontend
doku stop frontend
doku restart frontend

# Remove service
doku remove frontend
```

---

## Best Practices

### 1. Environment Variable Management

- ✅ **Do:** Use environment variables for configuration
- ✅ **Do:** Use `--restart` flag when changing critical variables
- ❌ **Don't:** Hardcode secrets in Dockerfile
- ❌ **Don't:** Commit `.env` files with secrets

### 2. Service Architecture

- ✅ **Do:** Use `--internal` for backend microservices
- ✅ **Do:** Expose only necessary services publicly
- ✅ **Do:** Use API Gateway pattern for microservices
- ❌ **Don't:** Expose databases or internal services publicly

### 3. Naming Conventions

- ✅ **Do:** Use descriptive names (`user-service`, `payment-api`)
- ✅ **Do:** Use lowercase with hyphens
- ❌ **Don't:** Use special characters or spaces
- ❌ **Don't:** Use generic names (`app1`, `service`)

### 4. Docker Best Practices

- ✅ **Do:** Use multi-stage builds for smaller images
- ✅ **Do:** Use `.dockerignore` to exclude unnecessary files
- ✅ **Do:** Use health checks in your Dockerfile
- ❌ **Don't:** Run as root in containers
- ❌ **Don't:** Install unnecessary packages

### 5. Development Workflow

```bash
# Development cycle
1. doku install myapp --path=./myapp --env NODE_ENV=development
2. Make code changes
3. doku remove myapp
4. doku install myapp --path=./myapp --env NODE_ENV=development
5. Test changes
```

Or use the project commands for more control:

```bash
# Alternative development cycle
1. doku project add ./myapp
2. Make code changes
3. doku project build myapp
4. doku project run myapp
```

---

## Migration Guide

### From `doku project` to `doku install --path`

**Old way:**
```bash
doku project add ./frontend
doku project build frontend
doku project run frontend
```

**New way:**
```bash
doku install frontend --path=./frontend
```

**Benefits:**
- Single command instead of three
- Consistent with catalog installations
- Automatic build and run
- Simpler workflow

**Note:** The `doku project` commands still work and are useful for advanced scenarios where you need more control over the build process.

---

## FAQ

### Q: Can I use both catalog services and custom projects?

**A:** Yes! They work seamlessly together:
```bash
doku install postgres  # From catalog
doku install myapp --path=./myapp  # Custom project
```

### Q: Can custom projects depend on catalog services?

**A:** Yes! Reference them by name:
```bash
doku install postgres
doku install myapp --path=./myapp \
  --env DATABASE_URL=postgresql://postgres@postgres:5432/mydb
```

### Q: How do I update a custom project?

**A:** Remove and reinstall:
```bash
doku remove frontend
doku install frontend --path=./frontend
```

### Q: Can I use environment variables from files?

**A:** Yes, you can source them:
```bash
# Create .env file
cat > .env <<EOF
API_KEY=secret
DATABASE_URL=postgres://...
EOF

# Read and pass to install
source .env
doku install myapp --path=./myapp \
  --env API_KEY=$API_KEY \
  --env DATABASE_URL=$DATABASE_URL
```

### Q: What's the difference between internal and public services?

**A:**
- **Public:** Accessible via HTTPS URL (e.g., `https://frontend.doku.local`)
- **Internal:** Only accessible within Docker network (e.g., `http://service:port`)

### Q: Can I change a service from internal to public?

**A:** You need to remove and reinstall without the `--internal` flag:
```bash
doku remove myservice
doku install myservice --path=./myservice  # Without --internal
```

### Q: Do environment variable changes persist after restart?

**A:** Yes! Changes are saved to Doku's configuration and persist across:
- Service restarts
- System reboots
- Doku updates

### Q: Can I see environment variables for catalog services?

**A:** Yes! All `doku env` commands work with both catalog and custom services:
```bash
doku env postgres --show-values
doku env set postgres POSTGRES_PASSWORD=newpass --restart
```

---

## Additional Resources

- [Doku CLI Documentation](https://github.com/dokulabs/doku-cli)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Traefik Documentation](https://doc.traefik.io/traefik/)
- [Dockerfile Reference](https://docs.docker.com/engine/reference/builder/)

---

## Support

If you encounter issues:

1. Check the [Troubleshooting](#troubleshooting) section
2. View service logs: `doku logs <service>`
3. Check Traefik dashboard: `https://traefik.doku.local`
4. Open an issue: [GitHub Issues](https://github.com/dokulabs/doku-cli/issues)

---

**Last Updated:** 2025-11-18
**Version:** 1.0.0
**Compatible with:** Doku CLI v0.1.0+
