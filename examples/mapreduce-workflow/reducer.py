#!/usr/bin/env python3
"""
MapReduce Reducer Example - Aggregate Word Counts

This reducer demonstrates the MapReduce pattern in Agentainer.
It aggregates results from all mapper tasks.
"""

import os
import sys
import json
import redis
from collections import Counter
import time
from datetime import datetime

# Connect to Redis
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
STEP_ID = os.environ.get('STEP_ID', 'reducer')
TASK_ID = os.environ.get('TASK_ID', '0')

def get_list_from_state(key):
    """Get list from workflow state"""
    list_key = f"workflow:{WORKFLOW_ID}:state:list:{key}"
    items = redis_client.lrange(list_key, 0, -1)
    return [json.loads(item) for item in items]

def set_workflow_state(key, value):
    """Set value in workflow state"""
    state_key = f"workflow:{WORKFLOW_ID}:state"
    redis_client.hset(state_key, key, json.dumps(value))

def reduce_phase():
    """Reduce phase - aggregate results from all mappers"""
    print(f"[REDUCE] Starting reduce phase for workflow {WORKFLOW_ID}")
    print(f"[REDUCE] Step ID: {STEP_ID}")
    
    # Get all map results
    map_results = get_list_from_state("map_results")
    map_errors = get_list_from_state("map_errors")
    
    print(f"[REDUCE] Found {len(map_results)} successful results and {len(map_errors)} errors")
    
    if not map_results and not map_errors:
        print("[REDUCE] No results to aggregate")
        # Still create a summary even with no results
        summary = {
            "workflow_id": WORKFLOW_ID,
            "timestamp": datetime.now().isoformat(),
            "error": "No data to process",
            "total_urls_processed": 0,
            "total_urls_failed": 0
        }
        set_workflow_state("final_summary", summary)
        return
    
    # Aggregate statistics
    total_words = sum(r.get('word_count', 0) for r in map_results)
    total_unique_words = sum(r.get('unique_words', 0) for r in map_results)
    total_bytes = sum(r.get('content_length', 0) for r in map_results)
    total_response_time = sum(r.get('response_time', 0) for r in map_results)
    
    # Combine all top words
    combined_words = Counter()
    for result in map_results:
        if 'top_words' in result:
            combined_words.update(result['top_words'])
    
    # Get overall top 30 words
    top_30_words = dict(combined_words.most_common(30))
    
    # Analyze performance
    response_times = [r.get('response_time', 0) for r in map_results]
    min_response_time = min(response_times) if response_times else 0
    max_response_time = max(response_times) if response_times else 0
    avg_response_time = total_response_time / len(map_results) if map_results else 0
    
    # Group errors by type
    error_types = Counter(e.get('error_type', 'unknown') for e in map_errors)
    
    # Create comprehensive summary
    summary = {
        "workflow_id": WORKFLOW_ID,
        "timestamp": datetime.now().isoformat(),
        "total_urls_processed": len(map_results),
        "total_urls_failed": len(map_errors),
        "success_rate": len(map_results) / (len(map_results) + len(map_errors)) * 100 if (map_results or map_errors) else 0,
        "total_words": total_words,
        "total_unique_words": total_unique_words,
        "total_bytes": total_bytes,
        "average_words_per_page": total_words / len(map_results) if map_results else 0,
        "average_unique_words_per_page": total_unique_words / len(map_results) if map_results else 0,
        "average_bytes_per_page": total_bytes / len(map_results) if map_results else 0,
        "performance": {
            "total_response_time": total_response_time,
            "min_response_time": min_response_time,
            "max_response_time": max_response_time,
            "avg_response_time": avg_response_time
        },
        "top_30_words": top_30_words,
        "error_summary": dict(error_types),
        "successful_urls": [r['url'] for r in map_results],
        "failed_urls": [{"url": e['url'], "error": e.get('error', 'Unknown')} for e in map_errors]
    }
    
    # Store final results
    set_workflow_state("final_summary", summary)
    
    # Print detailed summary
    print("\n[REDUCE] ===== WORKFLOW SUMMARY =====")
    print(f"Workflow ID: {summary['workflow_id']}")
    print(f"Completed at: {summary['timestamp']}")
    print(f"\nProcessing Statistics:")
    print(f"  - URLs processed: {summary['total_urls_processed']}")
    print(f"  - URLs failed: {summary['total_urls_failed']}")
    print(f"  - Success rate: {summary['success_rate']:.1f}%")
    print(f"\nContent Analysis:")
    print(f"  - Total words: {summary['total_words']:,}")
    print(f"  - Total unique words: {summary['total_unique_words']:,}")
    print(f"  - Average words/page: {summary['average_words_per_page']:.1f}")
    print(f"  - Average unique words/page: {summary['average_unique_words_per_page']:.1f}")
    print(f"  - Total bytes: {summary['total_bytes']:,}")
    print(f"\nPerformance Metrics:")
    print(f"  - Total response time: {summary['performance']['total_response_time']:.2f}s")
    print(f"  - Average response time: {summary['performance']['avg_response_time']:.2f}s")
    print(f"  - Min response time: {summary['performance']['min_response_time']:.2f}s")
    print(f"  - Max response time: {summary['performance']['max_response_time']:.2f}s")
    
    if summary['error_summary']:
        print(f"\nError Summary:")
        for error_type, count in summary['error_summary'].items():
            print(f"  - {error_type}: {count}")
        
        print(f"\nFailed URLs:")
        for failed in summary['failed_urls'][:5]:  # Show first 5
            print(f"  - {failed['url']}: {failed['error'][:50]}...")
        if len(summary['failed_urls']) > 5:
            print(f"  ... and {len(summary['failed_urls']) - 5} more")
    
    print(f"\nTop 15 words across all pages:")
    for i, (word, count) in enumerate(list(top_30_words.items())[:15], 1):
        print(f"  {i:2d}. {word}: {count}")
    print("==================================\n")
    
    # Output JSON summary for easy parsing
    summary_json = json.dumps(summary, indent=2)
    print("[REDUCE] JSON Summary:")
    print(summary_json)

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
    print("Starting reducer")
    
    try:
        reduce_phase()
        result = {
            "status": "success",
            "task_id": TASK_ID,
            "phase": "reduce",
            "completed": True
        }
        write_result(result)
        print("Reducer completed successfully")
        sys.exit(0)
        
    except Exception as e:
        print(f"Error in reducer: {e}")
        write_error(str(e))
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == "__main__":
    main()