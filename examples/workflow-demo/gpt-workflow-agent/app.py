#!/usr/bin/env python3
"""
GPT Workflow Agent - Task-based implementation
Reads task from Redis, processes it with GPT, and writes result back
"""

import os
import json
import time
import sys
import redis
from openai import OpenAI
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Initialize OpenAI client
client = OpenAI(api_key=os.getenv('OPENAI_API_KEY'))

# Redis connection
REDIS_HOST = os.environ.get('REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('REDIS_PORT', 6379))

# Task information from environment
TASK_ID = os.environ.get('TASK_ID', '')
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', '')
STEP_ID = os.environ.get('STEP_ID', '')
TASK_TYPE = os.environ.get('TASK_TYPE', 'summarize')
WORKER_ID = os.environ.get('WORKER_ID', '0')

def connect_redis():
    """Connect to Redis with retry logic"""
    max_retries = 10
    retry_delay = 1
    
    for i in range(max_retries):
        try:
            redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
            redis_client.ping()
            print(f"Connected to Redis at {REDIS_HOST}:{REDIS_PORT}")
            return redis_client
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

def get_workflow_state(redis_client, key):
    """Get value from workflow state"""
    if not WORKFLOW_ID:
        return None
    state_key = f"workflow:{WORKFLOW_ID}:state"
    value = redis_client.hget(state_key, key)
    if value:
        return json.loads(value)
    return None

def set_workflow_state(redis_client, key, value):
    """Set value in workflow state"""
    if not WORKFLOW_ID:
        return
    state_key = f"workflow:{WORKFLOW_ID}:state"
    redis_client.hset(state_key, key, json.dumps(value))

def summarize_chunk(task_data, redis_client):
    """Summarize a text chunk using GPT"""
    # Get chunk content - worker ID determines which chunk
    chunk_id = f"chunk_{WORKER_ID}"
    text = get_workflow_state(redis_client, chunk_id)
    
    if not text:
        # Try to get from task input
        text = task_data.get('input', {}).get('text', '')
    
    if not text:
        raise Exception(f"No text found for chunk {chunk_id}")
    
    max_words = int(os.environ.get('MAX_WORDS', 150))
    
    # Call GPT for summarization
    response = client.chat.completions.create(
        model=os.getenv('OPENAI_MODEL', 'gpt-3.5-turbo'),
        messages=[
            {"role": "system", "content": "You are a helpful assistant that provides clear, concise summaries."},
            {"role": "user", "content": f"Summarize the following text in no more than {max_words} words:\n\n{text}"}
        ],
        max_tokens=max_words * 2,
        temperature=0.3
    )
    
    summary = response.choices[0].message.content.strip()
    
    result = {
        "chunk_id": chunk_id,
        "worker_id": WORKER_ID,
        "summary": summary,
        "original_length": len(text),
        "summary_length": len(summary),
        "model": response.model,
        "timestamp": time.time()
    }
    
    # Save summary to workflow state
    set_workflow_state(redis_client, f"summary_{chunk_id}", result)
    
    return result

def analyze_text(task_data, redis_client):
    """Analyze text using GPT"""
    text = task_data.get('input', {}).get('text', '')
    analysis_type = task_data.get('input', {}).get('analysis_type', 'general')
    
    if not text:
        # Try to get all summaries for analysis
        summaries = []
        for i in range(10):  # Check up to 10 chunks
            summary_data = get_workflow_state(redis_client, f"summary_chunk_{i}")
            if summary_data:
                summaries.append(summary_data.get('summary', ''))
        
        if summaries:
            text = "\n\n".join(summaries)
        else:
            raise Exception("No text provided for analysis")
    
    prompts = {
        "sentiment": "Analyze the sentiment of this text. Return: positive, negative, or neutral.",
        "key_points": "Extract the 3 most important points from this text.",
        "entities": "Identify all named entities (people, places, organizations) in this text.",
        "general": "Provide a brief analysis of this text including main topic and tone."
    }
    
    prompt = prompts.get(analysis_type, prompts['general'])
    
    response = client.chat.completions.create(
        model=os.getenv('OPENAI_MODEL', 'gpt-3.5-turbo'),
        messages=[
            {"role": "system", "content": "You are an expert text analyst."},
            {"role": "user", "content": f"{prompt}\n\nText: {text}"}
        ],
        temperature=0.2
    )
    
    analysis = response.choices[0].message.content.strip()
    
    result = {
        "analysis_type": analysis_type,
        "analysis": analysis,
        "text_length": len(text),
        "timestamp": time.time()
    }
    
    # Save analysis to workflow state
    set_workflow_state(redis_client, f"analysis_{analysis_type}", result)
    
    return result

def main():
    """Main execution function"""
    print(f"GPT Workflow Agent starting...")
    print(f"Task ID: {TASK_ID}")
    print(f"Workflow ID: {WORKFLOW_ID}")
    print(f"Task Type: {TASK_TYPE}")
    print(f"Worker ID: {WORKER_ID}")
    
    try:
        # Connect to Redis
        redis_client = connect_redis()
        
        # Get task details
        task_data = get_task(redis_client)
        print(f"Retrieved task: {task_data.get('task_id')}")
        
        # Execute based on task type
        if TASK_TYPE == "summarize":
            result = summarize_chunk(task_data, redis_client)
        elif TASK_TYPE == "analyze":
            result = analyze_text(task_data, redis_client)
        else:
            raise Exception(f"Unknown task type: {TASK_TYPE}")
        
        # Write result
        write_result(redis_client, result)
        
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