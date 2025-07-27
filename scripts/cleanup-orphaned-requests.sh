#!/bin/bash
# Clean up orphaned request queues from removed agents

echo "Cleaning up orphaned request queues..."

# Get all agents that actually exist
EXISTING_AGENTS=$(redis-cli smembers agents:list)

# Get all request queue keys
REQUEST_KEYS=$(redis-cli keys "agent:*:requests:pending")

# Count before cleanup
TOTAL_KEYS=$(echo "$REQUEST_KEYS" | wc -l)
echo "Found $TOTAL_KEYS request queue keys"

# Check each request queue
CLEANED=0
for KEY in $REQUEST_KEYS; do
    # Extract agent ID from key
    AGENT_ID=$(echo $KEY | sed 's/agent:\(.*\):requests:pending/\1/')
    
    # Check if agent exists
    if ! echo "$EXISTING_AGENTS" | grep -q "$AGENT_ID"; then
        echo "Removing orphaned queue for non-existent agent: $AGENT_ID"
        # Remove the pending queue
        redis-cli del "$KEY" > /dev/null
        # Also remove any related keys
        redis-cli del "agent:$AGENT_ID:requests:*" > /dev/null 2>&1
        CLEANED=$((CLEANED + 1))
    fi
done

echo "Cleaned up $CLEANED orphaned request queues"