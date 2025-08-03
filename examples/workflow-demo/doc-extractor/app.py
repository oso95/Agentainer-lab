#!/usr/bin/env python3
"""
Document Extractor Agent - Task-based implementation
Reads task from Redis, processes it, and writes result back
"""

import os
import json
import time
import sys
import redis

# Redis connection
REDIS_HOST = os.environ.get('REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('REDIS_PORT', 6379))

# Task information from environment
TASK_ID = os.environ.get('TASK_ID', '')
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', '')
STEP_ID = os.environ.get('STEP_ID', '')
TASK_TYPE = os.environ.get('TASK_TYPE', 'extract')

def connect_redis():
    """Connect to Redis with retry logic"""
    max_retries = 10
    retry_delay = 1
    
    for i in range(max_retries):
        try:
            client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
            client.ping()
            print(f"Connected to Redis at {REDIS_HOST}:{REDIS_PORT}")
            return client
        except Exception as e:
            if i < max_retries - 1:
                print(f"Redis connection failed (attempt {i+1}/{max_retries}): {e}")
                time.sleep(retry_delay)
            else:
                raise Exception(f"Failed to connect to Redis after {max_retries} attempts: {e}")

def get_task(redis_client):
    """Get task details from Redis"""
    if not TASK_ID:
        raise Exception("No TASK_ID provided in environment")
    
    task_key = f"task:{TASK_ID}"
    task_data = redis_client.get(task_key)
    
    if not task_data:
        raise Exception(f"Task {TASK_ID} not found in Redis")
    
    return json.loads(task_data)

def write_result(redis_client, result):
    """Write task result to Redis and publish completion notification"""
    result_key = f"task:{TASK_ID}:result"
    redis_client.set(result_key, json.dumps(result), ex=3600)  # 1 hour expiry
    print(f"Wrote result to {result_key}")
    
    # Publish completion notification
    completion_channel = f"task:{TASK_ID}:complete"
    redis_client.publish(completion_channel, "completed")
    print(f"Published completion notification to {completion_channel}")

def write_error(redis_client, error_msg):
    """Write task error to Redis and publish error notification"""
    error_key = f"task:{TASK_ID}:error"
    redis_client.set(error_key, error_msg, ex=3600)
    print(f"Wrote error to {error_key}")
    
    # Publish error notification
    completion_channel = f"task:{TASK_ID}:complete"
    redis_client.publish(completion_channel, "error")
    print(f"Published error notification to {completion_channel}")

def set_workflow_state(redis_client, key, value):
    """Set value in workflow state"""
    if not WORKFLOW_ID:
        return
    state_key = f"workflow:{WORKFLOW_ID}:state"
    redis_client.hset(state_key, key, json.dumps(value))

def extract_document(task_data):
    """
    Extract and chunk document for processing
    In a real implementation, this would handle PDFs, DOCX, etc.
    """
    # For demo purposes, we'll simulate document extraction
    sample_document = """
    Executive Summary:
    This quarterly report analyzes market trends and company performance across multiple sectors. 
    Our analysis shows significant growth in the technology sector, with AI and cloud services 
    leading the expansion. However, traditional manufacturing faces challenges due to supply chain 
    disruptions and rising material costs.
    
    Market Analysis:
    The global technology market experienced unprecedented growth this quarter, driven primarily 
    by artificial intelligence adoption and cloud infrastructure expansion. Major tech companies 
    reported average revenue increases of 23%, significantly outperforming market expectations.
    Enterprise software solutions saw particularly strong demand as businesses accelerated their 
    digital transformation initiatives.
    
    Regional Performance:
    North American markets led growth with a 15% increase in overall activity. The Asia-Pacific 
    region showed resilience despite geopolitical tensions, maintaining steady growth at 8%. 
    European markets faced headwinds from energy costs but managed modest gains of 3%. Emerging 
    markets in Latin America and Africa demonstrated strong potential with 12% growth.
    """
    
    # Split into chunks
    sections = sample_document.strip().split('\n\n')
    chunks = []
    
    for i, section in enumerate(sections):
        if section.strip():
            chunk = {
                "chunk_id": f"chunk_{i}",
                "content": section.strip(),
                "word_count": len(section.split()),
                "position": i,
                "total_chunks": len(sections)
            }
            chunks.append(chunk)
    
    # Return extraction result
    result = {
        "document_id": task_data.get("input", {}).get("document_id", "sample_doc"),
        "total_chunks": len(chunks),
        "total_words": sum(c["word_count"] for c in chunks),
        "chunks": chunks,
        "extraction_time": time.time(),
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
    """Main execution function"""
    print(f"Document Extractor Agent starting...")
    print(f"Task ID: {TASK_ID}")
    print(f"Workflow ID: {WORKFLOW_ID}")
    print(f"Task Type: {TASK_TYPE}")
    
    try:
        # Connect to Redis
        redis_client = connect_redis()
        
        # Get task details
        task_data = get_task(redis_client)
        print(f"Retrieved task: {task_data.get('task_id')}")
        
        # Execute based on task type
        if TASK_TYPE == "extract":
            result, chunks = extract_document(task_data)
            
            # Store chunks in workflow state
            for i, chunk in enumerate(chunks):
                set_workflow_state(redis_client, f"chunk_{i}", chunk["content"])
            
            # Store metadata
            set_workflow_state(redis_client, "extraction_result", result)
            set_workflow_state(redis_client, "chunk_count", len(chunks))
            set_workflow_state(redis_client, "chunks", [c["chunk_id"] for c in chunks])
            
            write_result(redis_client, result)
            
        elif TASK_TYPE == "prepare":
            result = prepare_tasks(task_data, redis_client)
            
            # Store task list for mapreduce
            set_workflow_state(redis_client, "tasks", result["tasks"])
            set_workflow_state(redis_client, "task_count", result["tasks_prepared"])
            
            write_result(redis_client, result)
            
        else:
            raise Exception(f"Unknown task type: {TASK_TYPE}")
        
        print(f"Task completed successfully")
        sys.exit(0)
        
    except Exception as e:
        print(f"Error executing task: {e}")
        try:
            redis_client = connect_redis()
            write_error(redis_client, str(e))
        except:
            pass
        sys.exit(1)

if __name__ == '__main__':
    main()