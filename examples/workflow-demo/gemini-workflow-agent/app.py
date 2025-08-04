#!/usr/bin/env python3
"""
Gemini Workflow Agent - Task-based implementation
Reads task from Redis, processes it with Gemini, and writes result back
"""

import os
import json
import time
import sys
import redis
import google.generativeai as genai
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Configure Gemini
genai.configure(api_key=os.getenv('GEMINI_API_KEY'))
model = genai.GenerativeModel(os.getenv('GEMINI_MODEL', 'gemini-pro'))

# Redis connection
REDIS_HOST = os.environ.get('REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('REDIS_PORT', 6379))

# Task information from environment
TASK_ID = os.environ.get('TASK_ID', '')
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', '')
STEP_ID = os.environ.get('STEP_ID', '')
TASK_TYPE = os.environ.get('TASK_TYPE', 'entity_extraction')

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

def extract_entities(task_data, redis_client):
    """Extract entities from text using Gemini"""
    # Get text from workflow state
    use_summaries = os.environ.get('USE_SUMMARIES', 'false').lower() == 'true'
    
    if use_summaries:
        # Get all summaries
        summaries = []
        for i in range(10):  # Check up to 10 summaries
            summary_data = get_workflow_state(redis_client, f"summary_chunk_{i}")
            if summary_data:
                summaries.append(summary_data.get('summary', ''))
        
        if summaries:
            text = "\n\n".join(summaries)
        else:
            # Fallback to original chunks
            chunks = []
            for i in range(10):
                chunk_text = get_workflow_state(redis_client, f"chunk_{i}")
                if chunk_text:
                    chunks.append(chunk_text)
            text = "\n\n".join(chunks) if chunks else ""
    else:
        text = task_data.get('input', {}).get('text', '')
    
    if not text:
        raise Exception("No text provided for entity extraction")
    
    prompt = f"""Extract all named entities from the following text. 
    Categorize them as:
    - People
    - Organizations
    - Locations
    - Products
    - Events
    - Dates
    
    Return the results in JSON format.
    
    Text: {text}"""
    
    response = model.generate_content(prompt)
    
    # Parse Gemini's response - it should return JSON
    try:
        # Gemini might return markdown code blocks, so extract JSON
        response_text = response.text
        if "```json" in response_text:
            response_text = response_text.split("```json")[1].split("```")[0].strip()
        elif "```" in response_text:
            response_text = response_text.split("```")[1].split("```")[0].strip()
        
        entities = json.loads(response_text)
    except:
        # Fallback to text response
        entities = {"raw_response": response.text}
    
    result = {
        "entities": entities,
        "text_length": len(text),
        "extraction_method": "gemini",
        "used_summaries": use_summaries,
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "extracted_entities", result)
    
    return result

def cross_reference(task_data, redis_client):
    """Cross-reference entities with summaries"""
    # Get entities
    entities_data = get_workflow_state(redis_client, "extracted_entities")
    if not entities_data:
        raise Exception("No entities found in workflow state")
    
    entities = entities_data.get("entities", {})
    
    # Get summaries
    summaries = []
    for i in range(10):
        summary_data = get_workflow_state(redis_client, f"summary_chunk_{i}")
        if summary_data:
            summaries.append({
                "chunk_id": f"chunk_{i}",
                "summary": summary_data.get('summary', '')
            })
    
    prompt = f"""Given the following entities and document summaries, create a cross-reference 
    showing which entities appear in which sections of the document.
    
    Entities: {json.dumps(entities)}
    
    Summaries: {json.dumps(summaries)}
    
    Return a structured analysis showing entity distribution across document sections."""
    
    response = model.generate_content(prompt)
    
    result = {
        "cross_reference": response.text,
        "entity_count": sum(len(v) if isinstance(v, list) else 1 for v in entities.values()),
        "section_count": len(summaries),
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "cross_reference", result)
    
    return result

def generate_insights(task_data, redis_client):
    """Generate insights from the analysis"""
    # Gather all analysis data
    extraction_data = get_workflow_state(redis_client, "extraction_result") or {}
    entities_data = get_workflow_state(redis_client, "extracted_entities") or {}
    cross_ref_data = get_workflow_state(redis_client, "cross_reference") or {}
    
    prompt = f"""Based on the following document analysis, generate key insights:
    
    Document Overview:
    - Total chunks: {extraction_data.get('total_chunks', 0)}
    - Total words: {extraction_data.get('total_words', 0)}
    
    Entities Found: {json.dumps(entities_data.get('entities', {}))}
    
    Cross-Reference Analysis: {cross_ref_data.get('cross_reference', 'Not available')}
    
    Please provide:
    1. Key themes identified
    2. Notable patterns in entity distribution
    3. Important relationships between entities
    4. Strategic insights for decision-making
    """
    
    response = model.generate_content(prompt)
    
    result = {
        "insights": response.text,
        "analysis_depth": "comprehensive",
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "insights", result)
    
    return result

def aggregate_analysis(task_data, redis_client):
    """Create final aggregated analysis"""
    # Gather all components
    extraction_data = get_workflow_state(redis_client, "extraction_result") or {}
    entities_data = get_workflow_state(redis_client, "extracted_entities") or {}
    insights_data = get_workflow_state(redis_client, "insights") or {}
    
    # Get analysis focus
    focus = os.environ.get('ANALYSIS_FOCUS', 'comprehensive')
    
    if focus == "executive":
        prompt = f"""Create an executive summary based on the following analysis:
        
        Document Stats: {extraction_data.get('total_words', 0)} words in {extraction_data.get('total_chunks', 0)} sections
        
        Key Entities: {json.dumps(entities_data.get('entities', {}))}
        
        Insights: {insights_data.get('insights', 'Not available')}
        
        Create a concise executive summary (max 300 words) highlighting:
        1. Main findings
        2. Critical entities and relationships
        3. Actionable recommendations
        """
    else:
        prompt = f"""Create a comprehensive final analysis combining all findings.
        Include document overview, entity analysis, and strategic insights."""
    
    response = model.generate_content(prompt)
    
    result = {
        "final_analysis": response.text,
        "analysis_type": focus,
        "components_analyzed": ["extraction", "entities", "insights"],
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "final_analysis", result)
    
    return result

def extract_entities_multi(task_data, redis_client):
    """Extract entities from multiple URL results"""
    # Get aggregated results from workflow state
    aggregated_data = get_workflow_state(redis_client, "aggregated_results") or {}
    individual_results = aggregated_data.get("individual_results", [])
    
    if not individual_results:
        raise ValueError("No aggregated results found for entity extraction")
    
    # Collect all content
    all_content = []
    for result in individual_results:
        if result.get("status") == "success":
            url_id = result.get("url_id")
            content = get_workflow_state(redis_client, f"content_{url_id}")
            if content:
                all_content.append({
                    "url": result.get("url"),
                    "title": result.get("title"),
                    "content": content[:2000]  # Limit content per article
                })
    
    if not all_content:
        raise ValueError("No text provided for entity extraction")
    
    prompt = f"""Analyze the following {len(all_content)} articles and extract key entities:
    
    {json.dumps(all_content, indent=2)}
    
    Please identify and categorize:
    1. People (names, titles, roles)
    2. Organizations (companies, institutions)
    3. Technologies (AI models, software, platforms)
    4. Key concepts and themes
    5. Dates and events
    
    Return a structured analysis showing entities across all articles."""
    
    response = model.generate_content(prompt)
    
    result = {
        "entities": response.text,
        "articles_analyzed": len(all_content),
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "extracted_entities_multi", result)
    
    return result

def final_report_multi(task_data, redis_client):
    """Generate comprehensive report from multi-URL analysis"""
    # Gather all components
    aggregated_data = get_workflow_state(redis_client, "aggregated_results") or {}
    entities_data = get_workflow_state(redis_client, "extracted_entities_multi") or {}
    insights_data = get_workflow_state(redis_client, "insights_multi") or {}
    
    prompt = f"""Create a comprehensive report based on the analysis of {aggregated_data.get('total_urls', 0)} articles:
    
    Summary Statistics:
    - Articles analyzed: {aggregated_data.get('successful', 0)} successful, {aggregated_data.get('failed', 0)} failed
    - Total words analyzed: {aggregated_data.get('total_words_analyzed', 0)}
    - Domains covered: {', '.join(aggregated_data.get('unique_domains', []))}
    
    Entities Extracted:
    {entities_data.get('entities', 'Not available')}
    
    Cross-Article Insights:
    {insights_data.get('insights', 'Not available')}
    
    Please create a comprehensive report that includes:
    1. Executive summary
    2. Key findings across all articles
    3. Common themes and patterns
    4. Notable differences or contradictions
    5. Strategic recommendations
    6. Areas for further investigation
    """
    
    response = model.generate_content(prompt)
    
    result = {
        "final_report": response.text,
        "report_type": "multi_url_analysis",
        "articles_included": aggregated_data.get('successful', 0),
        "timestamp": time.time()
    }
    
    # Save to workflow state
    set_workflow_state(redis_client, "final_report_multi", result)
    
    return result

def main():
    """Main execution function"""
    print(f"Gemini Workflow Agent starting...")
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
        if TASK_TYPE == "entity_extraction":
            result = extract_entities(task_data, redis_client)
        elif TASK_TYPE == "cross_reference":
            result = cross_reference(task_data, redis_client)
        elif TASK_TYPE == "insight_generation":
            result = generate_insights(task_data, redis_client)
        elif TASK_TYPE == "aggregate_analysis":
            result = aggregate_analysis(task_data, redis_client)
        elif TASK_TYPE == "extract_entities_multi":
            result = extract_entities_multi(task_data, redis_client)
        elif TASK_TYPE == "final_report_multi":
            result = final_report_multi(task_data, redis_client)
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