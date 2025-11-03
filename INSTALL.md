# Doku CLI Installation Guide

There are multiple ways to install Doku CLI on your system.

## Quick Install (Recommended)

### Using curl

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

### Using wget

```bash
wget -qO- https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

This will automatically:
- Detect your OS and architecture
- Download the latest pre-built binary from GitHub releases
- Install to an appropriate location (`/usr/local/bin` or `~/go/bin`)
- Make the binary executable

## Install Specific Version

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --version v1.2.3
```

Or download and run locally:

```bash
wget https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh
chmod +x install.sh
./install.sh --version v1.2.3
```

## Install from Source

### Option 1: Using the install script

```bash
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli
./scripts/install.sh --source
```

### Option 2: Using Go install

Install latest release:
```bash
go install github.com/dokulabs/doku-cli/cmd/doku@latest
```

Install from main branch:
```bash
go install github.com/dokulabs/doku-cli/cmd/doku@main
```

Install specific version:
```bash
go install github.com/dokulabs/doku-cli/cmd/doku@v1.2.3
```

### Option 3: Manual build

```bash
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli
make build
sudo cp bin/doku /usr/local/bin/
```

## Custom Installation Directory

Install to a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --dir /path/to/dir
```

## Verify Installation

After installation, verify it works:

```bash
doku version
```

You should see output like:
```
Doku CLI v1.2.3
Commit: abc1234
Built: 2025-11-03T10:00:00Z
```

## Platform Support

Doku CLI provides pre-built binaries for:

| OS      | Architecture | Status |
|---------|-------------|--------|
| Linux   | amd64       | ✅     |
| Linux   | arm64       | ✅     |
| macOS   | amd64       | ✅     |
| macOS   | arm64 (M1+) | ✅     |
| Windows | amd64       | ✅     |

## Troubleshooting

### Command not found after installation

The installation directory may not be in your `PATH`. Add it:

**Bash:**
```bash
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.bashrc
source ~/.bashrc
```

**Zsh:**
```bash
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc
source ~/.zshrc
```

**Fish:**
```fish
fish_add_path $HOME/go/bin
```

### Permission denied when installing

If you get permission errors:

1. Try with sudo:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | sudo bash
   ```

2. Or install to a user directory:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --dir ~/.local/bin
   ```

### Binary not available for your platform

If pre-built binaries aren't available, build from source:

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash -s -- --source
```

This requires Go 1.23+ to be installed.

## Updating

To update to the latest version, simply run the install command again:

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

## Uninstalling

To completely remove Doku CLI:

```bash
doku uninstall --all
```

This will remove:
- All Docker containers
- All Docker volumes
- Docker network
- Configuration directory (`~/.doku/`)
- The doku binary (with instructions)

Then manually remove the binary:

**If installed to `/usr/local/bin`:**
```bash
sudo rm /usr/local/bin/doku
```

**If installed to `~/go/bin`:**
```bash
rm ~/go/bin/doku
```

## Next Steps

After installation:

1. Initialize Doku:
   ```bash
   doku init
   ```

2. View available services:
   ```bash
   doku catalog
   ```

3. Install a service:
   ```bash
   doku install postgres
   ```

For more information, see the [main README](README.md).
