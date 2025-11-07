# Volume Management and UX Improvements - Summary

## Overview

Implemented comprehensive volume management and user experience improvements for doku-cli, including:
1. Enhanced uninstall command with danger warnings and volume preservation options
2. Per-service volume removal prompts for the remove command
3. Custom domain selection during service installation
4. Simplified management UI URL for services like RabbitMQ

## Changes Made

### 1. Uninstall Command (`cmd/uninstall.go`)

#### Added Danger Warning
```bash
⚠️  DANGER: Doku Uninstall
⚠️  WARNING: This action CANNOT be undone!
```

#### Volume Preservation Prompt
- **Before**: Volumes were removed automatically
- **After**: User is asked whether to remove volumes
- **Default**: Volumes are preserved (safer default)

#### User Flow:
```bash
$ doku uninstall

⚠️  DANGER: Doku Uninstall
⚠️  WARNING: This action CANNOT be undone!

This will remove:
  • All Docker containers managed by Doku
  • Doku Docker network
  • Configuration directory (~/.doku/)
  • Docker volumes (you will be asked separately)

⚠️  Are you absolutely sure you want to uninstall Doku? This CANNOT be undone! (y/N)
> y

⚠️  Docker volumes contain your data (databases, files, etc.)
? Do you want to remove all Docker volumes? (This will delete all data) (y/N)
> n

→ Skipping Docker volumes (keeping your data)
  ✓ Preserved 3 Docker volume(s) with your data
```

#### Force Mode Behavior
- With `--force` flag: Removes volumes by default (explicit opt-in for automation)
- Interactive mode: Asks user, defaults to preserving volumes

### 2. Remove Command (`cmd/remove.go`)

#### Enhanced Volume Display
Shows each volume that will be affected:
```bash
⚠️  Remove Service: postgres

This will remove:
  • Container: doku-postgres
  • Volumes: 1 volume(s)
    - doku-postgres-data
  • Configuration for: postgres
```

#### Per-Service Volume Prompt
```bash
? Are you sure you want to remove 'postgres'? (y/N)
> y

⚠️  This service has Docker volumes containing data
? Do you want to remove the volumes? (This will delete all data) (y/N)
> n

Removing postgres...
✓ Service removed (volumes preserved)
```

#### Safety Features
- **Default**: Volumes are NOT removed (safer)
- **Interactive**: Explicit confirmation required to delete data
- **With `--yes` flag**: Volumes are preserved with warning message

### 3. Service Manager Updates (`internal/service/manager.go`)

#### Updated Method Signatures
```go
// Before:
func (m *Manager) Remove(instanceName string, force bool) error

// After:
func (m *Manager) Remove(instanceName string, force bool, removeVolumes bool) error
```

#### Multi-Container Support
Both single-container and multi-container services support selective volume removal:
```go
func (m *Manager) removeMultiContainerService(instance *types.Instance, force bool, removeVolumes bool) error {
    // ... remove containers ...

    // Remove associated volumes only if user agreed
    if removeVolumes {
        if err := m.removeMultiContainerVolumes(instance); err != nil {
            fmt.Printf("Warning: failed to remove some volumes: %v\n", err)
        }
    }

    return m.configMgr.RemoveInstance(instance.Name)
}
```

### 4. Install Command - Custom Domain (`cmd/install.go`)

#### Domain Selection Prompt
Users can now choose custom domains during installation:

```bash
$ doku install rabbitmq

Installing: 🐰 RabbitMQ
Message broker with management UI

Instance name: rabbitmq

? Domain for this service: (doku.local)
> myproject.local

URL: https://rabbitmq.myproject.local

? Proceed with installation? (Y/n)
```

#### Features:
- **Default**: Uses configured domain (doku.local)
- **Customizable**: Can specify any domain (myproject.local, dev.local, etc.)
- **Help Text**: Provides examples of valid domains
- **Skip with `--yes`**: Uses default domain in non-interactive mode

### 5. RabbitMQ Admin URL Simplification

#### URL Display Change
```bash
# Before:
Admin: https://rabbitmq-admin.doku.local (port 15672)

# After:
Management UI: https://rabbitmq.doku.local (port 15672)
```

#### Rationale:
- **Cleaner URLs**: No separate `-admin` subdomain needed
- **Intuitive**: Main URL serves the management UI for web access
- **Protocol Access**: AMQP port 5672 accessed via direct connection
- **Consistency**: Aligns with how users expect to access web UIs

## User Experience Improvements

### Safety First
1. **Dangerous operations clearly marked** with red warnings
2. **Cannot be undone** messages for irreversible actions
3. **Volume preservation by default** to prevent accidental data loss
4. **Explicit confirmation** required for data deletion

### Clear Communication
1. **Color-coded messages**:
   - Red: Danger/warnings
   - Yellow: Cautions/notices
   - Green: Success
   - Cyan: Actions/progress

2. **Informative prompts**:
   - Explain what will happen
   - Show affected resources
   - Provide context for decisions

3. **Status feedback**:
   - Progress indicators
   - Success confirmations
   - Helpful next steps

### Flexible Options
1. **Interactive mode**: Guided experience with prompts
2. **Non-interactive mode**: `--yes` flag for automation
3. **Force mode**: `--force` for emergency operations
4. **Customization**: Domain selection, volume management

## Examples

### Example 1: Safe Service Removal
```bash
$ doku remove postgres

⚠️  Remove Service: postgres

This will remove:
  • Container: doku-postgres
  • Volumes: 1 volume(s)
    - doku-postgres-data
  • Configuration for: postgres

? Are you sure you want to remove 'postgres'? (y/N)
> y

⚠️  This service has Docker volumes containing data
? Do you want to remove the volumes? (This will delete all data) (y/N)
> n

Removing postgres...
✓ Service removed (volumes preserved)

# Data is safe! Can reinstall later:
$ doku install postgres --name postgres
# Will reuse existing volume with data intact
```

### Example 2: Complete Removal
```bash
$ doku remove redis

⚠️  Remove Service: redis

This will remove:
  • Container: doku-redis
  • Volumes: 1 volume(s)
    - doku-redis-data

? Are you sure you want to remove 'redis'? (y/N)
> y

⚠️  This service has Docker volumes containing data
? Do you want to remove the volumes? (This will delete all data) (y/N)
> y

Removing redis...
✓ Service removed successfully

# Complete cleanup - no data remains
```

### Example 3: System Uninstall with Volume Preservation
```bash
$ doku uninstall

⚠️  DANGER: Doku Uninstall
⚠️  WARNING: This action CANNOT be undone!

This will remove:
  • All Docker containers managed by Doku
  • Doku Docker network
  • Configuration directory (~/.doku/)
  • Docker volumes (you will be asked separately)

⚠️  Are you absolutely sure you want to uninstall Doku? This CANNOT be undone! (y/N)
> y

⚠️  Docker volumes contain your data (databases, files, etc.)
? Do you want to remove all Docker volumes? (This will delete all data) (y/N)
> n

→ Stopping and removing Docker containers...
  ✓ Stopped doku-postgres
  ✓ Removed doku-postgres
  ✓ Stopped doku-redis
  ✓ Removed doku-redis

→ Skipping Docker volumes (keeping your data)
  ✓ Preserved 2 Docker volume(s) with your data

→ Removing Docker network...
  ✓ Removed network doku-network

→ Removing configuration directory...
  ✓ Removed /Users/user/.doku/

✓ Cleanup Complete

Removed:
  • 2 Docker container(s)
  • Docker network
  • Configuration directory

# Volumes preserved! Can reinstall doku later and reuse data:
$ doku init
$ doku install postgres  # Will find and use existing volume
```

### Example 4: Custom Domain Installation
```bash
$ doku install postgres

Installing: 🐘 PostgreSQL
Open source relational database

Instance name: postgres

? Domain for this service: (doku.local)
> myapp.local

URL: https://postgres.myapp.local

? Proceed with installation? (Y/n)
> y

✓ Successfully installed postgres

Access your service:
  URL: https://postgres.myapp.local

# Make sure to add DNS entry:
echo "127.0.0.1 postgres.myapp.local" | sudo tee -a /etc/hosts
```

## Migration Notes

### For Existing Users
- **Volume behavior changed**: Volumes are now preserved by default
- **Old behavior**: Use interactive prompts and explicitly choose to remove volumes
- **Scripts/Automation**: Update scripts using `doku remove` to handle volume preservation

### Backward Compatibility
- All existing commands work with default behaviors
- New prompts only appear in interactive mode
- `--yes` and `--force` flags maintain predictable behavior
- No breaking changes to command syntax

## Best Practices

### Development Workflow
```bash
# Install service
doku install postgres

# Work with service...

# Remove service but keep data
doku remove postgres
# Answer "N" to volume removal

# Reinstall later with data intact
doku install postgres
```

### Production Cleanup
```bash
# Complete removal including data
doku remove postgres
# Answer "Y" to volume removal

# Or for automation:
doku remove postgres --yes  # Preserves volumes
# Then manually remove volumes if needed:
docker volume rm doku-postgres-data
```

### System Maintenance
```bash
# Clean install (keep data)
doku uninstall
# Answer "Y" to uninstall, "N" to volume removal
doku init
# Reinstall services - they'll reuse existing volumes

# Fresh start (delete everything)
doku uninstall
# Answer "Y" to both prompts
doku init
# Start from scratch
```

## Testing

### Test Scenarios

1. **Volume Preservation**:
   ```bash
   doku install postgres
   # Create test data
   doku remove postgres  # Choose to preserve volumes
   doku install postgres  # Data should still exist
   ```

2. **Volume Deletion**:
   ```bash
   doku install redis
   doku remove redis  # Choose to remove volumes
   doku install redis  # Should start fresh
   ```

3. **System Uninstall**:
   ```bash
   doku install postgres redis
   doku uninstall  # Choose to preserve volumes
   # Volumes should remain in Docker
   docker volume ls | grep doku
   ```

4. **Custom Domain**:
   ```bash
   doku install rabbitmq
   # Enter custom domain when prompted
   # Verify URL uses custom domain
   ```

## Future Enhancements

Potential improvements:
- Volume size display before removal
- Backup creation before volume deletion
- Volume migration between services
- Bulk volume management commands
- Volume usage statistics
