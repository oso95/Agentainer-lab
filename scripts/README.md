# Agentainer Scripts Directory

This directory contains helper scripts for Agentainer Lab.

## Directory Structure

- `install-prerequisites.sh` - Installs all prerequisites (Git, Go, Docker) on fresh VMs

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