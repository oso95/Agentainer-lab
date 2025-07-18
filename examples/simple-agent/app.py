#!/usr/bin/env python3

import os
import time
import json
from flask import Flask, request, jsonify
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

class SimpleAgent:
    def __init__(self):
        self.name = os.getenv("AGENT_NAME", "simple-agent")
        self.version = "1.0.0"
        self.start_time = time.time()
        self.request_count = 0
        
    def get_status(self):
        return {
            "name": self.name,
            "version": self.version,
            "status": "running",
            "uptime": time.time() - self.start_time,
            "request_count": self.request_count
        }
    
    def process_request(self, data):
        self.request_count += 1
        logger.info(f"Processing request #{self.request_count}: {data}")
        
        # Simple echo response with some processing
        response = {
            "agent": self.name,
            "request_id": self.request_count,
            "timestamp": time.time(),
            "processed_data": data,
            "message": f"Hello from {self.name}! I received: {data}"
        }
        
        return response

agent = SimpleAgent()

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy"}), 200

@app.route('/status', methods=['GET'])
def status():
    return jsonify(agent.get_status()), 200

@app.route('/process', methods=['POST'])
def process():
    try:
        data = request.get_json()
        if not data:
            return jsonify({"error": "No data provided"}), 400
        
        result = agent.process_request(data)
        return jsonify(result), 200
        
    except Exception as e:
        logger.error(f"Error processing request: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/invoke', methods=['POST'])
def invoke():
    """Main invocation endpoint for the agent"""
    try:
        data = request.get_json() or {}
        
        # Extract the message or use default
        message = data.get("message", "Hello from Agentainer!")
        
        result = agent.process_request({"message": message})
        return jsonify(result), 200
        
    except Exception as e:
        logger.error(f"Error invoking agent: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/', methods=['GET'])
def root():
    return jsonify({
        "message": "Simple Agent is running",
        "endpoints": {
            "health": "/health",
            "status": "/status", 
            "process": "/process",
            "invoke": "/invoke"
        }
    }), 200

if __name__ == '__main__':
    logger.info(f"Starting {agent.name} v{agent.version}")
    app.run(host='0.0.0.0', port=8000, debug=False)