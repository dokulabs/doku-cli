# Catalog Sources

This document explains how to configure the catalog source for `doku catalog update`.

## Overview

By default, Doku downloads the service catalog from the `main` branch of the [doku-catalog](https://github.com/dokulabs/doku-catalog) repository. You can customize this to use:

- Different branches (for development/testing)
- Specific tags (for version pinning)
- Custom URLs (for private catalogs)

## Usage

### Command-Line Flag

Use the `--source` or `-s` flag with `doku catalog update`:

```bash
# Update from a specific branch
doku catalog update --source develop
doku catalog update --source feature-new-services

# Update from a specific tag
doku catalog update --source v1.0.0
doku catalog update --source v2.1.3

# Update from a custom URL
doku catalog update --source https://example.com/custom-catalog.tar.gz
```

### Environment Variable

Set the `DOKU_CATALOG_SOURCE` environment variable:

```bash
# Temporarily for one command
DOKU_CATALOG_SOURCE=develop doku catalog update

# Set persistently in your shell
export DOKU_CATALOG_SOURCE=develop
doku catalog update

# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
echo 'export DOKU_CATALOG_SOURCE=develop' >> ~/.zshrc
```

### Priority Order

When multiple sources are specified, Doku uses this priority:

1. Command-line flag (`--source`)
2. Environment variable (`DOKU_CATALOG_SOURCE`)
3. Default (GitHub main branch)

## Source Format Detection

Doku automatically detects the source type:

### Branch Names
Simple names without dots or version prefixes are treated as branches:
- `main` → `https://github.com/dokulabs/doku-catalog/archive/refs/heads/main.tar.gz`
- `develop` → `https://github.com/dokulabs/doku-catalog/archive/refs/heads/develop.tar.gz`
- `feature-x` → `https://github.com/dokulabs/doku-catalog/archive/refs/heads/feature-x.tar.gz`

### Tag Names
Sources starting with 'v' or containing dots are treated as tags:
- `v1.0.0` → `https://github.com/dokulabs/doku-catalog/archive/refs/tags/v1.0.0.tar.gz`
- `1.0.0` → `https://github.com/dokulabs/doku-catalog/archive/refs/tags/1.0.0.tar.gz`
- `v2.1.3` → `https://github.com/dokulabs/doku-catalog/archive/refs/tags/v2.1.3.tar.gz`

### Full URLs
URLs starting with `http://` or `https://` are used as-is:
- `https://example.com/catalog.tar.gz`
- `https://github.com/myorg/my-catalog/archive/refs/heads/main.tar.gz`

## Use Cases

### Development/Testing

When testing catalog changes locally before merging:

```bash
# Use your development branch
doku catalog update --source my-feature-branch

# Test the new services
doku catalog list
doku install new-service
```

### Version Pinning

Pin to a specific catalog version for reproducibility:

```bash
# Pin to v1.0.0
export DOKU_CATALOG_SOURCE=v1.0.0

# All updates will use this version
doku catalog update
```

### Private Catalogs

Use your own catalog repository:

```bash
# Update from private GitHub repo
doku catalog update --source https://github.com/mycompany/private-catalog/archive/refs/heads/main.tar.gz

# Update from internal server
doku catalog update --source https://internal.example.com/catalogs/services.tar.gz
```

### Team Development

Share a development catalog across your team:

```bash
# In team documentation or setup script
export DOKU_CATALOG_SOURCE=team-develop
doku catalog update
```

## Local Development Workflow

For local catalog development without publishing to GitHub:

1. **Clone the catalog repository:**
   ```bash
   git clone https://github.com/dokulabs/doku-catalog.git
   cd doku-catalog
   ```

2. **Make your changes:**
   ```bash
   # Add new service or modify existing ones
   vim services/database/mydb/service.yaml
   ```

3. **Copy to local catalog:**
   ```bash
   # Copy entire catalog
   cp -r services/* ~/.doku/catalog/services/

   # Or copy specific service
   cp -r services/database/mydb ~/.doku/catalog/services/database/
   ```

4. **Test locally:**
   ```bash
   doku catalog list
   doku install mydb
   ```

5. **When ready, push to branch:**
   ```bash
   git checkout -b add-mydb
   git add .
   git commit -m "Add mydb service"
   git push origin add-mydb
   ```

6. **Team can test your branch:**
   ```bash
   doku catalog update --source add-mydb
   ```

## Troubleshooting

### Catalog Update Fails

**Problem:** `failed to download catalog: HTTP 404`

**Solution:**
- Verify the branch/tag exists
- Check the URL is correct
- For private repos, ensure you have access

### Changes Not Appearing

**Problem:** Updated catalog but changes don't show

**Solution:**
```bash
# Clear local catalog cache
rm -rf ~/.doku/catalog

# Update again
doku catalog update --source your-branch
```

### Testing Local Changes

**Problem:** Want to test without pushing to GitHub

**Solution:**
```bash
# Option 1: Copy directly to local catalog
cp -r /path/to/local-catalog/* ~/.doku/catalog/

# Option 2: Create local tarball and use file URL
cd /path/to/local-catalog
tar czf /tmp/catalog.tar.gz .
doku catalog update --source file:///tmp/catalog.tar.gz
```

## Best Practices

1. **Use version tags for production:** Pin to specific tags for stability
2. **Use branches for development:** Test on feature branches before merging
3. **Document your source:** Add catalog source to project documentation
4. **Team conventions:** Agree on catalog source strategy with your team
5. **Test before sharing:** Validate catalog changes locally before pushing

## Examples

```bash
# Development workflow
export DOKU_CATALOG_SOURCE=develop
doku catalog update
doku install postgres:17-pgvector

# Production with version pinning
export DOKU_CATALOG_SOURCE=v1.2.0
doku catalog update

# Testing a PR
doku catalog update --source pr-123-add-service

# Custom company catalog
export DOKU_CATALOG_SOURCE=https://catalog.mycompany.com/latest.tar.gz
doku catalog update

# Quick local test
cp -r ~/projects/doku-catalog/services/* ~/.doku/catalog/services/
doku catalog list
```

## Related Commands

```bash
# Show current catalog info
doku catalog list

# Search for services
doku catalog search postgres

# Show service details
doku catalog show postgres --verbose

# Check catalog version
cat ~/.doku/catalog/catalog.yaml | grep version
```
