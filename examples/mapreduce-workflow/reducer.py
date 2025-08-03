#!/usr/bin/env python3
"""
MapReduce Reducer Example - Aggregate Word Counts

This reducer demonstrates the MapReduce pattern in Agentainer Flow.
It aggregates results from all mapper tasks.
"""

import os
import json
import redis
from collections import Counter

# Connect to Redis
redis_client = redis.Redis(host=os.environ.get('REDIS_HOST', 'redis'), port=6379, decode_responses=True)

# Get workflow metadata from environment
WORKFLOW_ID = os.environ.get('WORKFLOW_ID', 'test-workflow')
STEP_ID = os.environ.get('STEP_ID', 'reducer')

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
    
    # Get all map results
    map_results = get_list_from_state("map_results")
    map_errors = get_list_from_state("map_errors")
    
    print(f"[REDUCE] Found {len(map_results)} successful results and {len(map_errors)} errors")
    
    if not map_results:
        print("[REDUCE] No results to aggregate")
        return
    
    # Aggregate statistics
    total_words = sum(r.get('word_count', 0) for r in map_results)
    total_bytes = sum(r.get('content_length', 0) for r in map_results)
    
    # Combine all top words
    combined_words = Counter()
    for result in map_results:
        if 'top_words' in result:
            combined_words.update(result['top_words'])
    
    # Get overall top 20 words
    top_20_words = dict(combined_words.most_common(20))
    
    # Create summary
    summary = {
        "total_urls_processed": len(map_results),
        "total_urls_failed": len(map_errors),
        "total_words": total_words,
        "total_bytes": total_bytes,
        "average_words_per_page": total_words / len(map_results) if map_results else 0,
        "top_20_words": top_20_words,
        "successful_urls": [r['url'] for r in map_results],
        "failed_urls": [e['url'] for e in map_errors]
    }
    
    # Store final results
    set_workflow_state("final_summary", summary)
    
    # Print summary
    print("\n[REDUCE] === FINAL SUMMARY ===")
    print(f"Total URLs processed: {summary['total_urls_processed']}")
    print(f"Total URLs failed: {summary['total_urls_failed']}")
    print(f"Total words counted: {summary['total_words']}")
    print(f"Average words per page: {summary['average_words_per_page']:.2f}")
    print(f"\nTop 10 words across all pages:")
    for word, count in list(top_20_words.items())[:10]:
        print(f"  {word}: {count}")
    print("=========================\n")

def main():
    """Main entry point"""
    print("Starting reducer")
    reduce_phase()
    print("Reducer completed")

if __name__ == "__main__":
    main()