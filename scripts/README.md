# Agentainer Scripts Directory

This directory contains helper scripts for Agentainer Lab.

## Current Scripts

- `install-prerequisites.sh` - Installs all prerequisites (Git, Go, Docker) on fresh VMs

## Deprecated Scripts

The following scripts have been consolidated into the Makefile and are kept in `deprecated/` for reference:

- `setup.sh` → Use `make setup` instead
- `install.sh` → Use `make install-user` instead
- `uninstall.sh` → Use `make uninstall-user` instead
- `verify-setup.sh` → Use `make verify` instead

## Usage

All installation and setup tasks should now be done through the Makefile:

```bash
# Fresh VM setup
make setup

# Just install prerequisites
make install-prerequisites

# Install Agentainer
make install-user

# Verify installation
make verify
```

For a complete list of available commands, run `make help`.