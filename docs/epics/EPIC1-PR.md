# EPIC 1: Network Architecture Refactor - Network Isolation

## Summary
This PR implements complete network isolation for Agentainer agents by removing direct port exposure and routing all traffic through the authenticated proxy endpoint.

## Changes
- ✅ Created internal Docker network (`agentainer-network`) for agent communication
- ✅ Removed automatic port assignment (9000-9999 range)
- ✅ Updated proxy handler to connect to agents via container hostname
- ✅ Modified CLI output to show only proxy access methods
- ✅ Updated docker-compose.yml with network configuration
- ✅ Marked port flags as deprecated (kept for backward compatibility)

## Security Improvements
- **No Direct Access**: Agents cannot be accessed directly from the host
- **Authenticated Proxy**: All traffic goes through the authenticated API gateway
- **Network Isolation**: Complete isolation between agent containers
- **Reduced Attack Surface**: No exposed ports on the host

## Breaking Changes
⚠️ **Direct port access is no longer available**
- Old: `http://localhost:9001` (direct to container)
- New: `http://localhost:8081/agent/{id}/` (through proxy only)

## Testing
```bash
# Run the test script
./test-network-isolation.sh

# Or manually test:
1. Start server: agentainer server
2. Deploy agent: agentainer deploy --name test --image nginx:alpine
3. Start agent: agentainer start <agent-id>
4. Access via proxy: curl http://localhost:8081/agent/<agent-id>/
```

## Migration Guide
Users upgrading from previous versions should:
1. Update any scripts/tools using direct port access
2. Use the proxy endpoint for all agent communication
3. Remove any firewall rules for ports 9000-9999

## Next Steps
- EPIC 2: Implement proxy-level request persistence and replay
- EPIC 3: Add monitoring UI
- EPIC 4: YAML deployment support