# CLI Reference

Complete reference for all Agentainer CLI commands and options.

## Global Options

These options can be used with any command:

- `--help, -h`: Show help for command
- `--version, -v`: Show version information
- `--config`: Specify config file location (default: `~/.agentainer/config.yaml`)
- `--verbose`: Enable verbose output
- `--json`: Output in JSON format

## Commands

### `agentainer deploy`

Deploy a new agent from a Docker image or Dockerfile.

```bash
agentainer deploy --name <name> --image <image> [options]
```

**Options:**
- `--name, -n`: Agent name (required)
- `--image, -i`: Docker image or Dockerfile path (required)
- `--config`: Deploy from YAML configuration file
- `--env, -e`: Set environment variables (can be used multiple times)
- `--volume, -v`: Mount volumes (format: `host:container[:mode]`)
- `--cpu`: CPU limit (e.g., `0.5`, `2`)
- `--memory, -m`: Memory limit (e.g., `256M`, `1G`)
- `--auto-restart`: Enable automatic restart on failure
- `--restart-max-retries`: Maximum restart attempts (default: unlimited)
- `--restart-delay`: Delay between restarts (default: `10s`)
- `--token`: Custom authentication token for this agent
- `--health-endpoint`: Health check endpoint path
- `--health-interval`: Health check interval (default: `30s`)
- `--health-timeout`: Health check timeout (default: `5s`)
- `--health-retries`: Failures before marking unhealthy (default: `3`)
- `--health-start-period`: Grace period on startup (default: `0s`)

**Examples:**
```bash
# Deploy from Docker Hub
agentainer deploy --name web --image nginx:latest

# Deploy from Dockerfile
agentainer deploy --name api --image ./Dockerfile

# Deploy with options
agentainer deploy --name worker \
  --image worker:v1.0 \
  --env WORKER_TYPE=processor \
  --env LOG_LEVEL=debug \
  --volume ./data:/app/data \
  --cpu 2 \
  --memory 1G \
  --auto-restart

# Deploy from YAML
agentainer deploy --config deployment.yaml
```

### `agentainer start`

Start a stopped agent.

```bash
agentainer start <agent-id>
```

**Examples:**
```bash
agentainer start agent-123...89
agentainer start my-agent-387..94
```

### `agentainer stop`

Stop a running agent.

```bash
agentainer stop <agent-id> [options]
```

**Options:**
- `--timeout, -t`: Seconds to wait before force stopping (default: `10`)

**Examples:**
```bash
agentainer stop agent-123
agentainer stop my-agent --timeout 30
```

### `agentainer restart`

Restart a running agent (stop + start).

```bash
agentainer restart <agent-id> [options]
```

**Options:**
- `--timeout, -t`: Seconds to wait for stop (default: `10`)

**Examples:**
```bash
agentainer restart agent-123
agentainer restart my-agent-2439 --timeout 5
```

### `agentainer pause`

Pause agent execution (container keeps running).

```bash
agentainer pause <agent-id>
```

**Examples:**
```bash
agentainer pause agent-123
```

### `agentainer resume`

Resume any non-running agent (works on stopped, paused, or crashed agents).

```bash
agentainer resume <agent-id>
```

**Examples:**
```bash
# Resume after crash
agentainer resume agent-123

# Resume after pause
agentainer resume my-agent-3429

# Resume after stop
agentainer resume worker-1
```

### `agentainer remove`

Remove an agent and its container.

```bash
agentainer remove <agent-id> [options]
```

**Options:**
- `--force, -f`: Force removal even if running
- `--volumes`: Remove associated volumes

**Examples:**
```bash
agentainer remove agent-123
agentainer remove my-agent --force
agentainer remove worker --volumes
```

### `agentainer list`

List all agents and their status.

```bash
agentainer list [options]
```

**Options:**
- `--all, -a`: Show all agents including removed
- `--filter, -f`: Filter agents (e.g., `status=running`)
- `--format`: Output format (table, json, csv)
- `--quiet, -q`: Only display agent IDs

**Examples:**
```bash
# List all agents
agentainer list

# List running agents only
agentainer list --filter status=running

# List as JSON
agentainer list --format json

# Get agent IDs only
agentainer list -q
```

### `agentainer logs`

View agent logs.

```bash
agentainer logs <agent-id> [options]
```

**Options:**
- `--follow, -f`: Follow log output
- `--tail`: Number of lines to show from end (default: all)
- `--since`: Show logs since timestamp (e.g., `2023-01-01T00:00:00`)
- `--until`: Show logs until timestamp
- `--timestamps, -t`: Show timestamps

**Examples:**
```bash
# View all logs
agentainer logs agent-123

# Follow logs in real-time
agentainer logs my-agent --follow

# Last 100 lines
agentainer logs worker --tail 100

# Logs from last hour
agentainer logs api --since 1h
```

### `agentainer inspect`

Show detailed information about an agent.

```bash
agentainer inspect <agent-id> [options]
```

**Options:**
- `--format, -f`: Format output using Go template

**Examples:**
```bash
# Full inspection
agentainer inspect agent-123

# Get specific field
agentainer inspect my-agent --format '{{.Config.Image}}'
```

### `agentainer requests`

View and manage pending requests for an agent.

```bash
agentainer requests <agent-id> [subcommand]
```

**Subcommands:**
- `list`: List all pending requests (default)
- `show <request-id>`: Show request details
- `replay <request-id>`: Manually replay a request
- `clear`: Clear all pending requests

**Examples:**
```bash
# List pending requests
agentainer requests agent-123

# Show request details
agentainer requests agent-123 show req-456

# Replay specific request
agentainer requests agent-123 replay req-456

# Clear all requests
agentainer requests agent-123 clear
```

### `agentainer invoke`

Invoke an agent endpoint through the API (with authentication).

```bash
agentainer invoke <agent-id> [options]
```

**Options:**
- `--method, -X`: HTTP method (GET, POST, PUT, DELETE)
- `--path, -p`: Endpoint path (default: `/`)
- `--data, -d`: Request body data
- `--header, -H`: Add header (can be used multiple times)
- `--token`: Override default auth token

**Examples:**
```bash
# GET request
agentainer invoke agent-123 --path /health

# POST with data
agentainer invoke my-agent \
  --method POST \
  --path /api/process \
  --data '{"input": "data"}'

# With custom headers
agentainer invoke api \
  --method POST \
  --path /webhook \
  --header "X-Custom: value" \
  --data @payload.json
```

### `agentainer health`

View health status of agents.

```bash
agentainer health [agent-id] [options]
```

**Options:**
- `--watch, -w`: Continuously monitor health
- `--interval`: Watch interval (default: `5s`)

**Examples:**
```bash
# All agents health
agentainer health

# Specific agent
agentainer health agent-123

# Monitor continuously
agentainer health --watch --interval 10s
```

### `agentainer metrics`

View resource metrics for agents.

```bash
agentainer metrics <agent-id> [options]
```

**Options:**
- `--history`: Show historical data
- `--duration`: History duration (e.g., `1h`, `24h`)
- `--interval`: Data point interval
- `--format`: Output format (table, json, csv)

**Examples:**
```bash
# Current metrics
agentainer metrics agent-123

# Last hour history
agentainer metrics my-agent --history --duration 1h

# Export as CSV
agentainer metrics worker --history --format csv > metrics.csv
```

### `agentainer backup`

Backup and restore agent configurations and data.

```bash
agentainer backup <subcommand> [options]
```

**Subcommands:**

#### `create`
Create a new backup.

**Options:**
- `--name`: Backup name (required)
- `--description`: Backup description
- `--agents`: Specific agents to backup (comma-separated)
- `--exclude-volumes`: Don't backup volume data

**Example:**
```bash
agentainer backup create \
  --name "prod-backup" \
  --description "Weekly production backup" \
  --agents agent-1,agent-2
```

#### `list`
List available backups.

**Example:**
```bash
agentainer backup list
```

#### `restore`
Restore from backup.

**Options:**
- `--agents`: Restore specific agents only
- `--force`: Overwrite existing agents

**Example:**
```bash
agentainer backup restore backup-123 --agents agent-1
```

#### `export`
Export backup to file.

**Example:**
```bash
agentainer backup export backup-123 ./backup.tar.gz
```

#### `import`
Import backup from file.

**Example:**
```bash
agentainer backup import ./backup.tar.gz
```

#### `delete`
Delete a backup.

**Example:**
```bash
agentainer backup delete backup-123
```

### `agentainer audit`

View audit logs of all administrative actions.

```bash
agentainer audit [options]
```

**Options:**
- `--user`: Filter by user
- `--action`: Filter by action type
- `--resource`: Filter by resource type
- `--duration`: Time range (e.g., `24h`, `7d`)
- `--since`: Start time
- `--until`: End time
- `--limit`: Maximum entries to show
- `--format`: Output format (table, json, csv)

**Examples:**
```bash
# All audit logs
agentainer audit

# Filter by action
agentainer audit --action deploy_agent --duration 24h

# Filter by user
agentainer audit --user admin --duration 7d

# Export as JSON
agentainer audit --format json --limit 1000 > audit.json
```

### `agentainer config`

Manage Agentainer configuration.

```bash
agentainer config <subcommand>
```

**Subcommands:**
- `show`: Display current configuration
- `set <key> <value>`: Set configuration value
- `get <key>`: Get configuration value
- `reset`: Reset to defaults

**Examples:**
```bash
# Show all config
agentainer config show

# Set API endpoint
agentainer config set api.endpoint http://localhost:8081

# Get specific value
agentainer config get api.token

# Reset to defaults
agentainer config reset
```

### `agentainer version`

Show version information.

```bash
agentainer version [options]
```

**Options:**
- `--short`: Show version number only

**Examples:**
```bash
# Full version info
agentainer version

# Version number only
agentainer version --short
```

## Environment Variables

Agentainer CLI can be configured using environment variables:

- `AGENTAINER_API_URL`: API endpoint (default: `http://localhost:8081`)
- `AGENTAINER_API_TOKEN`: Authentication token
- `AGENTAINER_CONFIG_DIR`: Configuration directory (default: `~/.agentainer`)
- `AGENTAINER_LOG_LEVEL`: Log level (debug, info, warn, error)
- `NO_COLOR`: Disable colored output

## Configuration File

The CLI uses a configuration file at `~/.agentainer/config.yaml`:

```yaml
api:
  endpoint: http://localhost:8081
  token: agentainer-default-token
  timeout: 30s

defaults:
  cpu: 1
  memory: 512M
  auto_restart: true

formatting:
  colors: true
  timestamps: false
```

## Exit Codes

- `0`: Success
- `1`: General error
- `2`: Invalid arguments
- `3`: Agent not found
- `4`: Permission denied
- `5`: Connection error
- `6`: Timeout

## Tips and Tricks

### Aliases

Add these to your shell configuration:

```bash
alias ag='agentainer'
alias agl='agentainer list'
alias ags='agentainer start'
alias agx='agentainer stop'
alias aglogs='agentainer logs -f'
```

### Shell Completion

Enable shell completion:

```bash
# Bash
agentainer completion bash > /etc/bash_completion.d/agentainer

# Zsh
agentainer completion zsh > "${fpath[1]}/_agentainer"

# Fish
agentainer completion fish > ~/.config/fish/completions/agentainer.fish
```

### Common Workflows

```bash
# Quick deploy and start
agentainer deploy --name test --image nginx && agentainer start test

# Watch logs during debugging
agentainer logs my-agent -f | grep ERROR

# Batch operations
agentainer list -q | xargs -I {} agentainer stop {}

# Export metrics for analysis
for agent in $(agentainer list -q); do
  agentainer metrics $agent --history --format csv > ${agent}-metrics.csv
done
```