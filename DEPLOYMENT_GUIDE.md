# Doku CLI - Deployment & Release Guide

This guide explains how to create releases and deploy Doku CLI so users can install it with `doku` command globally.

## Overview

Doku uses GitHub Actions for automated builds and releases. Every push to `main` triggers a build, and every tag triggers a release with binaries for all platforms.

---

## Release Process

### 1. Create a New Release

#### Option A: Using Git Tags (Recommended)

```bash
# Make sure you're on main branch with latest changes
git checkout main
git pull origin main

# Create a new tag
git tag v0.2.0

# Push the tag to GitHub
git push origin v0.2.0
```

This will automatically:
- ✅ Trigger the GitHub Actions release workflow
- ✅ Build binaries for all platforms (macOS, Linux, Windows)
- ✅ Create a GitHub Release with binaries attached
- ✅ Generate changelog from commits
- ✅ Create checksums for verification

#### Option B: Using GitHub Web Interface

1. Go to https://github.com/dokulabs/doku-cli/releases
2. Click "Draft a new release"
3. Click "Choose a tag" and create a new tag (e.g., `v0.2.0`)
4. Fill in release title and description
5. Click "Publish release"

GitHub Actions will automatically build and attach the binaries.

### 2. Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **Major** (v1.0.0 → v2.0.0): Breaking changes
- **Minor** (v0.1.0 → v0.2.0): New features, backwards compatible
- **Patch** (v0.1.0 → v0.1.1): Bug fixes

Examples:
```bash
v0.1.0  # First release
v0.1.1  # Bug fix
v0.2.0  # New features (--path flag, env commands)
v1.0.0  # Stable release
```

---

## GitHub Actions Workflows

### Build Workflow (`.github/workflows/build.yml`)

Triggers on:
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop`

Actions:
- Run tests
- Build for Linux
- Upload artifacts (7-day retention)
- Verify code quality (go vet, gofmt)

### Release Workflow (`.github/workflows/release.yml`)

Triggers on:
- Push of tags matching `v*` (e.g., `v0.1.0`, `v1.2.3`)

Actions:
- Build for all platforms:
  - `doku-darwin-amd64` (macOS Intel)
  - `doku-darwin-arm64` (macOS Apple Silicon)
  - `doku-linux-amd64` (Linux x86_64)
  - `doku-linux-arm64` (Linux ARM64)
  - `doku-windows-amd64.exe` (Windows)
- Generate SHA256 checksums
- Create GitHub Release
- Attach binaries to release
- Generate changelog

---

## Installation Methods for Users

Once a release is created, users can install Doku in multiple ways:

### 1. One-Line Install (Easiest)

```bash
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
```

### 2. Direct Download

```bash
# macOS (Intel)
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-amd64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-darwin-arm64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/

# Linux
curl -L https://github.com/dokulabs/doku-cli/releases/latest/download/doku-linux-amd64 -o doku
chmod +x doku
sudo mv doku /usr/local/bin/
```

### 3. Build from Source

```bash
git clone https://github.com/dokulabs/doku-cli.git
cd doku-cli
make build
sudo mv bin/doku /usr/local/bin/
```

---

## Testing Releases

### Test Locally Before Release

```bash
# Build for your platform
make build

# Test the binary
./bin/doku version
./bin/doku --help

# Test installation
./bin/doku init
./bin/doku install postgres
```

### Test After Release

```bash
# Test the install script
curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash

# Verify installation
doku version
doku --help
```

---

## Release Checklist

Before creating a release:

- [ ] All tests passing (`make test`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Version bumped in appropriate places
- [ ] CHANGELOG.md updated
- [ ] Documentation updated
- [ ] New features documented
- [ ] README.md updated if needed
- [ ] All PRs merged to main
- [ ] Local testing completed

Create the release:

- [ ] Tag created and pushed
- [ ] GitHub Actions workflow completed successfully
- [ ] Binaries attached to release
- [ ] Release notes added
- [ ] Installation tested on macOS, Linux, and Windows

Post-release:

- [ ] Announce on social media
- [ ] Update documentation website
- [ ] Notify users in Discord/Slack
- [ ] Create a blog post (optional)

---

## Hotfix Process

For urgent bug fixes:

1. Create hotfix branch:
   ```bash
   git checkout -b hotfix/v0.1.1 v0.1.0
   ```

2. Make fixes and commit

3. Test thoroughly

4. Create new tag:
   ```bash
   git tag v0.1.1
   git push origin v0.1.1
   ```

5. Merge back to main:
   ```bash
   git checkout main
   git merge hotfix/v0.1.1
   git push origin main
   ```

---

## Troubleshooting

### GitHub Actions Failing

1. Check workflow logs in GitHub Actions tab
2. Common issues:
   - Go version mismatch
   - Test failures
   - Build errors
   - Permission issues

### Binary Not Downloadable

1. Check if release is published (not draft)
2. Verify binary names match expected format
3. Check GitHub Release assets are attached

### Install Script Failing

1. Test locally:
   ```bash
   ./scripts/install.sh
   ```

2. Check for:
   - Correct download URLs
   - Platform detection working
   - File permissions correct

---

## Continuous Deployment (Future)

### Homebrew Tap (macOS)

Create a Homebrew formula:

```ruby
# Formula/doku.rb
class Doku < Formula
  desc "Local development environment manager"
  homepage "https://github.com/dokulabs/doku-cli"
  url "https://github.com/dokulabs/doku-cli/releases/download/v0.1.0/doku-darwin-amd64"
  sha256 "..."
  version "0.1.0"

  def install
    bin.install "doku-darwin-amd64" => "doku"
  end

  test do
    system "#{bin}/doku", "version"
  end
end
```

Install:
```bash
brew install dokulabs/tap/doku
```

### APT Repository (Debian/Ubuntu)

Create a Debian package:

```bash
# Build .deb package
mkdir -p doku_0.1.0_amd64/DEBIAN
mkdir -p doku_0.1.0_amd64/usr/local/bin

# Copy binary
cp bin/doku doku_0.1.0_amd64/usr/local/bin/

# Create control file
cat > doku_0.1.0_amd64/DEBIAN/control <<EOF
Package: doku
Version: 0.1.0
Architecture: amd64
Maintainer: Doku Labs <[email protected]>
Description: Local development environment manager
EOF

# Build package
dpkg-deb --build doku_0.1.0_amd64
```

### Docker Hub

Publish Docker image with Doku CLI:

```dockerfile
# Dockerfile.cli
FROM alpine:latest
RUN apk add --no-cache docker-cli
COPY bin/doku /usr/local/bin/doku
ENTRYPOINT ["doku"]
CMD ["--help"]
```

Build and push:
```bash
docker build -f Dockerfile.cli -t dokulabs/doku:v0.1.0 .
docker push dokulabs/doku:v0.1.0
```

---

## Metrics and Monitoring

### Track Downloads

Use GitHub API to track release downloads:

```bash
curl -s https://api.github.com/repos/dokulabs/doku-cli/releases/latest | \
  jq '.assets[] | {name: .name, downloads: .download_count}'
```

### Analytics

Consider adding:
- Anonymous usage tracking
- Error reporting (Sentry)
- Update notifications
- Feature usage metrics

---

## Updating the Catalog

The catalog is in a separate repository: `dokulabs/doku-catalog`

When releasing a new version:

1. Update catalog if needed
2. Tag the catalog repository
3. Doku CLI fetches latest catalog on `doku init`

Users get the latest catalog automatically:
```bash
doku catalog update
```

---

## Documentation Deployment

### GitHub Pages (Recommended)

1. Create `docs/` directory with documentation
2. Enable GitHub Pages in repository settings
3. Use a static site generator (MkDocs, Docusaurus, etc.)

### Automatic Deployment

Add workflow `.github/workflows/docs.yml`:

```yaml
name: Deploy Docs

on:
  push:
    branches: [ main ]
    paths:
      - 'docs/**'
      - '*.md'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: 3.x
      - run: pip install mkdocs-material
      - run: mkdocs gh-deploy --force
```

---

## Support Channels

After release, monitor:

- GitHub Issues
- GitHub Discussions
- Discord/Slack community
- Email support
- Twitter mentions

---

## Next Release Planning

### Feature Roadmap

Track planned features:

- [ ] Homebrew tap
- [ ] Windows installer
- [ ] Auto-update command
- [ ] Service templates
- [ ] Docker Compose import
- [ ] Kubernetes export
- [ ] Web dashboard
- [ ] VS Code extension

### Release Schedule

Suggested schedule:
- **Patch releases**: As needed for bugs
- **Minor releases**: Monthly with new features
- **Major releases**: Yearly or on breaking changes

---

## Quick Commands Reference

```bash
# Create and push release
git tag v0.2.0 -m "Release v0.2.0: Custom projects and env management"
git push origin v0.2.0

# Build locally
make build

# Test locally
./bin/doku version
./bin/doku init

# Test install script
./scripts/install.sh

# View releases
gh release list

# Download release asset
gh release download v0.2.0
```

---

**Last Updated**: 2025-11-18
**Current Version**: v0.1.0
