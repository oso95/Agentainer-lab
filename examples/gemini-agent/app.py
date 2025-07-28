import os
import json
from flask import Flask, request, jsonify
from google import genai
from google.genai import types
from dotenv import load_dotenv
import redis
from datetime import datetime

# Load environment variables
load_dotenv()

app = Flask(__name__)

# Configuration - The SDK uses GEMINI_API_KEY env var automatically
MODEL = os.environ.get('GEMINI_MODEL', 'gemini-2.5-flash')

# Initialize Gemini client
try:
    client = genai.Client()
except Exception as e:
    print(f"Warning: Failed to initialize Gemini client: {e}")
    client = None

# Redis connection (Agentainer provides Redis at host.docker.internal:6379)
REDIS_HOST = os.environ.get('AGENTAINER_REDIS_HOST', 'host.docker.internal')
REDIS_PORT = int(os.environ.get('AGENTAINER_REDIS_PORT', 6379))
AGENT_ID = os.environ.get('AGENT_ID', 'gemini-agent')

try:
    redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
    redis_client.ping()
    print(f"Connected to Redis at {REDIS_HOST}:{REDIS_PORT}")
except:
    print(f"Warning: Could not connect to Redis at {REDIS_HOST}:{REDIS_PORT}")
    redis_client = None

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

@app.route('/health')
def health():
    return jsonify({
        'status': 'healthy' if client else 'unhealthy',
        'model': MODEL
    })

@app.route('/chat', methods=['POST'])
def chat():
    if not client:
        return jsonify({'error': 'Gemini client not initialized. Check GEMINI_API_KEY'}), 500
    
    data = request.get_json()
    if not data or 'message' not in data:
        return jsonify({'error': 'message required'}), 400
    
    user_message = data['message']
    
    # Build conversation with history
    conversation_text = ""
    
    # Add system prompt if provided
    system_prompt = data.get('system_prompt', '')
    if system_prompt:
        conversation_text += f"Instructions: {system_prompt}\n\n"
    
    # Add recent conversation history for context
    history = get_conversation_history()
    for conv in reversed(history[:3]):  # Last 3 conversations
        conversation_text += f"User: {conv['user']}\n"
        conversation_text += f"Assistant: {conv['assistant']}\n"
    
    # Add current message
    conversation_text += f"User: {user_message}\nAssistant: "
    
    try:
        # Use the official SDK to generate content
        response = client.models.generate_content(
            model=MODEL,
            contents=conversation_text,
            config=types.GenerateContentConfig(
                # Optional: disable thinking for faster responses
                # thinking_config=types.ThinkingConfig(thinking_budget=0)
            )
        )
        
        # Extract response text
        assistant_message = response.text
        
        # Save conversation to Redis
        save_conversation(user_message, assistant_message)
        
        return jsonify({
            'response': assistant_message,
            'model': MODEL,
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
        'agent': 'gemini-agent',
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