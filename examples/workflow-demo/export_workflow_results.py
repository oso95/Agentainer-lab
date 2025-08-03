#!/usr/bin/env python3
"""
Export Workflow Results to Files
Exports results from a completed workflow to local files
"""

import os
import sys
import json
import redis
import requests
from datetime import datetime
from typing import Dict, Any

class WorkflowResultsExporter:
    """Export workflow results to files"""
    
    def __init__(self, workflow_id: str, output_dir: str = None):
        self.workflow_id = workflow_id
        self.api_url = os.getenv('AGENTAINER_API_URL', 'http://localhost:8081')
        
        # Set up output directory
        if output_dir is None:
            output_dir = f"workflow_export_{workflow_id[:8]}_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
        self.output_dir = output_dir
        os.makedirs(self.output_dir, exist_ok=True)
        
        # Get auth token
        auth_token = os.getenv('AGENTAINER_AUTH_TOKEN', 'agentainer-default-token')
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {auth_token}"
        }
        
        # Connect to Redis
        self.redis_client = redis.Redis(host='localhost', port=6379, decode_responses=True)
        
        print(f"📁 Exporting to: {os.path.abspath(self.output_dir)}")
    
    def save_json(self, filename: str, data: Any):
        """Save data as JSON"""
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(data, f, indent=2, ensure_ascii=False)
        print(f"   💾 Saved: {filename}")
    
    def save_text(self, filename: str, text: str):
        """Save text content"""
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(text)
        print(f"   💾 Saved: {filename}")
    
    def get_workflow_state(self, key: str):
        """Get workflow state from Redis"""
        state_key = f"workflow:{self.workflow_id}:state"
        value = self.redis_client.hget(state_key, key)
        if value:
            try:
                return json.loads(value)
            except:
                return value
        return None
    
    def export_workflow_data(self):
        """Export all workflow data"""
        print(f"\n📊 Exporting workflow: {self.workflow_id}")
        
        # Get workflow details
        response = requests.get(
            f"{self.api_url}/workflows/{self.workflow_id}",
            headers=self.headers
        )
        
        if response.status_code != 200:
            print(f"❌ Failed to get workflow: {response.text}")
            return False
        
        workflow = response.json()["data"]
        
        # Save workflow metadata
        self.save_json("workflow_metadata.json", {
            "workflow_id": self.workflow_id,
            "name": workflow.get("name"),
            "status": workflow.get("status"),
            "started_at": workflow.get("started_at"),
            "completed_at": workflow.get("completed_at"),
            "exported_at": datetime.now().isoformat()
        })
        
        # Save full workflow data
        self.save_json("workflow_full.json", workflow)
        
        # Export results from each step
        print("\n📝 Exporting step results...")
        
        # Extract and save chunks
        chunks = self.get_workflow_state("chunks")
        if chunks:
            self.save_json("extracted_chunks.json", chunks)
            # Save individual chunk files
            for i, chunk in enumerate(chunks):
                if isinstance(chunk, dict) and "text" in chunk:
                    self.save_text(f"chunk_{i+1}.txt", chunk["text"])
                elif isinstance(chunk, str):
                    # Chunk might be just an ID, try to get the actual text
                    chunk_text = self.get_workflow_state(chunk)
                    if chunk_text:
                        self.save_text(f"chunk_{i+1}.txt", chunk_text)
        
        # Extract summaries
        summaries = {}
        for i in range(10):  # Check up to 10 chunks
            summary = self.get_workflow_state(f"summary_chunk_{i}")
            if summary:
                summaries[f"chunk_{i}"] = summary
        if summaries:
            self.save_json("chunk_summaries.json", summaries)
        
        # Extract entities
        entities = self.get_workflow_state("extracted_entities")
        if entities:
            self.save_json("extracted_entities.json", entities)
            # Create a readable entities report
            if isinstance(entities, dict) and "entities" in entities:
                entity_report = ["# Extracted Entities\n"]
                for category, items in entities["entities"].items():
                    entity_report.append(f"\n## {category}")
                    if isinstance(items, list):
                        for item in items:
                            entity_report.append(f"- {item}")
                    elif isinstance(items, dict):
                        for k, v in items.items():
                            entity_report.append(f"- {k}: {v}")
                self.save_text("entities_report.md", "\n".join(entity_report))
        
        # Extract cross references
        cross_refs = self.get_workflow_state("analysis_cross_reference")
        if cross_refs:
            self.save_json("cross_references.json", cross_refs)
        
        # Extract insights
        insights = self.get_workflow_state("analysis_insights")
        if insights:
            self.save_json("insights.json", insights)
            if isinstance(insights, dict) and "insights" in insights:
                self.save_text("insights.txt", insights["insights"])
            elif isinstance(insights, str):
                self.save_text("insights.txt", insights)
        
        # Extract final report
        final_report = self.get_workflow_state("analysis_final_report")
        if final_report:
            self.save_json("final_report.json", final_report)
            if isinstance(final_report, dict) and "final_analysis" in final_report:
                self.save_text("final_report.md", final_report["final_analysis"])
            elif isinstance(final_report, str):
                self.save_text("final_report.md", final_report)
        
        # Create index file
        self.create_index_file()
        
        return True
    
    def create_index_file(self):
        """Create an index file listing all exported files"""
        index_content = [
            f"# Workflow Export Index",
            f"\nWorkflow ID: `{self.workflow_id}`",
            f"Exported: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
            f"\n## Files"
        ]
        
        files = sorted([f for f in os.listdir(self.output_dir) if f != "INDEX.md"])
        for file in files:
            filepath = os.path.join(self.output_dir, file)
            size = os.path.getsize(filepath)
            index_content.append(f"- **{file}** ({size:,} bytes)")
            
            # Add description based on filename
            if file == "workflow_metadata.json":
                index_content.append("  - Workflow metadata and timestamps")
            elif file == "extracted_chunks.json":
                index_content.append("  - Document chunks extracted from source")
            elif file.startswith("chunk_") and file.endswith(".txt"):
                index_content.append("  - Individual document chunk text")
            elif file == "chunk_summaries.json":
                index_content.append("  - AI-generated summaries of each chunk")
            elif file == "extracted_entities.json":
                index_content.append("  - Named entities extracted from document")
            elif file == "entities_report.md":
                index_content.append("  - Human-readable entities report")
            elif file == "insights.txt":
                index_content.append("  - AI-generated insights from analysis")
            elif file == "final_report.md":
                index_content.append("  - Final analysis report in Markdown format")
        
        self.save_text("INDEX.md", "\n".join(index_content))

def main():
    """Export workflow results"""
    if len(sys.argv) < 2:
        print("Usage: python export_workflow_results.py <workflow_id> [output_dir]")
        print("\nExample:")
        print("  python export_workflow_results.py abc123def456")
        print("  python export_workflow_results.py abc123def456 my_results")
        sys.exit(1)
    
    workflow_id = sys.argv[1]
    output_dir = sys.argv[2] if len(sys.argv) > 2 else None
    
    exporter = WorkflowResultsExporter(workflow_id, output_dir)
    
    if exporter.export_workflow_data():
        print(f"\n✅ Export complete!")
        print(f"📁 Files saved to: {os.path.abspath(exporter.output_dir)}")
    else:
        print("\n❌ Export failed!")

if __name__ == "__main__":
    main()