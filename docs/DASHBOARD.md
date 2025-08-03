# Agentainer Flow Dashboard

The Agentainer Flow Dashboard provides a comprehensive web interface for monitoring and managing workflows in real-time.

## Features

- **Real-time Monitoring**: Live updates via WebSocket connections
- **Workflow Management**: View, filter, and analyze workflow executions
- **Performance Metrics**: Detailed resource usage and performance profiling
- **Agent Pool Monitoring**: Track agent pool statistics and utilization
- **Multi-tenant Support**: Secure access with tenant isolation

## Access

The dashboard is integrated into the main Agentainer API server and is available at:
```
http://localhost:8081/dashboard
```

## Configuration

The dashboard can be configured in your `config.yaml`:

```yaml
dashboard:
  enabled: true        # Enable/disable dashboard (default: true)
```

Or via environment variables:
```bash
AGENTAINER_DASHBOARD_ENABLED=true
```

Note: The dashboard is now integrated into the main API server and uses the same host and port as configured in the server section.

## Dashboard Pages

### Home Dashboard (`/dashboard/`)
- Overview of system status
- Active workflow count
- Recent workflow executions
- System health indicators

### Workflows (`/dashboard/workflows`)
- List all workflows with filtering options
- Status indicators (running, completed, failed)
- Quick actions for workflow management

### Workflow Detail (`/dashboard/workflow/{id}`)
- Detailed workflow execution view
- Step-by-step execution timeline
- Resource usage metrics
- Error logs and debugging information

### Metrics (`/dashboard/metrics`)
- Aggregate system metrics
- Performance trends
- Resource utilization graphs
- Historical data analysis

### Agents (`/dashboard/agents`)
- Active agent listing
- Agent pool statistics
- Resource allocation overview

## API Endpoints

The dashboard also provides RESTful API endpoints:

- `GET /dashboard/api/workflows` - List workflows with optional status filter
- `GET /dashboard/api/workflow/{id}` - Get workflow details
- `GET /dashboard/api/workflow/{id}/metrics` - Get workflow metrics
- `GET /dashboard/api/workflow/{id}/profile` - Get performance profile
- `GET /dashboard/api/metrics/aggregate` - Get aggregate metrics
- `GET /dashboard/api/metrics/realtime` - Get real-time metrics snapshot
- `GET /dashboard/api/agent-pools` - Get agent pool information
- `GET /dashboard/api/agents/active` - List active agents

## WebSocket Support

Connect to `/dashboard/ws` for real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8081/dashboard/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'metrics_update') {
    // Handle metrics update
    console.log('Metrics:', data.data);
  }
};
```

## Performance Profiling

Export workflow performance profiles in multiple formats:

```bash
# JSON format (default)
curl http://localhost:8081/dashboard/api/workflow/{id}/profile

# CSV format
curl http://localhost:8081/dashboard/api/workflow/{id}/profile?format=csv
```

## Security

The dashboard inherits security settings from the main Agentainer configuration:
- Authentication via API tokens
- Tenant isolation for multi-tenant deployments
- Audit logging for all operations

## Development

### Template Structure
- Templates are embedded in the binary using Go's embed feature
- Base template provides consistent layout
- Individual pages extend the base template

### Static Assets
- CSS files are embedded for easy deployment
- No external dependencies required

### Adding New Pages

1. Create template in `internal/dashboard/templates/`
2. Add handler in `internal/dashboard/server.go`
3. Register route in the `Start()` method

## Troubleshooting

### Dashboard Not Starting
- Check if port 8080 is already in use
- Verify Redis connection is available
- Check logs for startup errors

### No Real-time Updates
- Ensure WebSocket connections are not blocked by proxy/firewall
- Check browser console for WebSocket errors
- Verify Redis pub/sub is working correctly

### Missing Metrics
- Ensure workflows are running with metrics collection enabled
- Check Redis connection for metrics storage
- Verify MetricsCollector is properly initialized