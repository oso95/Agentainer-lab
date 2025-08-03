#!/usr/bin/env python3
"""
MapReduce Mapper Example - URL Word Counter

This mapper demonstrates the MapReduce pattern in Agentainer Flow.
It processes URLs and counts words on each page.
"""

import os
import json
import time
import redis
import requests
from collections import Counter

# Connect to Redis (Agentainer provides Redis for state management)
redis_client = redis.Redis(host=os.environ.get('REDIS_HOST', 'redis'), port=6379, decode_responses=True)

# Get workflow metadata from environment
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', 'test-workflow')
STEP_ID = os.environ.get('STEP_ID', 'mapper')
TASK_ID = os.environ.get('TASK_ID', '0')
STEP_TYPE = os.environ.get('STEP_TYPE', 'map')

def get_workflow_state(key):
    """Get value from workflow state"""
    state_key = f"workflow:{WORKFLOW_ID}:state"
    value = redis_client.hget(state_key, key)
    if value:
        return json.loads(value)
    return None

def set_workflow_state(key, value):
    """Set value in workflow state"""
    state_key = f"workflow:{WORKFLOW_ID}:state"
    redis_client.hset(state_key, key, json.dumps(value))

def append_to_list(key, value):
    """Append to a list in workflow state"""
    list_key = f"workflow:{WORKFLOW_ID}:state:list:{key}"
    redis_client.rpush(list_key, json.dumps(value))

def list_phase():
    """List phase - generate list of items to process"""
    print(f"[LIST] Starting list phase for workflow {WORKFLOW_ID}")
    
    # Example URLs to process
    urls = [
        "https://example.com",
        "https://httpbin.org/html",
        "https://httpbin.org/json",
        "https://httpbin.org/uuid",
        "https://httpbin.org/user-agent",
    ]
    
    # Store URLs in workflow state
    set_workflow_state("urls", urls)
    set_workflow_state("total_items", len(urls))
    
    print(f"[LIST] Generated {len(urls)} URLs to process")
    return urls

def map_phase():
    """Map phase - process individual items"""
    print(f"[MAP] Starting map phase for task {TASK_ID}")
    
    # Get URLs from state
    urls = get_workflow_state("urls")
    if not urls:
        print("[MAP] No URLs found in state")
        return
    
    # Determine which URL this task should process
    task_index = int(TASK_ID.split('-')[-1]) if TASK_ID.startswith('task-') else 0
    
    if task_index >= len(urls):
        print(f"[MAP] Task index {task_index} exceeds URL count")
        return
    
    url = urls[task_index]
    print(f"[MAP] Processing URL: {url}")
    
    try:
        # Fetch the URL
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        
        # Count words (simple split-based counting)
        text = response.text
        words = text.split()
        word_count = len(words)
        
        # Get top 10 most common words
        word_freq = Counter(words)
        top_words = dict(word_freq.most_common(10))
        
        # Store result
        result = {
            "task_id": TASK_ID,
            "url": url,
            "word_count": word_count,
            "top_words": top_words,
            "status_code": response.status_code,
            "content_length": len(text),
            "timestamp": time.time()
        }
        
        # Append to results list
        append_to_list("map_results", result)
        
        print(f"[MAP] Processed {url}: {word_count} words found")
        
    except Exception as e:
        print(f"[MAP] Error processing {url}: {str(e)}")
        error_result = {
            "task_id": TASK_ID,
            "url": url,
            "error": str(e),
            "timestamp": time.time()
        }
        append_to_list("map_errors", error_result)

def main():
    """Main entry point"""
    print(f"Starting mapper - Step Type: {STEP_TYPE}")
    
    if STEP_TYPE == "list":
        list_phase()
    elif STEP_TYPE == "map":
        map_phase()
    else:
        print(f"Unknown step type: {STEP_TYPE}")
    
    # Simulate some processing time
    time.sleep(2)
    print("Mapper completed")

if __name__ == "__main__":
    main()