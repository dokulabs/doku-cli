# Installation Section for README.md

## 📦 Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

### Download Binary

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS | Intel (amd64) | [Download](https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-amd64) |
| macOS | Apple Silicon (arm64) | [Download](https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-arm64) |
| Linux | amd64 | [Download](https://github.com/dokulabs/doku-cli/releases/latest/download/doku-linux-amd64) |
| Linux | arm64 | [Download](https://github.com/dokulabs/doku-cli/releases/latest/download/doku-linux-arm64) |
| Windows | amd64 | [Download](https://github.com/dokulabs/doku-cli/releases/latest/download/doku-windows-amd64.exe) |

After downloading:

```bash
# Make executable
chmod +x doku

# Move to PATH
sudo mv doku /usr/local/bin/

# Verify
doku version
```

### Build from Source

```bash
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli
make build
sudo mv bin/doku /usr/local/bin/
```

**Full installation guide**: [INSTALL.md](./INSTALL.md)

---

## 🚀 Quick Start

### 1. Initialize Doku

```bash
doku init
```

### 2. Install a Service

```bash
# From catalog
doku install postgres

# Custom project
doku install myapp --path=./myapp
```

### 3. Access Your Service

```bash
doku list
open https://postgres.doku.local
```

---

## 📚 Documentation

- [Installation Guide](./INSTALL.md) - Complete installation instructions
- [Custom Projects Guide](./CUSTOM_PROJECTS_GUIDE.md) - Build and deploy your own applications
- [Quick Reference](./QUICK_REFERENCE.md) - Command cheat sheet
- [New Features](./NEW_FEATURES.md) - Latest feature announcements

---

## ⚡ Features

- **🚀 One-Command Install**: Install services from catalog or custom projects
- **🔧 Dynamic Configuration**: Update environment variables without rebuilding
- **🌐 Automatic HTTPS**: Clean URLs with SSL certificates
- **🔒 Internal Services**: Mark backend services as internal-only
- **📦 Service Catalog**: 20+ pre-configured services (Postgres, Redis, MongoDB, etc.)
- **🛠 Custom Projects**: Build and run from Dockerfiles with `--path` flag
- **🔄 Service Discovery**: Automatic DNS and networking
- **📊 Traefik Dashboard**: Monitor and manage routing

---

## 💡 Examples

### Install Database

```bash
doku install postgres:16
doku install redis
doku install mongodb
```

### Full-Stack Application

```bash
# Backend (internal)
doku install api --path=./backend --internal --port 4000

# Frontend (public)
doku install frontend --path=./frontend --port 3000 \
  --env API_URL=http://api:4000
```

### Microservices with Gateway

```bash
# Microservices (internal)
doku install user-service --path=./user-service --internal --port 8081
doku install order-service --path=./order-service --internal --port 8082

# API Gateway (public)
doku install gateway --path=./gateway --port 8080 \
  --env USER_SERVICE_URL=http://user-service:8081 \
  --env ORDER_SERVICE_URL=http://order-service:8082
```

### Manage Environment

```bash
# Set variables
doku env set frontend API_KEY=secret DEBUG=true

# Auto-restart after changes
doku env set frontend API_KEY=secret --restart

# Remove variables
doku env unset frontend OLD_KEY
```

---

## 🔧 Requirements

- **Docker** 20.10+ or Docker Desktop
- **OS**: macOS 10.15+, Linux (Ubuntu 20.04+), Windows 10+
- **Architecture**: x86_64 (amd64) or ARM64

---

## 📖 Usage

```bash
# Initialize
doku init

# Catalog services
doku install <service>[:<version>]
doku install postgres
doku install redis:7

# Custom projects
doku install <name> --path=<directory>
doku install myapp --path=./myapp

# Manage services
doku list
doku logs <service>
doku restart <service>
doku stop <service>
doku remove <service>

# Environment variables
doku env <service>
doku env set <service> KEY=VALUE
doku env unset <service> KEY

# Get help
doku --help
doku install --help
```

---

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

---

## 📄 License

[MIT License](./LICENSE)

---

## 🙏 Support

- ⭐ Star this repository
- 🐛 [Report Issues](https://github.com/dokulabs/doku-cli/issues)
- 💬 [Join Discussions](https://github.com/dokulabs/doku-cli/discussions)
- 📧 Contact: [email protected]
