import os
import json
from flask import Flask, request, jsonify
import requests
from dotenv import load_dotenv
import redis
from datetime import datetime

# Load environment variables
load_dotenv()

app = Flask(__name__)

# Configuration
API_KEY = os.environ.get('OPENAI_API_KEY', '')
MODEL = os.environ.get('OPENAI_MODEL', 'gpt-3.5-turbo')
API_URL = 'https://api.openai.com/v1/chat/completions'

# Redis connection (Agentainer provides Redis at host.docker.internal:6379)
REDIS_HOST = os.environ.get('AGENTAINER_REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('AGENTAINER_REDIS_PORT', 6379))
AGENT_ID = os.environ.get('AGENT_ID', 'gpt-agent')

try:
    redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
    redis_client.ping()
    print(f"Connected to Redis at {REDIS_HOST}:{REDIS_PORT}")
except:
    print(f"Warning: Could not connect to Redis at {REDIS_HOST}:{REDIS_PORT}")
    redis_client = None

@app.route('/health')
def health():
    return jsonify({
        'status': 'healthy' if API_KEY else 'unhealthy',
        'model': MODEL
    })

def get_conversation_history():
    """Get conversation history from Redis"""
    if not redis_client:
        return []
    
    try:
        history = redis_client.lrange(f"agent:{AGENT_ID}:conversations", 0, 10)
        return [json.loads(h) for h in history]
    except:
        return []

def save_conversation(user_msg, assistant_msg):
    """Save conversation to Redis"""
    if not redis_client:
        return
    
    try:
        conversation = {
            'timestamp': datetime.now().isoformat(),
            'user': user_msg,
            'assistant': assistant_msg
        }
        # Push to list and keep only last 50 conversations
        redis_client.lpush(f"agent:{AGENT_ID}:conversations", json.dumps(conversation))
        redis_client.ltrim(f"agent:{AGENT_ID}:conversations", 0, 49)
        
        # Update metrics
        redis_client.hincrby(f"agent:{AGENT_ID}:metrics", "total_conversations", 1)
    except Exception as e:
        print(f"Error saving conversation: {e}")

@app.route('/chat', methods=['POST'])
def chat():
    if not API_KEY:
        return jsonify({'error': 'OPENAI_API_KEY not configured'}), 500
    
    data = request.get_json()
    if not data or 'message' not in data:
        return jsonify({'error': 'message required'}), 400
    
    user_message = data['message']
    
    # Build messages with conversation history
    messages = []
    
    # Add system message
    system_prompt = data.get('system_prompt', 'You are a helpful assistant.')
    messages.append({'role': 'system', 'content': system_prompt})
    
    # Add recent conversation history for context
    history = get_conversation_history()
    for conv in reversed(history[:3]):  # Last 3 conversations
        messages.append({'role': 'user', 'content': conv['user']})
        messages.append({'role': 'assistant', 'content': conv['assistant']})
    
    # Add current message
    messages.append({'role': 'user', 'content': user_message})
    
    try:
        response = requests.post(
            API_URL,
            headers={
                'Authorization': f'Bearer {API_KEY}',
                'Content-Type': 'application/json'
            },
            json={
                'model': MODEL,
                'messages': messages,
                'temperature': 0.7
            }
        )
        response.raise_for_status()
        result = response.json()
        
        assistant_message = result['choices'][0]['message']['content']
        
        # Save conversation to Redis
        save_conversation(user_message, assistant_message)
        
        return jsonify({
            'response': assistant_message,
            'model': MODEL,
            'usage': result.get('usage', {}),
            'conversation_history': len(history) + 1
        })
        
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/history', methods=['GET'])
def history():
    """Get conversation history"""
    history = get_conversation_history()
    return jsonify({
        'conversations': history,
        'total': len(history)
    })

@app.route('/clear', methods=['POST'])
def clear():
    """Clear conversation history"""
    if redis_client:
        try:
            redis_client.delete(f"agent:{AGENT_ID}:conversations")
            redis_client.delete(f"agent:{AGENT_ID}:metrics")
            return jsonify({'message': 'History cleared'})
        except Exception as e:
            return jsonify({'error': str(e)}), 500
    return jsonify({'message': 'Redis not available'})

@app.route('/metrics', methods=['GET'])
def metrics():
    """Get agent metrics"""
    if not redis_client:
        return jsonify({'error': 'Redis not available'}), 500
    
    try:
        metrics = redis_client.hgetall(f"agent:{AGENT_ID}:metrics")
        metrics['conversation_count'] = len(get_conversation_history())
        return jsonify(metrics)
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/')
def index():
    return jsonify({
        'agent': 'gpt-agent',
        'endpoints': {
            '/': 'This help',
            '/health': 'Health check',
            '/chat': 'Chat endpoint (POST)',
            '/history': 'Get conversation history',
            '/clear': 'Clear history (POST)',
            '/metrics': 'Get agent metrics'
        },
        'features': {
            'conversation_memory': 'Remembers last conversations',
            'redis_state': 'State persisted in Redis',
            'context_aware': 'Uses conversation history for context'
        }
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)