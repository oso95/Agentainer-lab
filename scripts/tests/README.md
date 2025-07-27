# Agentainer Test Scripts

This directory contains test scripts for validating various Agentainer features.

## Network Isolation Tests

- `test-network-isolation.sh` - Validates that agents are properly isolated in the internal Docker network and cannot be accessed directly

## Request Persistence Tests

- `test-request-persistence.sh` - Comprehensive test of request persistence feature
- `test-persistence-final.sh` - End-to-end test showing request queuing and replay
- `test-persistence-quick.sh` - Quick validation of request persistence
- `test-replay-debug.sh` - Debug script for troubleshooting replay functionality

## Crash Resilience Tests

- `test-crash-resilience.sh` - Tests how the system handles agent crashes during request processing
- `test-crash-simple.sh` - Simple demonstration of crash resilience with request persistence

## Running Tests

All test scripts are executable and can be run directly:

```bash
# Run a specific test
./scripts/tests/test-network-isolation.sh

# Run all persistence tests
for test in scripts/tests/test-persistence-*.sh; do
    echo "Running $test..."
    $test
done
```

## Prerequisites

- Agentainer server must be running (or the test will start it)
- Docker must be running
- Redis must be available (via docker-compose)

## Test Conventions

- All tests are self-contained and clean up after themselves
- Tests use nginx:alpine or python:3.9-slim as test images
- Tests create agents with descriptive names (e.g., test-persist, crash-demo)
- Tests output clear status messages and results

## Adding New Tests

When adding new test scripts:
1. Name them descriptively with `test-` prefix
2. Make them executable (`chmod +x`)
3. Include cleanup steps at the end
4. Add documentation to this README