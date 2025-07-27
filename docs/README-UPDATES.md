# README.md Update Summary

## Key Changes Made

### 1. Network Architecture Updates
- **Removed**: References to auto-port assignment (9000-9999 range)
- **Removed**: Direct port access examples
- **Added**: Internal network architecture explanation
- **Added**: Security note about no direct port exposure

### 2. Request Persistence Feature
- **Added**: New section "Request Persistence & Replay" in architecture
- **Added**: `requests` CLI command documentation
- **Added**: API endpoints for request management
- **Added**: Crash resilience explanation

### 3. Installation Updates
- **Replaced**: Shell script installation with `make` commands
- **Changed**: `setup.sh` → `make setup`
- **Changed**: `install.sh` → `make install-user`
- **Added**: `make verify` command

### 4. New Features Section
- **Added**: "What's New" section highlighting:
  - Network Isolation
  - Request Persistence
  - Crash Resilience
  - Simplified Installation

### 5. Access Methods Simplification
- **Removed**: Three access methods (direct, proxy, API)
- **Simplified**: Only proxy and API access documented
- **Updated**: Examples to use only proxy endpoints

### 6. CLI Commands
- **Added**: `requests` command to view pending requests
- **Removed**: Port information from deployment examples
- **Added**: Note about deprecated `--port` flag

### 7. Project Structure
- **Added**: `internal/requests/` package
- **Added**: `scripts/` directory structure
- **Updated**: Development section with new make commands

### 8. Troubleshooting
- **Updated**: Common issues to reflect new architecture
- **Added**: Request persistence troubleshooting
- **Added**: `make verify` as diagnostic tool

### 9. Examples
- **Updated**: Deployment examples without port mappings
- **Added**: Note about request queuing on crashes
- **Added**: Note about proxy-based agent communication

### 10. API Reference
- **Added**: Request management endpoints:
  - GET `/agents/{id}/requests`
  - GET `/agents/{id}/requests/{reqId}`
  - POST `/agents/{id}/requests/{reqId}/replay`

These updates ensure the README accurately reflects the current state of Agentainer after completing EPICs 1-3, providing users with correct information about the enhanced security, reliability, and simplified usage of the system.