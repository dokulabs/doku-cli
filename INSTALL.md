# Doku CLI Installation Guide

Install Doku CLI easily on macOS, Linux, or Windows.

## Quick Install (Recommended)

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

This will:
- ✅ Automatically detect your OS and architecture
- ✅ Download the latest release
- ✅ Install to `/usr/local/bin` (or appropriate location)
- ✅ Make `doku` command available globally

---

## Installation Methods

### Method 1: Download from GitHub Releases (Recommended)

#### macOS

```bash
# Intel Mac (amd64)
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-amd64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/

# Apple Silicon (arm64/M1/M2/M3)
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-arm64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/
```

#### Linux

```bash
# x86_64/amd64
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-linux-amd64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/

# arm64/aarch64
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-linux-arm64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/
```

#### Windows

Download from [GitHub Releases](https://github.com/dokulabs/doku-cli/releases/latest):
- `doku-windows-amd64.exe`

Rename to `doku.exe` and add to your PATH.

### Method 2: Build from Source

Requirements:
- Go 1.21 or higher
- Git

```bash
# Clone the repository
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli

# Build and install
make build
sudo mv bin/doku /usr/local/bin/

# Or use Go install directly
go install github.com/dokulabs/doku-cli/cmd/doku@latest
```

### Method 3: Using the Install Script

Download and run the installer:

```bash
# Install latest version
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash

# Install specific version
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --version v0.2.0

# Build from source
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --source

# Install to custom directory
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --dir $HOME/.local/bin
```

---

## Verify Installation

After installation, verify that Doku is working:

```bash
# Check version
doku version

# Get help
doku --help

# Check if doku is in PATH
which doku
```

Expected output:
```
Doku CLI vX.X.X
Commit: abc1234
Build Date: 2025-11-18T12:00:00Z
```

---

## Post-Installation

### 1. Initialize Doku

```bash
doku init
```

This will:
- Set up Docker networking
- Configure Traefik reverse proxy
- Set up SSL certificates
- Download the service catalog

### 2. Install Your First Service

```bash
# From catalog
doku install postgres

# Custom project
doku install myapp --path=./myapp
```

### 3. Access Services

```bash
# List all services
doku list

# Access service
open https://postgres.doku.local
```

---

## Troubleshooting

### "doku: command not found"

The installation directory is not in your PATH.

**Solution:**

Add the install directory to your PATH:

```bash
# For bash (~/.bashrc)
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.bashrc
source ~/.bashrc

# For zsh (~/.zshrc)
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.zshrc
source ~/.zshrc

# For fish (~/.config/fish/config.fish)
echo 'set -gx PATH $PATH /usr/local/bin' >> ~/.config/fish/config.fish
source ~/.config/fish/config.fish
```

### Permission Denied

**Solution:**

```bash
# Make the binary executable
chmod +x /path/to/doku

# Or install with sudo
sudo mv doku /usr/local/bin/
```

### "Cannot download binary"

If the automated download fails:

**Solution 1: Try building from source**
```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --source
```

**Solution 2: Manual download**
1. Go to [GitHub Releases](https://github.com/dokulabs/doku-cli/releases/latest)
2. Download the appropriate binary for your system
3. Extract and install manually

### macOS "cannot be opened because the developer cannot be verified"

**Solution:**
```bash
# Remove quarantine attribute
xattr -d com.apple.quarantine /usr/local/bin/doku

# Or allow in System Preferences
# System Preferences → Security & Privacy → General → Click "Allow Anyway"
```

---

## Updating Doku

### Update to Latest Version

```bash
# Method 1: Re-run install script
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash

# Method 2: Download manually
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') -o doku
chmod +x doku
sudo mv doku /usr/local/bin/

# Method 3: Use upgrade command (if available)
doku upgrade
```

### Check for Updates

```bash
# Check current version
doku version

# View latest release
curl -s https://api.github.com/repos/dokulabs/doku-cli/releases/latest | grep '"tag_name"'
```

---

## Uninstalling Doku

### Remove Doku CLI

```bash
# Remove binary
sudo rm /usr/local/bin/doku

# Remove Doku data and configuration
doku uninstall --purge

# Or manually
rm -rf ~/.doku
```

### Stop All Services

Before uninstalling:

```bash
# List all services
doku list

# Remove all services
doku remove --all

# Stop Traefik
docker stop doku-traefik
docker rm doku-traefik

# Remove Docker network
docker network rm doku-network
```

---

## Platform-Specific Notes

### macOS

- **Homebrew support** (coming soon):
  ```bash
  brew install dokulabs/tap/doku
  ```

- **Apple Silicon (M1/M2/M3)**: Use the arm64 binary
- **Intel Mac**: Use the amd64 binary

### Linux

- **Ubuntu/Debian**:
  ```bash
  sudo apt-get install curl
  curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
  ```

- **Fedora/RHEL/CentOS**:
  ```bash
  sudo dnf install curl
  curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
  ```

- **Arch Linux** (coming soon):
  ```bash
  yay -S doku-cli
  ```

### Windows

- **PowerShell**:
  ```powershell
  # Download
  Invoke-WebRequest -Uri "https://github.com/dokulabs/doku-cli/releases/latest/download/doku-windows-amd64.exe" -OutFile "doku.exe"

  # Move to a directory in PATH (e.g., C:\Program Files\Doku\)
  Move-Item doku.exe "C:\Program Files\Doku\doku.exe"

  # Add to PATH
  [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\Doku", "Machine")
  ```

- **WSL2 (Recommended)**: Install the Linux version inside WSL2

---

## Docker Requirements

Doku requires Docker to be installed and running:

### Install Docker

#### macOS
Download [Docker Desktop for Mac](https://www.docker.com/products/docker-desktop/)

#### Linux
```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com | bash

# Start Docker
sudo systemctl start docker
sudo systemctl enable docker

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

#### Windows
Download [Docker Desktop for Windows](https://www.docker.com/products/docker-desktop/)

### Verify Docker

```bash
docker --version
docker ps
```

---

## Next Steps

After installation:

1. **Read the documentation**:
   - [Getting Started Guide](./README.md)
   - [Custom Projects Guide](./CUSTOM_PROJECTS_GUIDE.md)
   - [Quick Reference](./QUICK_REFERENCE.md)

2. **Initialize Doku**:
   ```bash
   doku init
   ```

3. **Install your first service**:
   ```bash
   doku install postgres
   ```

4. **Explore available services**:
   ```bash
   doku catalog list
   ```

---

## Getting Help

- 📚 [Documentation](https://github.com/dokulabs/doku-cli)
- 🐛 [Report Issues](https://github.com/dokulabs/doku-cli/issues)
- 💬 [Discussions](https://github.com/dokulabs/doku-cli/discussions)
- 📖 [Wiki](https://github.com/dokulabs/doku-cli/wiki)

---

## System Requirements

- **Operating System**: macOS 10.15+, Linux (Ubuntu 20.04+, Debian 10+, etc.), Windows 10+
- **Architecture**: x86_64 (amd64) or ARM64 (aarch64)
- **Docker**: 20.10+ or Docker Desktop
- **Disk Space**: 1GB minimum for Doku and basic services
- **RAM**: 4GB minimum (8GB recommended)
- **Network**: Internet connection for downloading catalog and services

---

**Last Updated**: 2025-11-18
**Doku Version**: v0.1.0+
