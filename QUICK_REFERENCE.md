# Doku CLI - Quick Reference Guide

## Custom Project Installation

### Basic Installation
```bash
# Public service
doku install frontend --path=./frontend

# Internal service (no external access)
doku install user-service --path=./user-service --internal

# With port mapping
doku install api --path=./api --port 8080

# With environment variables
doku install app --path=./app \
  --env DATABASE_URL=postgres://... \
  --env API_KEY=secret

# Skip confirmation
doku install app --path=./app --yes
```

---

## Environment Variable Management

### View Variables
```bash
# View (masked)
doku env frontend

# View actual values
doku env frontend --show-values

# Export for shell
doku env frontend --export > .env
```

### Set Variables
```bash
# Single variable
doku env set frontend API_KEY=secret

# Multiple variables
doku env set frontend API_KEY=secret DEBUG=true

# Set and auto-restart
doku env set frontend API_KEY=secret --restart
```

### Remove Variables
```bash
# Single variable
doku env unset frontend OLD_KEY

# Multiple variables
doku env unset frontend OLD_KEY DEPRECATED_VAR

# Remove and auto-restart
doku env unset frontend OLD_KEY --restart
```

---

## Complete Microservices Example

```bash
# 1. Install dependencies
doku install postgres:16

# 2. Install backend services (internal)
doku install user-service --path=./user-service --internal --port 8081
doku install order-service --path=./order-service --internal --port 8082
doku install payment-service --path=./payment-service --internal --port 8083

# 3. Install API gateway (public)
doku install gateway --path=./gateway \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082 \
  --env PAYMENT_SERVICE_URL=http://payment-service:8083 \
  --port 8080

# 4. Install frontend (public)
doku install frontend --path=./frontend \
  --env API_BASE_URL=https://gateway.doku.local \
  --port 3000

# 5. Verify
doku list
```

---

## Service Management

```bash
# List all services
doku list

# View service info
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

## Common Patterns

### React + Node.js Backend
```bash
# Backend (internal)
doku install api --path=./backend --internal --port 4000 \
  --env MONGODB_URL=mongodb://mongo:27017/myapp

# Frontend (public)
doku install frontend --path=./frontend --port 3000 \
  --env REACT_APP_API_URL=http://api:4000
```

### Python Flask + Celery
```bash
# Dependencies
doku install redis
doku install postgres

# Flask API (public)
doku install flask-api --path=./api --port 5000 \
  --env REDIS_URL=redis://redis:6379

# Celery Worker (internal)
doku install celery-worker --path=./worker --internal \
  --env REDIS_URL=redis://redis:6379
```

---

## Troubleshooting

```bash
# Check if service is running
doku list
docker ps | grep doku

# View logs
doku logs service-name

# Check Traefik dashboard
open https://traefik.doku.local

# Verify network
docker network inspect doku-network

# Test internal connectivity
docker exec -it doku-service curl http://other-service:port

# Rebuild and restart
doku remove service-name
doku install service-name --path=./service-name
```

---

## Flags Reference

### `doku install --path`
```
--path string          Path to project with Dockerfile
--name string          Custom instance name
--port strings         Port mappings (8080 or 8081:8080)
--env strings          Environment variables (KEY=VALUE)
--internal             Install as internal service
--volume strings       Volume mounts (host:container)
--memory string        Memory limit (512m, 1g)
--cpu string           CPU limit (0.5, 1.0)
--yes                  Skip confirmation
```

### `doku env set`
```
--restart              Restart service after setting
```

### `doku env unset`
```
--restart              Restart service after unsetting
```

### `doku env` (view)
```
--show-values          Show actual values
--export               Export format
```

---

## URLs

```bash
# Traefik Dashboard
https://traefik.doku.local

# Public Services
https://service-name.doku.local

# Internal Services
http://service-name:port
```

---

## Best Practices

✅ **Do:**
- Use `--internal` for backend services
- Use environment variables for config
- Use descriptive service names
- Restart services after env changes

❌ **Don't:**
- Expose internal services publicly
- Hardcode secrets in Dockerfile
- Use special characters in names
- Run containers as root

---

## Quick Commands

```bash
# Initialize Doku
doku init

# Install from catalog
doku install postgres

# Install custom project
doku install myapp --path=./myapp

# Update environment
doku env set myapp KEY=value --restart

# Check status
doku list

# View logs
doku logs myapp -f

# Restart service
doku restart myapp

# Remove service
doku remove myapp
```
