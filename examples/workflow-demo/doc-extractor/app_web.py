#!/usr/bin/env python3
"""
Document Extractor Agent - Web Version
Extracts and chunks content from web pages
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
TASK_TYPE = os.environ.get('TASK_TYPE', 'extract')

print(f"Starting Document Extractor (Web) - Task: {TASK_ID}")

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
        # Try to find main content areas
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
        
        return {
            "url": url,
            "title": title_text,
            "content": text_content,
            "fetch_time": time.time()
        }
        
    except requests.RequestException as e:
        raise Exception(f"Failed to fetch URL {url}: {str(e)}")
    except Exception as e:
        raise Exception(f"Error processing URL {url}: {str(e)}")

def extract_document(task_data, redis_client):
    """Extract content from URL and chunk it"""
    # Get URL from task data
    url = task_data.get("input", {}).get("url")
    if not url:
        # Fallback to sample content if no URL provided
        url = task_data.get("input", {}).get("document_url")
    
    if not url:
        raise ValueError("No URL provided in task input")
    
    print(f"Fetching content from: {url}")
    
    # Fetch web content
    web_data = fetch_web_content(url)
    content = web_data["content"]
    
    # Split content into chunks
    # Simple chunking by paragraphs with size limit
    chunks = []
    current_chunk = []
    current_size = 0
    max_chunk_size = 1500  # characters
    
    paragraphs = content.split('\n\n')
    
    for para in paragraphs:
        para = para.strip()
        if not para:
            continue
            
        para_size = len(para)
        
        # If this paragraph would make chunk too large, save current chunk
        if current_size + para_size > max_chunk_size and current_chunk:
            chunk_text = '\n\n'.join(current_chunk)
            chunk = {
                "chunk_id": f"chunk_{len(chunks)}",
                "text": chunk_text,
                "word_count": len(chunk_text.split()),
                "char_count": len(chunk_text),
                "position": len(chunks),
                "source_url": url
            }
            chunks.append(chunk)
            current_chunk = []
            current_size = 0
        
        current_chunk.append(para)
        current_size += para_size
    
    # Don't forget the last chunk
    if current_chunk:
        chunk_text = '\n\n'.join(current_chunk)
        chunk = {
            "chunk_id": f"chunk_{len(chunks)}",
            "text": chunk_text,
            "word_count": len(chunk_text.split()),
            "char_count": len(chunk_text),
            "position": len(chunks),
            "source_url": url
        }
        chunks.append(chunk)
    
    # Ensure we have at least 3 chunks for the demo
    if len(chunks) < 3:
        # Split the largest chunk
        while len(chunks) < 3 and chunks:
            largest_idx = max(range(len(chunks)), key=lambda i: chunks[i]["char_count"])
            largest = chunks[largest_idx]
            
            if largest["char_count"] > 200:
                text = largest["text"]
                mid = len(text) // 2
                
                # Find nearest paragraph break
                for i in range(50):
                    if mid + i < len(text) and text[mid + i:mid + i + 2] == '\n\n':
                        mid = mid + i
                        break
                    if mid - i > 0 and text[mid - i - 2:mid - i] == '\n\n':
                        mid = mid - i
                        break
                
                # Split the chunk
                text1 = text[:mid].strip()
                text2 = text[mid:].strip()
                
                chunks[largest_idx] = {
                    "chunk_id": f"chunk_{largest_idx}",
                    "text": text1,
                    "word_count": len(text1.split()),
                    "char_count": len(text1),
                    "position": largest_idx,
                    "source_url": url
                }
                
                chunks.insert(largest_idx + 1, {
                    "chunk_id": f"chunk_{largest_idx + 1}",
                    "text": text2,
                    "word_count": len(text2.split()),
                    "char_count": len(text2),
                    "position": largest_idx + 1,
                    "source_url": url
                })
                
                # Update positions
                for i in range(len(chunks)):
                    chunks[i]["chunk_id"] = f"chunk_{i}"
                    chunks[i]["position"] = i
            else:
                break
    
    # Return extraction result
    result = {
        "document_id": web_data["title"],
        "source_url": url,
        "title": web_data["title"],
        "total_chunks": len(chunks),
        "total_words": sum(c["word_count"] for c in chunks),
        "chunks": chunks,
        "extraction_time": time.time(),
        "fetch_time": web_data["fetch_time"],
        "status": "success"
    }
    
    return result, chunks

def prepare_tasks(task_data, redis_client):
    """Prepare extracted chunks for parallel processing"""
    # Get chunk count from workflow state
    state_key = f"workflow:{WORKFLOW_ID}:state"
    chunk_count_data = redis_client.hget(state_key, "chunk_count")
    
    if not chunk_count_data:
        # Fallback: try to get from extraction result
        extraction_data = redis_client.hget(state_key, "extraction_result")
        if extraction_data:
            extraction_result = json.loads(extraction_data)
            chunk_count = extraction_result.get("total_chunks", 0)
        else:
            chunk_count = 3  # Default for demo
    else:
        chunk_count = json.loads(chunk_count_data)
    
    # Prepare tasks for parallel processing
    tasks = []
    for i in range(chunk_count):
        task = {
            "task_id": f"task-chunk_{i}",
            "chunk_id": f"chunk_{i}",
            "task_type": "summarize",
            "priority": 1
        }
        tasks.append(task)
    
    return {
        "status": "success",
        "tasks_prepared": len(tasks),
        "tasks": tasks
    }

def main():
    """Main execution"""
    redis_client = None
    
    try:
        # Connect to Redis
        redis_client = connect_redis()
        
        # Get task data
        task_data = get_task(redis_client)
        print(f"Task type: {TASK_TYPE}")
        
        if TASK_TYPE == "extract":
            # Extract document
            result, chunks = extract_document(task_data, redis_client)
            
            # Save chunks to workflow state
            for chunk in chunks:
                set_workflow_state(redis_client, chunk["chunk_id"], chunk["text"])
            
            # Save extraction result and chunk list
            set_workflow_state(redis_client, "extraction_result", result)
            set_workflow_state(redis_client, "chunk_count", len(chunks))
            set_workflow_state(redis_client, "chunks", [c["chunk_id"] for c in chunks])
            
            write_result(redis_client, result)
            
        elif TASK_TYPE == "prepare":
            # Prepare parallel tasks
            result = prepare_tasks(task_data, redis_client)
            
            # Save task preparation info
            set_workflow_state(redis_client, "task_preparation", result)
            set_workflow_state(redis_client, "task_count", result["tasks_prepared"])
            
            write_result(redis_client, result)
            
        else:
            raise ValueError(f"Unknown task type: {TASK_TYPE}")
        
        print("Task completed successfully")
        sys.exit(0)
        
    except Exception as e:
        print(f"Error: {e}")
        if redis_client:
            write_error(redis_client, str(e))
        sys.exit(1)

if __name__ == "__main__":
    main()