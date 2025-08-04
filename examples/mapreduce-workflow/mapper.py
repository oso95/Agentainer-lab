#!/usr/bin/env python3
"""
MapReduce Mapper Example - URL Word Counter

This mapper demonstrates the MapReduce pattern in Agentainer.
It processes URLs and counts words on each page.
"""

import os
import sys
import json
import time
import redis
import requests
from collections import Counter
import re
from bs4 import BeautifulSoup

# Connect to Redis (Agentainer provides Redis for state management)
redis_host = os.environ.get('REDIS_HOST', 'host.docker.internal')
redis_port = int(os.environ.get('REDIS_PORT', '6379'))

try:
    redis_client = redis.Redis(host=redis_host, port=redis_port, decode_responses=True)
    redis_client.ping()
except redis.ConnectionError:
    print(f"Error: Cannot connect to Redis at {redis_host}:{redis_port}")
    sys.exit(1)

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
    
    # Try to get URLs from environment variable first
    urls_json = os.environ.get('URLS_JSON')
    urls = []
    
    if urls_json:
        try:
            urls = json.loads(urls_json)
            print(f"[LIST] Loaded {len(urls)} URLs from environment")
        except json.JSONDecodeError:
            print("[LIST] Failed to parse URLS_JSON")
            urls = []
    
    if not urls:
        # No URLs provided - fail the step
        raise ValueError("[LIST] No URLs provided. Please create urls.txt with URLs to process.")
    
    # Store URLs in workflow state
    set_workflow_state("urls", urls)
    set_workflow_state("total_items", len(urls))
    
    print(f"[LIST] Generated {len(urls)} URLs to process")
    print(f"[LIST] URLs: {json.dumps(urls, indent=2)}")
    return urls

def extract_text_from_html(html_content):
    """Extract clean text from HTML content"""
    try:
        soup = BeautifulSoup(html_content, 'html.parser')
        
        # Remove script and style elements
        for script in soup(["script", "style"]):
            script.decompose()
        
        # Get text
        text = soup.get_text()
        
        # Break into lines and remove leading/trailing space
        lines = (line.strip() for line in text.splitlines())
        # Break multi-headlines into a line each
        chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
        # Drop blank lines
        text = ' '.join(chunk for chunk in chunks if chunk)
        
        return text
    except:
        # Fallback to simple regex if BeautifulSoup fails
        text = re.sub(r'<[^>]+>', ' ', html_content)
        return text

def extract_words(text):
    """Extract and clean words from text"""
    # Convert to lowercase and extract words
    words = re.findall(r'\b[a-z]+\b', text.lower())
    # Filter out very short words and common stop words
    stop_words = {'the', 'a', 'an', 'and', 'or', 'but', 'in', 'on', 'at', 'to', 'for', 'of', 'with', 'by', 'is', 'was', 'are', 'were'}
    words = [w for w in words if len(w) > 2 and w not in stop_words]
    return words

def get_task_data():
    """Get task data from Redis"""
    task_key = f"task:{TASK_ID}"
    task_data = redis_client.get(task_key)
    if not task_data:
        print(f"[MAP] No task data found for {TASK_ID}")
        return {}
    return json.loads(task_data)

def map_phase():
    """Map phase - process individual items"""
    print(f"[MAP] Starting map phase for task {TASK_ID}")
    
    # Get task data from Redis
    task_data = get_task_data()
    workflow_state = task_data.get("input", {})
    
    # Get the current URL from map input (set by Agentainer via item_alias)
    url = workflow_state.get("current_url")
    
    if not url:
        # Try getting from workflow state as fallback
        url = get_workflow_state("current_url")
    
    if not url:
        print(f"[MAP] No current_url found in task data or state")
        print(f"[MAP] Task data: {json.dumps(task_data, indent=2)}")
        raise ValueError("No URL provided for map task")
    
    print(f"[MAP] Processing URL: {url}")
    
    try:
        # Check if this is a retry
        retry_marker_key = f"workflow:{WORKFLOW_ID}:retry:{TASK_ID}"
        retry_count = int(redis_client.get(retry_marker_key) or 0)
        
        if retry_count > 0:
            print(f"[MAP] This is retry attempt #{retry_count} for {url}")
        
        # Increment retry counter
        redis_client.incr(retry_marker_key)
        redis_client.expire(retry_marker_key, 3600)  # Expire after 1 hour
        
        # Fetch the URL with headers
        headers = {
            'User-Agent': 'Mozilla/5.0 (compatible; Agentainer-MapReduce/1.0)'
        }
        
        # Use shorter timeout for problematic URLs
        timeout = 5 if 'delay' in url else 10
        
        response = requests.get(url, timeout=timeout, headers=headers)
        
        # Handle specific status codes
        if response.status_code == 429:
            # Rate limited - this should trigger retry
            print(f"[MAP] Rate limited (429) for {url} - will retry")
            raise requests.exceptions.HTTPError("Rate limited - retry needed")
        elif response.status_code >= 500:
            # Server error - might be transient
            print(f"[MAP] Server error ({response.status_code}) for {url}")
            # On first attempt, fail to trigger retry
            if retry_count == 1:
                raise requests.exceptions.HTTPError(f"Server error {response.status_code}")
            # On retry, sometimes succeed (simulate transient error)
            print(f"[MAP] Simulating recovery on retry for {url}")
        
        response.raise_for_status()
        
        # Determine content type
        content_type = response.headers.get('content-type', '').lower()
        
        # Extract text based on content type
        if 'html' in content_type:
            text = extract_text_from_html(response.text)
        elif 'json' in content_type:
            # For JSON, convert to string representation
            text = json.dumps(response.json(), indent=2)
        else:
            # For other content types, use raw text
            text = response.text
        
        # Extract and count words
        words = extract_words(text)
        word_count = len(words)
        
        # Get top 20 most common words
        word_freq = Counter(words)
        top_words = dict(word_freq.most_common(20))
        
        # Store result
        result = {
            "task_id": TASK_ID,
            "url": url,
            "word_count": word_count,
            "unique_words": len(set(words)),
            "top_words": top_words,
            "status_code": response.status_code,
            "content_type": content_type,
            "content_length": len(response.content),
            "response_time": response.elapsed.total_seconds(),
            "timestamp": time.time()
        }
        
        # Append to results list
        append_to_list("map_results", result)
        
        print(f"[MAP] Successfully processed {url}:")
        print(f"      - Words: {word_count}")
        print(f"      - Unique words: {len(set(words))}")
        print(f"      - Response time: {response.elapsed.total_seconds():.2f}s")
        print(f"      - Top 5 words: {list(top_words.items())[:5]}")
        
    except requests.RequestException as e:
        print(f"[MAP] Request error for {url}: {str(e)}")
        error_result = {
            "task_id": TASK_ID,
            "url": url,
            "error": str(e),
            "error_type": "request_error",
            "timestamp": time.time()
        }
        append_to_list("map_errors", error_result)
    except Exception as e:
        print(f"[MAP] Processing error for {url}: {str(e)}")
        error_result = {
            "task_id": TASK_ID,
            "url": url,
            "error": str(e),
            "error_type": "processing_error",
            "timestamp": time.time()
        }
        append_to_list("map_errors", error_result)

def write_result(result):
    """Write task result to Redis and publish completion notification"""
    result_key = f"task:{TASK_ID}:result"
    result_channel = f"task:{TASK_ID}:complete"
    
    redis_client.set(result_key, json.dumps(result), ex=3600)  # 1 hour expiry
    redis_client.publish(result_channel, "completed")
    print(f"Result written to {result_key}")

def write_error(error_msg):
    """Write error to Redis and publish error notification"""
    error_key = f"task:{TASK_ID}:error"
    result_channel = f"task:{TASK_ID}:complete"
    
    redis_client.set(error_key, error_msg, ex=3600)
    redis_client.publish(result_channel, "error")
    print(f"Error written to {error_key}")

def main():
    """Main entry point"""
    print(f"Starting mapper - Step Type: {STEP_TYPE}")
    
    try:
        if STEP_TYPE == "list":
            urls = list_phase()
            result = {
                "status": "success",
                "urls_prepared": len(urls),
                "urls": urls
            }
            write_result(result)
        elif STEP_TYPE == "map":
            map_phase()
            result = {
                "status": "success",
                "task_id": TASK_ID,
                "url_processed": True
            }
            write_result(result)
        else:
            print(f"Unknown step type: {STEP_TYPE}")
            sys.exit(1)
        
        print("Mapper completed successfully")
        sys.exit(0)
        
    except Exception as e:
        print(f"Error in mapper: {e}")
        write_error(str(e))
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == "__main__":
    main()