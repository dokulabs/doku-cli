# Doku CLI Installation Guide

## ✅ Problem Solved!

When you use `go install github.com/dokulabs/doku-cli/cmd/doku@latest`, it installs the binary but doesn't include version information because Go doesn't apply the ldflags from the Makefile.

**Solution:** Use our custom installation script that properly embeds version information.

---

## 📦 Installation Methods

### Method 1: Install Script (Recommended) ✅

This method properly embeds version, commit, and build date information.

```bash
# Clone the repository (if not already cloned)
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli

# Run the installation script
./scripts/install.sh

# Verify installation
doku version
```

**Output:**
```
Doku CLI
  Version:    v0.1.0+15
  Commit:     12fee2b
  Build Date: 2025-11-03T05:31:01Z
```

---

### Method 2: Make Install

```bash
cd doku-cli
make install

# Verify
doku version
```

---

### Method 3: Local Build Only

If you just want to build and test locally without installing:

```bash
# Option A: Using install script
./scripts/install.sh --local
./bin/doku version

# Option B: Using Makefile
make build
./bin/doku version
```

---

## 🔧 Installation Script Features

The `scripts/install.sh` script:

✅ **Auto-detects version** from git tags
✅ **Embeds commit hash** for traceability
✅ **Sets build date** automatically
✅ **Checks prerequisites** (Go, Git)
✅ **Validates PATH** and provides setup instructions
✅ **Supports local builds** with `--local` flag

---

## 📍 Installation Locations

### macOS/Linux:
```bash
# Installed to:
$GOPATH/bin/doku
# Usually: ~/go/bin/doku

# Or check with:
which doku
```

### Windows:
```powershell
# Installed to:
%GOPATH%\bin\doku.exe
# Usually: C:\Users\<YourName>\go\bin\doku.exe
```

---

## ⚙️ Ensure doku is in PATH

If `doku version` doesn't work after installation:

### macOS/Linux (zsh):
```bash
# Add to ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"

# Reload shell
source ~/.zshrc

# Test
doku version
```

### macOS/Linux (bash):
```bash
# Add to ~/.bashrc or ~/.bash_profile
export PATH="$PATH:$(go env GOPATH)/bin"

# Reload shell
source ~/.bashrc

# Test
doku version
```

### Windows:
1. Open "Environment Variables" settings
2. Add `%GOPATH%\bin` to your PATH
3. Restart terminal
4. Test: `doku version`

---

## 🚀 Quick Start After Installation

```bash
# Check version
doku version

# See all commands
doku --help

# Initialize doku
doku init

# Try the new config command
doku config --help
doku config list

# Install a service
doku install postgres

# View monitoring dashboard
doku monitor
```

---

## 🔄 Updating Doku

### From Repository:
```bash
cd doku-cli
git pull
./scripts/install.sh
doku version  # Verify new version
```

### From Source:
```bash
cd doku-cli
make clean
make install
doku version
```

---

## 🐛 Troubleshooting

### Issue: "command not found: doku"

**Solution:**
```bash
# Check if installed
ls $(go env GOPATH)/bin/doku

# If it exists, add to PATH
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

### Issue: Version shows "dev", "none", "unknown"

**This happens when using:**
```bash
go install github.com/dokulabs/doku-cli/cmd/doku@latest  # ❌ No version info
```

**Solution: Use the installation script instead:**
```bash
./scripts/install.sh  # ✅ Includes version info
```

---

### Issue: "Permission denied"

**Solution:**
```bash
# Make script executable
chmod +x scripts/install.sh

# Run again
./scripts/install.sh
```

---

## 📊 Version Information Explained

```
Doku CLI
  Version:    v0.1.0+15      ← Tag + commits ahead
  Commit:     12fee2b        ← Git commit hash
  Build Date: 2025-11-03...  ← When it was built
```

**Version Format:**
- `v0.1.0` - Latest git tag
- `+15` - Number of commits ahead of that tag
- If you're exactly on a tag: just `v0.1.0`
- If no tags exist: `v0.1.0-dev`

---

## 🎯 Development Workflow

### For Contributors:

```bash
# 1. Clone and setup
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli

# 2. Make changes
# ... edit code ...

# 3. Build locally
./scripts/install.sh --local
./bin/doku version

# 4. Test
./bin/doku init
./bin/doku config --help

# 5. Install for system-wide testing
./scripts/install.sh
doku version

# 6. Verify everything works
doku init
```

---

## 📝 For End Users

If you're installing from a release:

```bash
# Download release binary for your OS
# From: https://github.com/dokulabs/doku-cli/releases

# macOS/Linux
chmod +x doku
sudo mv doku /usr/local/bin/
doku version

# Windows
# Move doku.exe to a directory in your PATH
doku.exe version
```

---

## ✅ Installation Complete!

You should now have doku installed with proper version information:

```bash
$ doku version
Doku CLI
  Version:    v0.1.0+15
  Commit:     12fee2b
  Build Date: 2025-11-03T05:31:01Z
```

**Ready to start?**
```bash
doku init
```

---

## 🔗 Additional Resources

- **Documentation:** [docs.doku.dev](https://docs.doku.dev)
- **GitHub:** [github.com/dokulabs/doku-cli](https://github.com/dokulabs/doku-cli)
- **Issues:** [github.com/dokulabs/doku-cli/issues](https://github.com/dokulabs/doku-cli/issues)

---

## 📧 Need Help?

If you encounter any issues:

1. Check the troubleshooting section above
2. Run `doku version` to verify installation
3. Run `doku --help` to see all commands
4. Check [GitHub Issues](https://github.com/dokulabs/doku-cli/issues)
5. Create a new issue with your problem

---

**Happy coding! 🚀**
