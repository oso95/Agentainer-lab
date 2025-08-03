#!/usr/bin/env python3
"""
Debug version - prints all environment variables
"""

import os
import json
import time
from flask import Flask, jsonify

app = Flask(__name__)

print("=== ENVIRONMENT VARIABLES ===")
for key, value in os.environ.items():
    if 'REDIS' in key or 'WORKFLOW' in key or 'AGENTAINER' in key:
        print(f"{key}={value}")
print("=============================")

# Show what Redis host we're using
REDIS_HOST = os.environ.get('AGENTAINER_REDIS_HOST', os.environ.get('REDIS_HOST', 'host.docker.internal'))
print(f"Final REDIS_HOST: {REDIS_HOST}")

@app.route('/')
def home():
    """Return environment info"""
    return jsonify({
        "agent": "doc-extractor-debug",
        "env": {
            "REDIS_HOST": os.environ.get('REDIS_HOST'),
            "AGENTAINER_REDIS_HOST": os.environ.get('AGENTAINER_REDIS_HOST'),
            "WORKFLOW_ID": os.environ.get('WORKFLOW_ID'),
            "final_redis_host": REDIS_HOST
        }
    })

@app.route('/health')
def health():
    return jsonify({"status": "healthy"})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 8080))
    print(f"Starting Debug Agent on port {port}")
    app.run(host='0.0.0.0', port=port)