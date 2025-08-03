#!/usr/bin/env python3
"""
Multi-URL Document Extractor Agent
Handles different phases of multi-URL workflow processing
"""

import os
import sys
import json
import time
import redis
import requests
from bs4 import BeautifulSoup
from urllib.parse import urlparse
import html2text

# Redis connection
REDIS_HOST = os.environ.get('REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('REDIS_PORT', 6379))

# Task information from environment
TASK_ID = os.environ.get('TASK_ID', '')
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', '')
STEP_ID = os.environ.get('STEP_ID', '')
TASK_TYPE = os.environ.get('TASK_TYPE', 'prepare_urls')

print(f"Starting Multi-URL Extractor - Task: {TASK_ID}, Type: {TASK_TYPE}")

def connect_redis():
    """Connect to Redis with retry logic"""
    max_retries = 5
    retry_delay = 1
    
    for i in range(max_retries):
        try:
            redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
            redis_client.ping()
            print(f"Connected to Redis at {REDIS_HOST}:{REDIS_PORT}")
            return redis_client
        except Exception as e:
            print(f"Failed to connect to Redis (attempt {i+1}/{max_retries}): {e}")
            if i < max_retries - 1:
                time.sleep(retry_delay)
    
    raise Exception("Could not connect to Redis after all retries")

def get_task(redis_client):
    """Get task data from Redis"""
    task_key = f"task:{TASK_ID}"
    task_data = redis_client.get(task_key)
    if not task_data:
        raise Exception(f"No task data found for {TASK_ID}")
    return json.loads(task_data)

def write_result(redis_client, result):
    """Write task result to Redis and publish completion notification"""
    result_key = f"task:{TASK_ID}:result"
    result_channel = f"task:{TASK_ID}:complete"
    
    redis_client.set(result_key, json.dumps(result), ex=3600)  # 1 hour expiry
    redis_client.publish(result_channel, "completed")
    print(f"Result written to {result_key}")

def write_error(redis_client, error_msg):
    """Write error to Redis and publish error notification"""
    error_key = f"task:{TASK_ID}:error"
    result_channel = f"task:{TASK_ID}:complete"
    
    redis_client.set(error_key, error_msg, ex=3600)
    redis_client.publish(result_channel, "error")
    print(f"Error written to {error_key}")

def get_workflow_state(redis_client, key):
    """Get workflow state from Redis"""
    state_key = f"workflow:{WORKFLOW_ID}:state"
    value = redis_client.hget(state_key, key)
    if value:
        try:
            return json.loads(value)
        except:
            return value
    return None

def set_workflow_state(redis_client, key, value):
    """Set workflow state in Redis"""
    state_key = f"workflow:{WORKFLOW_ID}:state"
    redis_client.hset(state_key, key, json.dumps(value))

def fetch_web_content(url):
    """Fetch and extract text content from a web page"""
    headers = {
        'User-Agent': 'Mozilla/5.0 (compatible; Agentainer/1.0; +https://github.com/agentainer)'
    }
    
    try:
        # Validate URL
        parsed = urlparse(url)
        if not parsed.scheme or not parsed.netloc:
            raise ValueError(f"Invalid URL: {url}")
        
        # Fetch the page
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()
        
        # Parse HTML
        soup = BeautifulSoup(response.text, 'html.parser')
        
        # Remove script and style elements
        for script in soup(["script", "style"]):
            script.decompose()
        
        # Extract title
        title = soup.find('title')
        title_text = title.get_text(strip=True) if title else "Untitled"
        
        # Convert to markdown for better structure preservation
        h = html2text.HTML2Text()
        h.ignore_links = False
        h.ignore_images = True
        h.body_width = 0  # Don't wrap lines
        
        # Get the main content
        main_content = None
        for selector in ['main', 'article', '[role="main"]', '.content', '#content']:
            main_content = soup.select_one(selector)
            if main_content:
                break
        
        if not main_content:
            main_content = soup.find('body')
        
        if main_content:
            text_content = h.handle(str(main_content))
        else:
            text_content = h.handle(response.text)
        
        # Limit content size for processing
        max_chars = 5000
        if len(text_content) > max_chars:
            text_content = text_content[:max_chars] + "\n\n[Content truncated...]"
        
        return {
            "url": url,
            "title": title_text,
            "content": text_content,
            "word_count": len(text_content.split()),
            "fetch_time": time.time(),
            "domain": parsed.netloc
        }
        
    except requests.RequestException as e:
        return {
            "url": url,
            "error": f"Failed to fetch: {str(e)}",
            "fetch_time": time.time()
        }
    except Exception as e:
        return {
            "url": url,
            "error": f"Error processing: {str(e)}",
            "fetch_time": time.time()
        }

def prepare_urls(task_data, redis_client):
    """
    STEP 1: Prepare URLs for processing
    This demonstrates how to prepare data for the MAP phase
    """
    # Get URLs from task input - the input is the entire workflow state
    input_data = task_data.get("input", {})
    urls_input = input_data.get("urls", [])
    
    # Always try to read from the URLs file since workflow state may be empty initially
    try:
        with open("/app/urls.txt", "r") as f:
            urls_input = [line.strip() for line in f if line.strip() and not line.startswith("#")]
        print(f"Loaded {len(urls_input)} URLs from file")
    except FileNotFoundError:
        print("urls.txt not found in container, checking workflow state")
        if not urls_input:
            urls_input = []
    
    if not urls_input:
        raise ValueError("No URLs provided for processing")
    
    # Validate and prepare URLs
    valid_urls = []
    for url in urls_input[:10]:  # Limit to 10 URLs
        if url.startswith("http://") or url.startswith("https://"):
            valid_urls.append({
                "url": url,
                "url_id": f"url_{len(valid_urls)}",
                "status": "pending"
            })
    
    # Store URLs in workflow state for map phase
    set_workflow_state(redis_client, "urls", valid_urls)
    set_workflow_state(redis_client, "url_count", len(valid_urls))
    
    # Prepare the result
    result = {
        "status": "success",
        "urls_prepared": len(valid_urls),
        "urls": valid_urls,
        "preparation_time": time.time()
    }
    
    print(f"Prepared {len(valid_urls)} URLs for processing")
    return result

def process_single_url(task_data, redis_client):
    """
    STEP 2 (MAP): Process a single URL
    This is called in parallel for each URL
    """
    # With the new map step, each task gets the current item
    workflow_state = task_data.get("input", {})
    
    # Get the current URL from the map input
    current_url = workflow_state.get("current_url")
    if current_url:
        # New map step format
        if isinstance(current_url, dict):
            url = current_url.get("url")
            url_id = current_url.get("url_id", "unknown")
        else:
            # Direct URL string
            url = current_url
            url_id = f"url_{workflow_state.get('_map_index', 0)}"
    else:
        # Fallback to old method for testing
        urls = get_workflow_state(redis_client, "urls") or []
        worker_id = int(os.environ.get('WORKER_ID', workflow_state.get('_map_index', 0)))
        
        if worker_id >= len(urls):
            return {"status": "skipped", "message": f"No URL for worker {worker_id}"}
        
        url_info = urls[worker_id]
        url = url_info.get("url")
        url_id = url_info.get("url_id", f"url_{worker_id}")
    
    if not url:
        raise ValueError("No URL provided in task input")
    
    print(f"Processing URL: {url}")
    
    # Fetch and analyze the web content
    web_data = fetch_web_content(url)
    
    # Basic content analysis
    if "error" not in web_data:
        content = web_data["content"]
        
        # Simple analysis (you can make this more sophisticated)
        analysis = {
            "url": url,
            "url_id": url_id,
            "title": web_data["title"],
            "domain": web_data["domain"],
            "word_count": web_data["word_count"],
            "fetch_time": web_data["fetch_time"],
            "summary": content[:500] + "..." if len(content) > 500 else content,
            "status": "success"
        }
        
        # Store the full content for later stages
        set_workflow_state(redis_client, f"content_{url_id}", content)
        
    else:
        analysis = {
            "url": url,
            "url_id": url_id,
            "error": web_data["error"],
            "status": "failed"
        }
    
    # Store individual result
    set_workflow_state(redis_client, f"result_{url_id}", analysis)
    
    return analysis

def aggregate_results(task_data, redis_client):
    """
    STEP 3 (REDUCE): Aggregate results from all URLs
    This demonstrates the REDUCE phase of the workflow
    """
    # Get the number of URLs from workflow state
    url_count = get_workflow_state(redis_client, "url_count") or 0
    urls = get_workflow_state(redis_client, "urls") or []
    
    # Collect all individual results
    all_results = []
    successful = 0
    failed = 0
    total_words = 0
    domains = set()
    
    for i in range(url_count):
        url_id = f"url_{i}"
        result = get_workflow_state(redis_client, f"result_{url_id}")
        
        if result:
            all_results.append(result)
            if result.get("status") == "success":
                successful += 1
                total_words += result.get("word_count", 0)
                domains.add(result.get("domain", "unknown"))
            else:
                failed += 1
    
    # Create aggregated summary
    aggregated = {
        "total_urls": url_count,
        "successful": successful,
        "failed": failed,
        "total_words_analyzed": total_words,
        "unique_domains": list(domains),
        "average_words_per_article": total_words / successful if successful > 0 else 0,
        "processing_time": time.time(),
        "individual_results": all_results
    }
    
    # Store aggregated results
    set_workflow_state(redis_client, "aggregated_results", aggregated)
    
    print(f"Aggregated results: {successful} successful, {failed} failed out of {url_count} URLs")
    return aggregated

def main():
    """Main execution"""
    redis_client = None
    
    try:
        # Connect to Redis
        redis_client = connect_redis()
        
        # Get task data
        task_data = get_task(redis_client)
        print(f"Task type: {TASK_TYPE}")
        
        # Route to appropriate handler based on task type
        if TASK_TYPE == "prepare_urls":
            # STEP 1: Prepare URLs for processing
            result = prepare_urls(task_data, redis_client)
            
        elif TASK_TYPE == "process_url":
            # STEP 2: Process a single URL (MAP phase)
            result = process_single_url(task_data, redis_client)
            
        elif TASK_TYPE == "aggregate":
            # STEP 3: Aggregate all results (REDUCE phase)
            result = aggregate_results(task_data, redis_client)
            
        else:
            raise ValueError(f"Unknown task type: {TASK_TYPE}")
        
        # Write result
        write_result(redis_client, result)
        print("Task completed successfully")
        sys.exit(0)
        
    except Exception as e:
        print(f"Error: {e}")
        if redis_client:
            write_error(redis_client, str(e))
        sys.exit(1)

if __name__ == "__main__":
    main()