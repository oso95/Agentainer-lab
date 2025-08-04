#!/usr/bin/env python3
"""
Multi-URL Analysis Workflow Demo
Demonstrates Agentainer's workflow orchestration with Map-Reduce pattern

This example shows:
1. Sequential processing (prepare URLs)
2. Parallel MAP processing (analyze each URL)
3. REDUCE aggregation (combine results)
4. Complex multi-step orchestration
"""

import os
import sys
import json
import time
import requests
from typing import Dict, List, Any
from datetime import datetime
import redis

class MultiURLWorkflow:
    """
    Demonstrates Agentainer's workflow orchestration capabilities
    """
    
    def __init__(self, api_url=None, output_dir=None):
        # Agentainer API configuration
        if api_url is None:
            api_url = os.getenv('AGENTAINER_API_URL', 'http://localhost:8081')
        self.api_url = api_url
        
        # Output directory for results
        if output_dir is None:
            # Check if running in container
            if os.path.exists('/output'):
                # Container environment - use mounted output directory
                timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
                output_dir = f"/output/analysis_results_{timestamp}"
            else:
                # Host environment
                timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
                output_dir = f"analysis_results_{timestamp}"
        self.output_dir = output_dir
        os.makedirs(self.output_dir, exist_ok=True)
        
        # Authentication for Agentainer API
        auth_token = os.getenv('AGENTAINER_AUTH_TOKEN', 'agentainer-default-token')
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {auth_token}"
        }
        
        # Redis client for accessing workflow state
        redis_host = os.getenv('REDIS_HOST', 'localhost')
        redis_port = int(os.getenv('REDIS_PORT', '6379'))
        self.redis_client = redis.Redis(host=redis_host, port=redis_port, decode_responses=True)
        
        print(f"📁 Results will be saved to: {os.path.abspath(self.output_dir)}")
    
    def load_urls(self, urls_file: str = "urls.txt") -> List[str]:
        """Load URLs from file"""
        urls = []
        try:
            with open(urls_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith('#'):
                        urls.append(line)
        except FileNotFoundError:
            print(f"❌ URLs file not found: {urls_file}")
            print("Please create urls.txt with one URL per line")
            sys.exit(1)
        
        return urls
    
    def create_workflow(self, urls: List[str]) -> str:
        """
        Create the workflow using Agentainer's Workflow API
        
        KEY AGENTAINER FEATURES DEMONSTRATED:
        1. Multi-step workflow with dependencies
        2. Different step types: sequential, map, reduce
        3. Dynamic input passing between steps
        4. Parallel execution with map_over
        """
        
        # ========================================
        # WORKFLOW CONFIGURATION
        # This defines the entire processing pipeline
        # ========================================
        workflow_config = {
            "name": f"multi-url-analysis-{len(urls)}-articles",
            "description": f"Analyze {len(urls)} web articles in parallel",
            "config": {
                "max_parallel": 3,  # Limited to 3 for testing
                "timeout": "30m",
                "failure_strategy": "continue_on_partial"  # Continue even if some URLs fail
            },
            
            # ========================================
            # WORKFLOW STEPS
            # Each step represents a phase in processing
            # ========================================
            "steps": [
                # -------------------------------------
                # STEP 1: Prepare URLs (Sequential)
                # -------------------------------------
                {
                    "id": "prepare",
                    "name": "Prepare URLs",
                    "type": "sequential",  # Runs once
                    "config": {
                        "image": "doc-extractor:latest",
                        "command": ["python", "app_multi.py"],
                        "env_vars": {
                            "TASK_TYPE": "prepare_urls",
                            "URL_COUNT": str(len(urls))  # Pass URL count for validation
                        },
                        "resource_limits": {
                            "cpu_limit": 500000000,  # 0.5 CPU cores (in nanoseconds)
                            "memory_limit": 268435456  # 256 MB
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 2: Process URLs (Map)
                # This is where parallel magic happens!
                # -------------------------------------
                {
                    "id": "process",
                    "name": "Process URLs in Parallel",
                    "type": "map",  # MAP: Run in parallel
                    "depends_on": ["prepare"],  # Wait for prepare to complete
                    "config": {
                        "image": "doc-extractor:latest",
                        "command": ["python", "app_multi.py"],
                        "env_vars": {
                            "TASK_TYPE": "process_url"
                        },
                        "map_config": {
                            "input_path": "urls",  # Map over the urls array from workflow state
                            "item_alias": "current_url",  # Each URL will be available as current_url
                            "max_concurrency": 3,  # Process up to 3 URLs in parallel
                            "error_handling": "continue_on_error"  # Continue even if some URLs fail
                        },
                        "resource_limits": {
                            "cpu_limit": 250000000,  # 0.25 CPU cores per URL processor
                            "memory_limit": 268435456  # 256 MB per URL processor
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 3: Aggregate Results (Reduce)
                # -------------------------------------
                {
                    "id": "aggregate",
                    "name": "Aggregate Results",
                    "type": "reduce",  # REDUCE: Combine all results
                    "depends_on": ["process"],  # Wait for all map processing
                    "config": {
                        "image": "doc-extractor:latest",
                        "command": ["python", "app_multi.py"],
                        "env_vars": {
                            "TASK_TYPE": "aggregate"
                        },
                        "resource_limits": {
                            "cpu_limit": 500000000,  # 0.5 CPU cores
                            "memory_limit": 268435456  # 256 MB
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 4: Extract Entities (Sequential)
                # Process aggregated content with AI
                # -------------------------------------
                {
                    "id": "entities",
                    "name": "Extract Entities with AI",
                    "type": "sequential",
                    "depends_on": ["aggregate"],
                    "config": {
                        "image": "gemini-workflow-agent:latest",
                        "command": ["python", "app.py"],
                        "env_vars": {
                            "TASK_TYPE": "extract_entities_multi"
                        },
                        "resource_limits": {
                            "cpu_limit": 1000000000,  # 1 CPU core for AI processing
                            "memory_limit": 1073741824  # 1 GB for AI model
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 5: Generate Insights (Sequential)
                # Create insights from all articles
                # -------------------------------------
                {
                    "id": "insights",
                    "name": "Generate Cross-Article Insights",
                    "type": "sequential",
                    "depends_on": ["entities"],
                    "config": {
                        "image": "gpt-workflow-agent:latest",
                        "command": ["python", "app.py"],
                        "env_vars": {
                            "TASK_TYPE": "generate_insights_multi"
                        },
                        "resource_limits": {
                            "cpu_limit": 1000000000,  # 1 CPU core for AI processing
                            "memory_limit": 1073741824  # 1 GB for AI model
                        }
                    }
                },
                
                # -------------------------------------
                # STEP 6: Create Final Report (Sequential)
                # -------------------------------------
                {
                    "id": "report",
                    "name": "Create Comprehensive Report",
                    "type": "sequential",
                    "depends_on": ["insights"],
                    "config": {
                        "image": "gemini-workflow-agent:latest",
                        "command": ["python", "app.py"],
                        "env_vars": {
                            "TASK_TYPE": "final_report_multi"
                        },
                        "resource_limits": {
                            "cpu_limit": 1000000000,  # 1 CPU core for AI processing
                            "memory_limit": 1073741824  # 1 GB for AI model
                        }
                    }
                }
            ]
        }
        # ========================================
        # CREATE WORKFLOW VIA AGENTAINER API
        # ========================================
        response = requests.post(
            f"{self.api_url}/workflows",
            headers=self.headers,
            json=workflow_config
        )
        
        if response.status_code == 201:
            workflow_id = response.json()["data"]["id"]
            
            # Save workflow configuration
            self.save_json("workflow_config.json", workflow_config)
            self.save_json("workflow_metadata.json", {
                "workflow_id": workflow_id,
                "created_at": datetime.now().isoformat(),
                "url_count": len(urls),
                "urls": urls
            })
            
            return workflow_id
        else:
            raise Exception(f"Failed to create workflow: {response.text}")
    
    def monitor_workflow(self, workflow_id: str) -> Dict:
        """
        Monitor workflow execution and save results
        Shows real-time progress of parallel execution
        """
        print(f"\n📊 Monitoring Workflow: {workflow_id}")
        print("=" * 60)
        
        saved_steps = set()
        last_status = None
        
        while True:
            # ========================================
            # GET WORKFLOW STATUS FROM AGENTAINER
            # ========================================
            try:
                response = requests.get(
                    f"{self.api_url}/workflows/{workflow_id}",
                    headers=self.headers,
                    timeout=10  # Add timeout to prevent hanging
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
            
            # ========================================
            # DISPLAY PROGRESS FOR EACH STEP
            # ========================================
            for step in workflow.get("steps", []):
                step_id = step["id"]
                step_status = step.get("status", "pending")
                
                # Special handling for URL processing steps
                if step_id.startswith("process_url_"):
                    # Show URL processing status
                    status_icon = {
                        "completed": "✅",
                        "running": "🔄",
                        "failed": "❌",
                        "pending": "⏳"
                    }.get(step_status, "❓")
                    print(f"  {status_icon} {step['name']}: {step_status}")
                else:
                    # Regular step display
                    status_icon = {
                        "completed": "✅",
                        "running": "🔄",
                        "failed": "❌",
                        "pending": "⏳"
                    }.get(step_status, "❓")
                    
                    print(f"  {status_icon} {step['name']}: {step_status}")
                
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
        
        # First try to get from Redis, then check workflow state
        def get_state_data(key):
            # Try Redis first
            data = self.get_workflow_state(workflow_id, key)
            if data:
                return data
            
            # If not in Redis, check if it's in the workflow's state field
            try:
                response = requests.get(
                    f"{self.api_url}/workflows/{workflow_id}",
                    headers=self.headers,
                    timeout=10
                )
                if response.status_code == 200:
                    workflow = response.json()["data"]
                    state = workflow.get("state", {})
                    return state.get(key)
            except:
                pass
            return None
        
        # Get results based on step type
        if step_id == "process":
            # Save individual URL processing results
            process_results = get_state_data("process_results") or get_state_data("individual_results")
            if process_results and isinstance(process_results, list):
                self.save_json("url_processing_results.json", process_results)
                
                # Save individual URL content files
                for idx, result in enumerate(process_results):
                    if isinstance(result, dict):
                        self.save_json(f"url_{idx}_metadata.json", result)
                        
                        # Extract and save content if available
                        if result.get("status") == "success" and "content" in result:
                            self.save_text(f"url_{idx}_content.txt", result["content"])
                        elif result.get("status") == "failed":
                            error_info = f"# URL {idx} Failed\n\n"
                            error_info += f"URL: {result.get('url', 'Unknown')}\n"
                            error_info += f"Error: {result.get('error', 'Unknown error')}\n"
                            self.save_text(f"url_{idx}_error.txt", error_info)
                            
        elif step_id == "aggregate":
            # First check if aggregated results exist as a separate key
            results = get_state_data("aggregated_results")
            
            # If not found, extract from workflow state
            if not results:
                try:
                    response = requests.get(
                        f"{self.api_url}/workflows/{workflow_id}",
                        headers=self.headers,
                        timeout=10
                    )
                    if response.status_code == 200:
                        workflow = response.json()["data"]
                        state = workflow.get("state", {})
                        # Extract aggregation data from state
                        results = {
                            "total_urls": state.get("total_urls", 0),
                            "successful": state.get("successful", 0),
                            "failed": state.get("failed", 0),
                            "total_words_analyzed": state.get("total_words_analyzed", 0),
                            "average_words_per_article": state.get("average_words_per_article", 0),
                            "unique_domains": state.get("unique_domains", []),
                            "articles_analyzed": state.get("articles_analyzed", 0)
                        }
                except:
                    pass
            
            if results:
                self.save_json("aggregated_results.json", results)
                
                # Create summary report
                summary = [
                    "# URL Processing Summary\n",
                    f"Total URLs: {results.get('total_urls', 0)}",
                    f"Successful: {results.get('successful', 0)}",
                    f"Failed: {results.get('failed', 0)}",
                    f"Total words analyzed: {results.get('total_words_analyzed', 0):,}",
                    f"Average words per article: {results.get('average_words_per_article', 0):.0f}",
                    "\n## Domains Analyzed:"
                ]
                for domain in results.get('unique_domains', []):
                    summary.append(f"- {domain}")
                
                self.save_text("processing_summary.txt", "\n".join(summary))
        
        elif step_id == "entities":
            # Save entity extraction results
            entities = get_state_data("extracted_entities_multi") or get_state_data("entities")
            if entities:
                self.save_json("entities_all_articles.json", entities)
                # Check if entities is a string (direct content) or dict
                if isinstance(entities, str):
                    self.save_text("entities.md", f"# Extracted Entities\n\n{entities}")
                elif isinstance(entities, dict) and "entities" in entities:
                    self.save_text("entities.md", f"# Extracted Entities\n\n{entities['entities']}")
        
        elif step_id == "insights":
            # Save insights
            insights = get_state_data("insights_multi") or get_state_data("insights")
            if insights:
                self.save_json("cross_article_insights.json", insights)
                # Check if insights is a string (direct content) or dict
                if isinstance(insights, str):
                    self.save_text("insights.md", f"# Cross-Article Insights\n\n{insights}")
                elif isinstance(insights, dict) and "insights" in insights:
                    self.save_text("insights.md", f"# Cross-Article Insights\n\n{insights['insights']}")
        
        elif step_id == "report":
            # Save final report
            report = get_state_data("final_report_multi") or get_state_data("final_report")
            if report:
                self.save_json("final_report.json", report)
                # Check if report is a string (direct content) or dict
                if isinstance(report, str):
                    self.save_text("FINAL_REPORT.md", report)
                elif isinstance(report, dict) and "final_report" in report:
                    self.save_text("FINAL_REPORT.md", report["final_report"])
    
    def get_workflow_state(self, workflow_id: str, key: str):
        """Get data from workflow state in Redis"""
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
        print(f"\n   💾 Saved: {filename}")
    
    def save_text(self, filename: str, text: str):
        """Save text content"""
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(text)
        print(f"   💾 Saved: {filename}")
    
    def create_final_summary(self, workflow_id: str, workflow: Dict):
        """Create a comprehensive summary of the analysis"""
        duration = self.calculate_duration(workflow)
        
        summary = [
            "# Multi-URL Analysis Summary",
            f"\nWorkflow ID: {workflow_id}",
            f"Duration: {duration}",
            f"Status: {workflow.get('status', 'Unknown')}",
            "\n## Files Generated:",
        ]
        
        # List all files
        files = sorted([f for f in os.listdir(self.output_dir) if f.endswith(('.json', '.txt', '.md'))])
        for file in files:
            summary.append(f"- {file}")
        
        self.save_text("ANALYSIS_SUMMARY.md", "\n".join(summary))
    
    def calculate_duration(self, workflow: Dict) -> str:
        """Calculate workflow execution time"""
        started = workflow.get("started_at")
        completed = workflow.get("completed_at")
        
        if not started or not completed:
            return "N/A"
        
        try:
            start_time = datetime.fromisoformat(started.replace('Z', '+00:00'))
            end_time = datetime.fromisoformat(completed.replace('Z', '+00:00'))
            duration = end_time - start_time
            
            seconds = duration.total_seconds()
            if seconds < 60:
                return f"{seconds:.1f} seconds"
            else:
                return f"{seconds/60:.1f} minutes"
        except:
            return "N/A"

def main():
    """
    Main execution - Orchestrate multi-URL analysis
    """
    print("🌐 Multi-URL Analysis Workflow Demo")
    print("=" * 60)
    print("\nThis demo shows Agentainer's workflow orchestration:")
    print("- Sequential processing (prepare)")
    print("- Parallel MAP processing (analyze URLs)")
    print("- REDUCE aggregation (combine results)")
    print("- AI-powered analysis steps")
    
    # Initialize workflow
    workflow = MultiURLWorkflow()
    
    # Load URLs
    print("\n📋 Loading URLs from urls.txt...")
    urls = workflow.load_urls()
    print(f"   Found {len(urls)} URLs to analyze")
    
    if len(urls) == 0:
        print("\n❌ No URLs found in urls.txt")
        print("Please add URLs to analyze (one per line)")
        sys.exit(1)
    
    try:
        # ========================================
        # STEP 1: CREATE WORKFLOW
        # ========================================
        print("\n1️⃣ Creating workflow with Agentainer API...")
        workflow_id = workflow.create_workflow(urls)
        print(f"   ✅ Workflow created: {workflow_id}")
        
        # ========================================
        # STEP 2: START WORKFLOW EXECUTION
        # ========================================
        print("\n2️⃣ Starting workflow execution...")
        response = requests.post(
            f"{workflow.api_url}/workflows/{workflow_id}/start",
            headers=workflow.headers
        )
        
        if response.status_code in [200, 202]:
            print("   ✅ Workflow started")
            print("\n💡 Watch as Agentainer:")
            print("   - Prepares URLs for processing")
            print("   - Launches parallel agents for each URL")
            print("   - Aggregates results automatically")
            print("   - Runs AI analysis on combined data")
        else:
            print(f"   ❌ Failed to start workflow: {response.text}")
            return
        
        # ========================================
        # STEP 3: MONITOR EXECUTION
        # ========================================
        print("\n3️⃣ Monitoring workflow execution...")
        final_workflow = workflow.monitor_workflow(workflow_id)
        
        # ========================================
        # STEP 4: CREATE FINAL SUMMARY
        # ========================================
        workflow.create_final_summary(workflow_id, final_workflow)
        
        print("\n✨ Analysis complete!")
        print(f"\n📁 All results saved to: {os.path.abspath(workflow.output_dir)}")
        
        # Show key insights
        insights_file = os.path.join(workflow.output_dir, "insights.txt")
        if os.path.exists(insights_file):
            print("\n🔍 Key Insights Preview:")
            with open(insights_file, 'r') as f:
                preview = f.read()[:500]
                print(preview + "..." if len(preview) == 500 else preview)
        
    except KeyboardInterrupt:
        print("\n\n⚠️  Analysis interrupted by user")
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()