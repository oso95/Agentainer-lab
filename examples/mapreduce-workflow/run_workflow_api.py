#!/usr/bin/env python3
"""
MapReduce Word Counter Workflow
Demonstrates Agentainer's workflow orchestration with retry support

This example shows:
1. List phase - generate URLs to process
2. Parallel MAP processing with retry - analyze each URL  
3. REDUCE aggregation - combine word counts
"""

import os
import sys
import json
import time
import requests
import csv
from typing import Dict, List, Any
from datetime import datetime

try:
    import redis
    HAS_REDIS = True
except ImportError:
    HAS_REDIS = False
    print("Warning: Redis module not available. Some features may be limited.")

class MapReduceWorkflow:
    """
    Demonstrates MapReduce pattern with Agentainer workflows
    """
    
    def __init__(self, api_url=None, output_dir=None, urls_file=None):
        # Agentainer API configuration
        if api_url is None:
            api_url = os.getenv('AGENTAINER_API_URL', 'http://localhost:8081')
        self.api_url = api_url
        
        # URLs file
        self.urls_file = urls_file or "urls.txt"
        
        # Output directory for results
        if output_dir is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            output_dir = f"mapreduce_results_{timestamp}"
        self.output_dir = output_dir
        os.makedirs(self.output_dir, exist_ok=True)
        
        # Authentication for Agentainer API
        auth_token = os.getenv('AGENTAINER_AUTH_TOKEN', 'agentainer-default-token')
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {auth_token}"
        }
        
        # Redis client for accessing workflow state
        if HAS_REDIS:
            try:
                self.redis_client = redis.Redis(host='localhost', port=6379, decode_responses=True)
                self.redis_client.ping()  # Test connection
            except:
                print("Warning: Could not connect to Redis. State access will be limited.")
                self.redis_client = None
        else:
            self.redis_client = None
        
        print(f"📁 Results will be saved to: {os.path.abspath(self.output_dir)}")
    
    def load_urls(self) -> List[str]:
        """Load URLs from file"""
        urls = []
        try:
            with open(self.urls_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith('#'):
                        urls.append(line)
        except FileNotFoundError:
            print(f"❌ URLs file not found: {self.urls_file}")
            print("Please create urls.txt with one URL per line")
            sys.exit(1)
        
        return urls
    
    def create_workflow(self, urls: List[str]) -> str:
        """
        Create the MapReduce workflow using Agentainer's Workflow API
        """
        
        print("Creating workflow configuration...")
        
        # ========================================
        # WORKFLOW CONFIGURATION
        # ========================================
        workflow_config = {
            "name": "mapreduce-word-counter",
            "description": "MapReduce example that counts words from multiple URLs in parallel with retry support",
            "config": {
                "max_parallel": 5,
                "timeout": "10m",
                "failure_strategy": "continue",
                "cleanup_policy": "on_success"  # Keep failed agents for debugging
            },
            
            # ========================================
            # WORKFLOW STEPS
            # ========================================
            "steps": [
                # -------------------------------------
                # STEP 1: List URLs (Sequential)
                # -------------------------------------
                {
                    "id": "list",
                    "name": "list-urls",
                    "type": "sequential",
                    "config": {
                        "image": "mapreduce-mapper:latest",
                        "command": ["python", "mapper.py"],
                        "env_vars": {
                            "STEP_TYPE": "list",
                            "STEP_ID": "list-urls",
                            "URLS_JSON": json.dumps(urls)  # Pass URLs as JSON string
                        },
                        "timeout": "30s",
                        "retry_policy": {
                            "max_attempts": 2,
                            "backoff": "constant",
                            "delay": "3s"
                        },
                        "resource_limits": {
                            "cpu_limit": 500000000,    # 0.5 CPU
                            "memory_limit": 268435456   # 256MB
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 2: Process URLs (Map)
                # -------------------------------------
                {
                    "id": "process",
                    "name": "process-urls", 
                    "type": "map",
                    "depends_on": ["list"],
                    "config": {
                        "image": "mapreduce-mapper:latest",
                        "command": ["python", "mapper.py"],
                        "env_vars": {
                            "STEP_TYPE": "map"
                        },
                        "map_config": {
                            "input_path": "urls",
                            "item_alias": "current_url",
                            "max_concurrency": 5,
                            "error_handling": "continue_on_error"
                        },
                        "timeout": "2m",
                        "retry_policy": {
                            "max_attempts": 3,
                            "backoff": "exponential",
                            "delay": "2s"
                        },
                        "resource_limits": {
                            "cpu_limit": 500000000,    # 0.5 CPU
                            "memory_limit": 268435456   # 256MB
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 3: Aggregate Results (Reduce)
                # -------------------------------------
                {
                    "id": "aggregate",
                    "name": "aggregate-results",
                    "type": "reduce",
                    "depends_on": ["process"],
                    "config": {
                        "image": "mapreduce-reducer:latest",
                        "command": ["python", "reducer.py"],
                        "env_vars": {
                            "STEP_ID": "aggregate-results"
                        },
                        "timeout": "1m",
                        "retry_policy": {
                            "max_attempts": 2,
                            "backoff": "linear",
                            "delay": "5s"
                        },
                        "resource_limits": {
                            "cpu_limit": 1000000000,    # 1 CPU
                            "memory_limit": 536870912    # 512MB
                        }
                    }
                }
            ]
        }
        
        # ========================================
        # CREATE WORKFLOW VIA AGENTAINER API
        # ========================================
        print(f"\nCreating workflow at {self.api_url}/workflows...")
        response = requests.post(
            f"{self.api_url}/workflows",
            headers=self.headers,
            json=workflow_config,
            timeout=10
        )
        
        print(f"Response status: {response.status_code}")
        if response.status_code == 201:
            workflow_id = response.json()["data"]["id"]
            
            # Save workflow configuration
            self.save_json("workflow_config.json", workflow_config)
            self.save_json("workflow_metadata.json", {
                "workflow_id": workflow_id,
                "created_at": datetime.now().isoformat()
            })
            
            return workflow_id
        else:
            raise Exception(f"Failed to create workflow: {response.text}")
    
    def monitor_workflow(self, workflow_id: str) -> Dict:
        """
        Monitor workflow execution and save results
        """
        print(f"\n📊 Monitoring Workflow: {workflow_id}")
        print("=" * 60)
        
        saved_steps = set()
        last_status = None
        retry_info = {}
        
        while True:
            # Get workflow status from Agentainer
            try:
                response = requests.get(
                    f"{self.api_url}/workflows/{workflow_id}",
                    headers=self.headers,
                    timeout=10
                )
                
                if response.status_code != 200:
                    print(f"Error getting workflow status: {response.text}")
                    break
            except requests.exceptions.RequestException as e:
                print(f"\n⚠️  Connection error: {e}")
                print("Retrying in 5 seconds...")
                time.sleep(5)
                continue
            
            workflow = response.json()["data"]
            status = workflow["status"]
            
            # Only print if status changed
            if status != last_status:
                print(f"\n🔄 Workflow Status: {status}")
                last_status = status
            
            # Display progress for each step
            for step in workflow.get("steps", []):
                step_id = step["id"]
                step_status = step.get("status", "pending")
                step_metadata = step.get("metadata", {})
                
                # Check for retry count
                retry_count = int(step_metadata.get("retry_count", 0))
                
                # Track retries
                if retry_count > 0 and step_id not in retry_info:
                    retry_info[step_id] = retry_count
                    print(f"\n  🔁 {step['name']} is retrying (attempt #{retry_count + 1})")
                
                # Status icon
                status_icon = {
                    "completed": "✅",
                    "running": "🔄",
                    "failed": "❌", 
                    "pending": "⏳"
                }.get(step_status, "❓")
                
                # Special handling for map steps
                if step_id.startswith("process_url_"):
                    url_index = step_id.split("_")[-1]
                    print(f"  {status_icon} URL {url_index}: {step_status}")
                else:
                    retry_msg = f" (after {retry_count} retries)" if retry_count > 0 and step_status == "completed" else ""
                    print(f"  {status_icon} {step['name']}: {step_status}{retry_msg}")
                
                # Save completed step results
                if step_status == "completed" and step_id not in saved_steps:
                    self.save_step_results(workflow_id, step)
                    saved_steps.add(step_id)
            
            # Check if workflow is complete
            if status in ["completed", "failed", "cancelled"]:
                self.save_json("workflow_final_state.json", workflow)
                break
            
            time.sleep(2)
        
        return workflow
    
    def save_step_results(self, workflow_id: str, step: Dict):
        """Save results from completed workflow steps"""
        step_id = step["id"]
        
        if step_id == "list":
            # Save generated URLs
            urls = self.get_workflow_state(workflow_id, "urls")
            if urls:
                self.save_json("generated_urls.json", urls)
                print(f"\n   💾 Saved: {len(urls)} URLs to process")
        
        elif step_id == "process":
            # Save map results
            map_results = []
            map_errors = []
            
            if self.redis_client:
                # Get results from Redis lists
                results_key = f"workflow:{workflow_id}:state:list:map_results"
                errors_key = f"workflow:{workflow_id}:state:list:map_errors"
                
                # Get all results
                all_results = self.redis_client.lrange(results_key, 0, -1)
                for result in all_results:
                    try:
                        map_results.append(json.loads(result))
                    except:
                        pass
                
                # Get all errors
                all_errors = self.redis_client.lrange(errors_key, 0, -1)
                for error in all_errors:
                    try:
                        map_errors.append(json.loads(error))
                    except:
                        pass
            
            if map_results:
                self.save_json("map_results.json", map_results)
                print(f"   💾 Saved: {len(map_results)} successful URL results")
                
                # Save individual URL word counts
                for i, result in enumerate(map_results):
                    url = result.get('url', 'unknown')
                    # Create a safe filename from URL
                    safe_name = url.replace('https://', '').replace('http://', '').replace('/', '_').replace(':', '_')
                    self.save_json(f"wordcount_{safe_name}.json", {
                        "url": url,
                        "word_count": result.get('word_count', 0),
                        "unique_words": result.get('unique_words', 0),
                        "top_words": result.get('top_words', {}),
                        "response_time": result.get('response_time', 0)
                    })
            
            if map_errors:
                self.save_json("map_errors.json", map_errors)
                print(f"   💾 Saved: {len(map_errors)} failed URL results")
        
        elif step_id == "aggregate":
            # Save final summary
            summary = self.get_workflow_state(workflow_id, "final_summary")
            if summary:
                self.save_json("final_summary.json", summary)
                
                # Create readable report
                report = [
                    "# MapReduce Word Count Results\n",
                    f"Workflow ID: {workflow_id}",
                    f"Timestamp: {summary.get('timestamp', 'N/A')}",
                    f"\n## Processing Statistics:",
                    f"- Total URLs processed: {summary.get('total_urls_processed', 0)}",
                    f"- Total URLs failed: {summary.get('total_urls_failed', 0)}",
                    f"- Success rate: {summary.get('success_rate', 0):.1f}%",
                    f"\n## Content Analysis:",
                    f"- Total words: {summary.get('total_words', 0):,}",
                    f"- Total unique words: {summary.get('total_unique_words', 0):,}",
                    f"- Average words/page: {summary.get('average_words_per_page', 0):.1f}",
                    f"\n## Performance Metrics:",
                    f"- Total response time: {summary.get('performance', {}).get('total_response_time', 0):.2f}s",
                    f"- Average response time: {summary.get('performance', {}).get('avg_response_time', 0):.2f}s",
                    f"\n## Top 20 Words:"
                ]
                
                top_words = summary.get('top_30_words', {})
                for i, (word, count) in enumerate(list(top_words.items())[:20], 1):
                    report.append(f"{i:2d}. {word}: {count}")
                
                self.save_text("FINAL_REPORT.md", "\n".join(report))
    
    def get_workflow_state(self, workflow_id: str, key: str):
        """Get data from workflow state in Redis"""
        if not self.redis_client:
            return None
            
        state_key = f"workflow:{workflow_id}:state"
        value = self.redis_client.hget(state_key, key)
        if value:
            try:
                return json.loads(value)
            except:
                return value
        return None
    
    def save_json(self, filename: str, data: Any):
        """Save data as JSON"""
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=2, ensure_ascii=False)
    
    def save_text(self, filename: str, text: str):
        """Save text content"""
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(text)
    
    def export_word_frequency_csv(self):
        """Export word frequencies to CSV"""
        try:
            summary_file = os.path.join(self.output_dir, "final_summary.json")
            if not os.path.exists(summary_file):
                return
                
            with open(summary_file, 'r') as f:
                summary = json.load(f)
            
            top_words = summary.get('top_30_words', {})
            
            csv_file = os.path.join(self.output_dir, "word_frequencies.csv")
            with open(csv_file, 'w', newline='') as f:
                writer = csv.writer(f)
                writer.writerow(['Rank', 'Word', 'Count', 'Percentage'])
                
                total_words = summary.get('total_words', 1)
                for i, (word, count) in enumerate(top_words.items(), 1):
                    percentage = (count / total_words) * 100
                    writer.writerow([i, word, count, f"{percentage:.2f}%"])
            
            print(f"   💾 Exported: word_frequencies.csv")
        except Exception as e:
            print(f"   ⚠️  Failed to export word frequencies: {e}")
    
    def export_url_performance_csv(self):
        """Export URL performance metrics to CSV"""
        try:
            map_results_file = os.path.join(self.output_dir, "map_results.json")
            if not os.path.exists(map_results_file):
                return
                
            with open(map_results_file, 'r') as f:
                map_results = json.load(f)
            
            csv_file = os.path.join(self.output_dir, "url_performance.csv")
            with open(csv_file, 'w', newline='') as f:
                writer = csv.writer(f)
                writer.writerow(['URL', 'Word Count', 'Unique Words', 'Response Time (s)', 'Status Code'])
                
                for result in map_results:
                    writer.writerow([
                        result.get('url', ''),
                        result.get('word_count', 0),
                        result.get('unique_words', 0),
                        result.get('response_time', 0),
                        result.get('status_code', 'N/A')
                    ])
            
            print(f"   💾 Exported: url_performance.csv")
        except Exception as e:
            print(f"   ⚠️  Failed to export URL performance: {e}")
    
    def export_detailed_analysis(self):
        """Export detailed analysis markdown"""
        try:
            summary_file = os.path.join(self.output_dir, "final_summary.json")
            if not os.path.exists(summary_file):
                return
                
            with open(summary_file, 'r') as f:
                summary = json.load(f)
            
            analysis = [
                "# MapReduce Detailed Analysis\n",
                f"Generated: {datetime.now().isoformat()}\n",
                "\n## Overview\n",
                f"- **Total URLs Analyzed**: {summary.get('total_urls_processed', 0)}",
                f"- **Successful**: {summary.get('total_urls_processed', 0) - summary.get('total_urls_failed', 0)}",
                f"- **Failed**: {summary.get('total_urls_failed', 0)}",
                f"- **Success Rate**: {summary.get('success_rate', 0):.1f}%\n",
                "\n## Content Statistics\n",
                f"- **Total Words Analyzed**: {summary.get('total_words', 0):,}",
                f"- **Unique Words**: {summary.get('total_unique_words', 0):,}",
                f"- **Average Words per Page**: {summary.get('average_words_per_page', 0):.1f}",
                f"- **Vocabulary Diversity**: {summary.get('vocabulary_diversity', 0):.2f}\n",
                "\n## Performance Metrics\n"
            ]
            
            perf = summary.get('performance', {})
            analysis.extend([
                f"- **Total Processing Time**: {perf.get('total_response_time', 0):.2f}s",
                f"- **Average Response Time**: {perf.get('avg_response_time', 0):.2f}s",
                f"- **Min Response Time**: {perf.get('min_response_time', 0):.2f}s",
                f"- **Max Response Time**: {perf.get('max_response_time', 0):.2f}s\n"
            ])
            
            self.save_text("DETAILED_ANALYSIS.md", "\n".join(analysis))
            print(f"   💾 Exported: DETAILED_ANALYSIS.md")
        except Exception as e:
            print(f"   ⚠️  Failed to export detailed analysis: {e}")
    
    def export_all_formats(self):
        """Export results in all available formats"""
        print("\n📤 Exporting additional formats...")
        self.export_word_frequency_csv()
        self.export_url_performance_csv()
        self.export_detailed_analysis()

def main():
    """
    Main execution - Orchestrate MapReduce workflow
    """
    import argparse
    
    parser = argparse.ArgumentParser(description='MapReduce Word Counter Workflow')
    parser.add_argument('--urls', default='urls.txt', help='Path to URLs file (default: urls.txt)')
    parser.add_argument('--output', help='Output directory name')
    parser.add_argument('--no-export', action='store_true', help='Skip exporting additional formats (CSV, detailed analysis)')
    args = parser.parse_args()
    
    print("🔢 MapReduce Word Counter Workflow")
    print("=" * 60)
    print("\nThis demo shows Agentainer's MapReduce pattern with:")
    print("- Automatic retry on failures")
    print("- Parallel URL processing")
    print("- Error resilience")
    print("- Resource management")
    
    # Check if Docker is running
    try:
        result = os.system("docker info > /dev/null 2>&1")
        if result != 0:
            print("\n❌ Docker is not running!")
            print("Please start Docker Desktop and try again.")
            sys.exit(1)
    except:
        print("\n❌ Error checking Docker status")
        sys.exit(1)
    
    # Check if Agentainer server is reachable
    try:
        response = requests.get("http://localhost:8081/health", timeout=2)
        if response.status_code != 200:
            print("\n❌ Agentainer server is not responding!")
            print("Please ensure Agentainer server is running:")
            print("  ./agentainer server")
            sys.exit(1)
    except requests.exceptions.RequestException:
        print("\n❌ Cannot connect to Agentainer server!")
        print("Please ensure Agentainer server is running:")
        print("  ./agentainer server")
        print("\nFrom the repository root directory, run:")
        print("  go build -o agentainer ./cmd/agentainer/")
        print("  ./agentainer server")
        sys.exit(1)
    
    # Check if images exist
    try:
        result = os.system("docker images -q mapreduce-mapper:latest > /dev/null 2>&1")
        if result != 0:
            print("\n❌ Docker images not found!")
            print("Please build the images first:")
            print("  ./build.sh")
            sys.exit(1)
    except:
        pass
    
    # Initialize workflow with command-line arguments
    workflow = MapReduceWorkflow(output_dir=args.output, urls_file=args.urls)
    
    # Load URLs
    print(f"\n📋 Loading URLs from {args.urls}...")
    urls = workflow.load_urls()
    print(f"   Found {len(urls)} URLs to analyze")
    
    if len(urls) == 0:
        print("\n❌ No URLs found in urls.txt")
        print("Please add URLs to analyze (one per line)")
        print("\nExample urls.txt format:")
        print("# Comments start with #")
        print("https://example.com")
        print("https://techcrunch.com/some-article")
        sys.exit(1)
    
    # Show first few URLs
    print("\n   URLs to process:")
    for i, url in enumerate(urls[:5]):
        print(f"   - {url}")
    if len(urls) > 5:
        print(f"   ... and {len(urls) - 5} more")
    
    try:
        # Save URLs to results directory
        workflow.save_json("input_urls.json", urls)
        
        # Create workflow with URLs
        print("\n1️⃣ Creating MapReduce workflow...")
        workflow_id = workflow.create_workflow(urls)
        print(f"   ✅ Workflow created: {workflow_id}")
        
        # Start workflow execution
        print("\n2️⃣ Starting workflow execution...")
        response = requests.post(
            f"{workflow.api_url}/workflows/{workflow_id}/start",
            headers=workflow.headers
        )
        
        if response.status_code in [200, 202]:
            print("   ✅ Workflow started")
            print("\n💡 Watch as Agentainer:")
            print("   - Generates list of URLs (including some that will fail)")
            print("   - Processes URLs in parallel with retry on failures")
            print("   - Aggregates word counts from all successful URLs")
        else:
            print(f"   ❌ Failed to start workflow: {response.text}")
            return
        
        # Monitor execution
        print("\n3️⃣ Monitoring workflow execution...")
        final_workflow = workflow.monitor_workflow(workflow_id)
        
        print("\n✨ MapReduce workflow complete!")
        print(f"\n📁 All results saved to: {os.path.abspath(workflow.output_dir)}")
        
        # Export additional formats unless disabled
        if not args.no_export:
            workflow.export_all_formats()
        
        # Show summary
        summary_file = os.path.join(workflow.output_dir, "FINAL_REPORT.md")
        if os.path.exists(summary_file):
            print("\n📊 Results Summary:")
            with open(summary_file, 'r') as f:
                print(f.read())
        
        # List all exported files
        print("\n📄 All Exported Files:")
        exported_files = sorted([f for f in os.listdir(workflow.output_dir) if f.endswith(('.json', '.md', '.txt', '.csv'))])
        for file in exported_files:
            file_path = os.path.join(workflow.output_dir, file)
            file_size = os.path.getsize(file_path)
            print(f"   - {file} ({file_size:,} bytes)")
        
    except KeyboardInterrupt:
        print("\n\n⚠️  Workflow interrupted by user")
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()